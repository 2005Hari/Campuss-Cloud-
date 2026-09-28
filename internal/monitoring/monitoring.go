// Package monitoring implements FR-06 (Resource Monitoring): gathering
// host CPU, RAM, and storage usage and classifying each against the
// thresholds from PRD section 13.
package monitoring

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/2005Hari/campuscloud/internal/config"
)

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// State is a resource-usage classification bucket.
type State string

const (
	StateNormal   State = "normal"
	StateWarning  State = "warning"
	StateHigh     State = "high"
	StateCritical State = "critical"
)

// Classify buckets a usage percentage against the configured thresholds:
//
//	< warning         -> normal
//	[warning, high)    -> warning
//	[high, critical)   -> high
//	>= critical         -> critical
func Classify(percent float64, t config.Thresholds) State {
	switch {
	case percent >= t.Critical:
		return StateCritical
	case percent >= t.High:
		return StateHigh
	case percent >= t.Warning:
		return StateWarning
	default:
		return StateNormal
	}
}

// Usage is a single resource-usage reading.
type Usage struct {
	Label   string
	Used    uint64
	Total   uint64
	Percent float64
	State   State
}

func newUsage(label string, used, total uint64, t config.Thresholds) Usage {
	var pct float64
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return Usage{Label: label, Used: used, Total: total, Percent: pct, State: Classify(pct, t)}
}

// FormatBytes renders a byte count as a human-readable string (base 1024).
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := "KMGTPE"
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), units[exp])
}

// --- Memory (RAM) ---

// ParseMemInfo extracts MemTotal and MemAvailable (both in bytes) from the
// contents of /proc/meminfo.
func ParseMemInfo(data []byte) (totalBytes, availableBytes uint64, err error) {
	var total, available uint64
	var haveTotal, haveAvailable bool

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		valueKB, convErr := strconv.ParseUint(fields[1], 10, 64)
		if convErr != nil {
			continue
		}
		switch key {
		case "MemTotal":
			total, haveTotal = valueKB*1024, true
		case "MemAvailable":
			available, haveAvailable = valueKB*1024, true
		}
	}

	if !haveTotal {
		return 0, 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
	}
	if !haveAvailable {
		// Older kernels lack MemAvailable; treat all memory as used rather
		// than fail the whole read.
		available = 0
	}
	return total, available, nil
}

// HostMemory reads current RAM usage from /proc/meminfo and classifies it.
func HostMemory(t config.Thresholds) (Usage, error) {
	data, err := readFile("/proc/meminfo")
	if err != nil {
		return Usage{}, fmt.Errorf("reading /proc/meminfo: %w", err)
	}
	total, available, err := ParseMemInfo(data)
	if err != nil {
		return Usage{}, err
	}
	used := total - available
	return newUsage("Memory", used, total, t), nil
}

// --- Storage ---

// HostStorage reports disk usage for the filesystem containing path
// (typically the Docker data root or the configured backup directory).
// path itself need not exist yet (e.g. a backup directory checked by
// `doctor` before the first deploy ever creates it) — HostStorage walks up
// to the nearest existing ancestor directory, which is on the same
// filesystem in every case that matters here.
func HostStorage(path string, t config.Thresholds) (Usage, error) {
	existing, err := nearestExistingDir(path)
	if err != nil {
		return Usage{}, err
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(existing, &stat); err != nil {
		return Usage{}, fmt.Errorf("statfs %s: %w", existing, err)
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free
	return newUsage("Storage", used, total, t), nil
}

// nearestExistingDir returns path itself, or the nearest ancestor
// directory that exists, resolving relative paths against the current
// working directory first.
func nearestExistingDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", path, err)
	}
	for {
		if _, err := os.Stat(abs); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return abs, nil // reached the filesystem root
		}
		abs = parent
	}
}

// --- CPU ---

// CPUSample is a snapshot of aggregate CPU time counters (in USER_HZ ticks)
// read from the first "cpu " line of /proc/stat.
type CPUSample struct {
	User, Nice, System, Idle, IOWait, IRQ, SoftIRQ, Steal uint64
}

func (s CPUSample) total() uint64 {
	return s.User + s.Nice + s.System + s.Idle + s.IOWait + s.IRQ + s.SoftIRQ + s.Steal
}

func (s CPUSample) idleAll() uint64 {
	return s.Idle + s.IOWait
}

// ParseStatCPU extracts the aggregate CPU sample from the contents of
// /proc/stat.
func ParseStatCPU(data []byte) (CPUSample, error) {
	scanner := bytes.NewBuffer(data)
	for {
		line, err := scanner.ReadString('\n')
		if line == "" && err != nil {
			break
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "cpu" {
			return parseCPUFields(fields[1:])
		}
		if err != nil {
			break
		}
	}
	return CPUSample{}, fmt.Errorf("cpu line not found in /proc/stat")
}

func parseCPUFields(fields []string) (CPUSample, error) {
	vals := make([]uint64, 8)
	for i := 0; i < len(fields) && i < 8; i++ {
		v, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return CPUSample{}, fmt.Errorf("parsing cpu field %d (%q): %w", i, fields[i], err)
		}
		vals[i] = v
	}
	return CPUSample{
		User: vals[0], Nice: vals[1], System: vals[2], Idle: vals[3],
		IOWait: vals[4], IRQ: vals[5], SoftIRQ: vals[6], Steal: vals[7],
	}, nil
}

// CPUPercent computes the busy-percentage between two samples taken a short
// interval apart. It is pure and safe to unit test with synthetic samples.
func CPUPercent(prev, curr CPUSample) float64 {
	totalDelta := curr.total() - prev.total()
	if totalDelta == 0 {
		return 0
	}
	idleDelta := curr.idleAll() - prev.idleAll()
	busy := float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	if busy < 0 {
		return 0
	}
	if busy > 100 {
		return 100
	}
	return busy
}

// ReadCPUSample reads the current aggregate CPU sample from /proc/stat.
func ReadCPUSample() (CPUSample, error) {
	data, err := readFile("/proc/stat")
	if err != nil {
		return CPUSample{}, fmt.Errorf("reading /proc/stat: %w", err)
	}
	return ParseStatCPU(data)
}

// HostCPU samples CPU usage over the given interval (typically a few
// hundred milliseconds) and classifies the result.
func HostCPU(interval time.Duration, t config.Thresholds) (Usage, error) {
	prev, err := ReadCPUSample()
	if err != nil {
		return Usage{}, err
	}
	time.Sleep(interval)
	curr, err := ReadCPUSample()
	if err != nil {
		return Usage{}, err
	}
	percent := CPUPercent(prev, curr)
	return Usage{Label: "CPU", Percent: percent, State: Classify(percent, t)}, nil
}
