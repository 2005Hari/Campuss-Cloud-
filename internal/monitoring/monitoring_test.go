package monitoring

import (
	"testing"

	"github.com/2005Hari/campuscloud/internal/config"
)

func thresholds() config.Thresholds {
	return config.Thresholds{Warning: 70, High: 80, Critical: 90}
}

func TestClassify(t *testing.T) {
	tr := thresholds()
	cases := []struct {
		percent float64
		want    State
	}{
		{0, StateNormal},
		{69.9, StateNormal},
		{70, StateWarning},
		{75, StateWarning},
		{80, StateHigh},
		{89.9, StateHigh},
		{90, StateCritical},
		{100, StateCritical},
	}
	for _, c := range cases {
		if got := Classify(c.percent, tr); got != c.want {
			t.Errorf("Classify(%v) = %v, want %v", c.percent, got, c.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{500, "500 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.in); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseMemInfo(t *testing.T) {
	data := []byte(`MemTotal:        8000000 kB
MemFree:         1000000 kB
MemAvailable:    3000000 kB
Buffers:          200000 kB
`)
	total, available, err := ParseMemInfo(data)
	if err != nil {
		t.Fatal(err)
	}
	if total != 8000000*1024 {
		t.Errorf("total = %d, want %d", total, 8000000*1024)
	}
	if available != 3000000*1024 {
		t.Errorf("available = %d, want %d", available, 3000000*1024)
	}
}

func TestParseMemInfoMissingTotal(t *testing.T) {
	_, _, err := ParseMemInfo([]byte("MemFree: 100 kB\n"))
	if err == nil {
		t.Fatal("expected error when MemTotal is missing")
	}
}

func TestParseMemInfoMissingAvailableFallsBackToZero(t *testing.T) {
	total, available, err := ParseMemInfo([]byte("MemTotal: 1000 kB\n"))
	if err != nil {
		t.Fatal(err)
	}
	if total != 1000*1024 {
		t.Errorf("total = %d", total)
	}
	if available != 0 {
		t.Errorf("expected available=0 fallback, got %d", available)
	}
}

func TestParseStatCPU(t *testing.T) {
	data := []byte(`cpu  100 10 50 800 20 5 5 0 0 0
cpu0 50 5 25 400 10 2 2 0 0 0
intr 12345
`)
	sample, err := ParseStatCPU(data)
	if err != nil {
		t.Fatal(err)
	}
	want := CPUSample{User: 100, Nice: 10, System: 50, Idle: 800, IOWait: 20, IRQ: 5, SoftIRQ: 5, Steal: 0}
	if sample != want {
		t.Errorf("got %+v, want %+v", sample, want)
	}
}

func TestParseStatCPUMissingLine(t *testing.T) {
	_, err := ParseStatCPU([]byte("intr 12345\n"))
	if err == nil {
		t.Fatal("expected error when cpu line is missing")
	}
}

func TestCPUPercent(t *testing.T) {
	prev := CPUSample{User: 100, System: 50, Idle: 800}
	// After some time: +50 busy ticks, +50 idle ticks -> total delta 100, idle delta 50 -> 50% busy.
	curr := CPUSample{User: 150, System: 50, Idle: 850}

	got := CPUPercent(prev, curr)
	if got != 50 {
		t.Errorf("CPUPercent = %v, want 50", got)
	}
}

func TestCPUPercentNoDelta(t *testing.T) {
	s := CPUSample{User: 100, Idle: 800}
	if got := CPUPercent(s, s); got != 0 {
		t.Errorf("CPUPercent with no delta = %v, want 0", got)
	}
}

func TestCPUPercentFullyBusy(t *testing.T) {
	prev := CPUSample{User: 0, Idle: 0}
	curr := CPUSample{User: 100, Idle: 0}
	if got := CPUPercent(prev, curr); got != 100 {
		t.Errorf("CPUPercent = %v, want 100", got)
	}
}

func TestHostStorageOnRealFilesystem(t *testing.T) {
	dir := t.TempDir()
	usage, err := HostStorage(dir, thresholds())
	if err != nil {
		t.Fatal(err)
	}
	if usage.Total == 0 {
		t.Error("expected non-zero total storage")
	}
	if usage.Used > usage.Total {
		t.Errorf("used (%d) > total (%d)", usage.Used, usage.Total)
	}
	if usage.Percent < 0 || usage.Percent > 100 {
		t.Errorf("percent out of range: %v", usage.Percent)
	}
}

func TestHostStorageOnNonexistentNestedPath(t *testing.T) {
	dir := t.TempDir()
	// backups/ doesn't exist yet — this is the normal pre-deploy state
	// `campuscloud doctor` runs in, and it must not fail statfs outright.
	nested := dir + "/backups/does/not/exist"
	usage, err := HostStorage(nested, thresholds())
	if err != nil {
		t.Fatalf("expected HostStorage to walk up to an existing ancestor, got error: %v", err)
	}
	if usage.Total == 0 {
		t.Error("expected non-zero total storage")
	}
}

func TestHostMemoryOnRealProcfs(t *testing.T) {
	usage, err := HostMemory(thresholds())
	if err != nil {
		t.Skipf("skipping: /proc/meminfo unavailable in this environment: %v", err)
	}
	if usage.Total == 0 {
		t.Error("expected non-zero total memory")
	}
}
