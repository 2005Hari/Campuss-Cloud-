// Package health implements FR-07 (Health Check): a set of independent
// checks against Docker, the Nextcloud and MariaDB containers, database
// connectivity, storage, the campuscloud network, and HTTP availability,
// plus aggregation into an overall report.
package health

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/2005Hari/campuscloud/internal/config"
	"github.com/2005Hari/campuscloud/internal/dockercli"
	"github.com/2005Hari/campuscloud/internal/monitoring"
)

// Status is the outcome of a single check.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Result is the outcome of a single named check.
type Result struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message"`
}

// Checker runs one health check.
type Checker interface {
	Name() string
	Check(ctx context.Context) Result
}

// CheckFunc adapts a plain function to the Checker interface.
type CheckFunc struct {
	CheckerName string
	Fn          func(ctx context.Context) Result
}

func (f CheckFunc) Name() string                     { return f.CheckerName }
func (f CheckFunc) Check(ctx context.Context) Result { return f.Fn(ctx) }

func pass(name, msg string) Result { return Result{Name: name, Status: StatusPass, Message: msg} }
func warn(name, msg string) Result { return Result{Name: name, Status: StatusWarn, Message: msg} }
func fail(name, msg string) Result { return Result{Name: name, Status: StatusFail, Message: msg} }

// Report is the aggregate result of running every check.
type Report struct {
	Results []Result `json:"results"`
	Overall Status   `json:"overall"`
}

// Run executes every checker (sequentially, so output ordering is stable
// and predictable for the CLI) and aggregates the overall status: fail
// beats warn beats pass.
func Run(ctx context.Context, checkers []Checker) Report {
	report := Report{Overall: StatusPass}
	for _, c := range checkers {
		res := c.Check(ctx)
		report.Results = append(report.Results, res)
		switch res.Status {
		case StatusFail:
			report.Overall = StatusFail
		case StatusWarn:
			if report.Overall != StatusFail {
				report.Overall = StatusWarn
			}
		}
	}
	return report
}

// --- Concrete checkers ---

// DockerDaemonCheck verifies the Docker daemon is reachable.
func DockerDaemonCheck(client *dockercli.Client) Checker {
	return CheckFunc{CheckerName: "docker-daemon", Fn: func(ctx context.Context) Result {
		if err := client.DaemonReachable(ctx); err != nil {
			return fail("docker-daemon", err.Error())
		}
		return pass("docker-daemon", "reachable")
	}}
}

// ContainerRunningCheck verifies the named container is in the running state.
func ContainerRunningCheck(name string, client *dockercli.Client) Checker {
	return CheckFunc{CheckerName: name + "-running", Fn: func(ctx context.Context) Result {
		running, err := client.ContainerRunning(ctx, name)
		if err != nil {
			return fail(name+"-running", err.Error())
		}
		if !running {
			return fail(name+"-running", "container is not running")
		}
		return pass(name+"-running", "running")
	}}
}

// ContainerHealthCheck verifies the named container's Docker HEALTHCHECK
// (if it declares one) reports "healthy".
func ContainerHealthCheck(name string, client *dockercli.Client) Checker {
	return CheckFunc{CheckerName: name + "-health", Fn: func(ctx context.Context) Result {
		status, err := client.ContainerHealth(ctx, name)
		if err != nil {
			return fail(name+"-health", err.Error())
		}
		switch status {
		case "", "healthy":
			return pass(name+"-health", "healthy")
		case "starting":
			return warn(name+"-health", "still starting")
		default:
			return fail(name+"-health", "status: "+status)
		}
	}}
}

// NetworkCheck verifies the campuscloud Docker network exists.
func NetworkCheck(name string, client *dockercli.Client) Checker {
	return CheckFunc{CheckerName: "network", Fn: func(ctx context.Context) Result {
		exists, err := client.NetworkExists(ctx, name)
		if err != nil {
			return fail("network", err.Error())
		}
		if !exists {
			return fail("network", fmt.Sprintf("network %q does not exist", name))
		}
		return pass("network", name+" present")
	}}
}

// ComposeExecCheck runs a command inside a compose service via `docker
// compose exec` and treats a zero exit status as passing. This is how
// CampusCloud checks MariaDB connectivity from inside the stack, since the
// database is deliberately kept off the host-published network (PRD
// section 18, Security Requirements) and so isn't reachable by dialing it
// directly from the host.
func ComposeExecCheck(name string, client *dockercli.Client, service string, cmdArgs ...string) Checker {
	return CheckFunc{CheckerName: name, Fn: func(ctx context.Context) Result {
		out, err := client.ComposeExec(ctx, service, cmdArgs...)
		if err != nil {
			return fail(name, err.Error())
		}
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = "ok"
		}
		return pass(name, msg)
	}}
}

// StorageCheck verifies free disk space at path against the configured
// thresholds (PRD section 13).
func StorageCheck(path string, thresholds config.Thresholds) Checker {
	return CheckFunc{CheckerName: "storage", Fn: func(ctx context.Context) Result {
		usage, err := monitoring.HostStorage(path, thresholds)
		if err != nil {
			return fail("storage", err.Error())
		}
		msg := fmt.Sprintf("%.1f%% used (%s / %s)", usage.Percent, monitoring.FormatBytes(usage.Used), monitoring.FormatBytes(usage.Total))
		switch usage.State {
		case monitoring.StateCritical:
			return fail("storage", msg)
		case monitoring.StateHigh, monitoring.StateWarning:
			return warn("storage", msg)
		default:
			return pass("storage", msg)
		}
	}}
}

// HTTPCheck verifies an HTTP(S) endpoint responds with a non-5xx,
// non-error status (used against Nextcloud's status.php).
func HTTPCheck(name, url string, timeout time.Duration) Checker {
	return CheckFunc{CheckerName: name, Fn: func(ctx context.Context) Result {
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if err != nil {
			return fail(name, err.Error())
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fail(name, err.Error())
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return fail(name, fmt.Sprintf("HTTP %d", resp.StatusCode))
		}
		if resp.StatusCode >= 400 {
			return warn(name, fmt.Sprintf("HTTP %d", resp.StatusCode))
		}
		return pass(name, fmt.Sprintf("HTTP %d", resp.StatusCode))
	}}
}

// TCPCheck verifies a TCP port accepts connections (a lightweight fallback
// database-connectivity signal when full SQL credentials aren't needed).
func TCPCheck(name, address string, timeout time.Duration) Checker {
	return CheckFunc{CheckerName: name, Fn: func(ctx context.Context) Result {
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err != nil {
			return fail(name, err.Error())
		}
		conn.Close()
		return pass(name, "port open")
	}}
}

// MySQLDSN builds a go-sql-driver/mysql DSN for connectivity checks.
func MySQLDSN(user, password, host string, port int, database string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=5s", user, password, host, port, database)
}

// DBConnectivityCheck verifies CampusCloud can authenticate to MariaDB and
// run a trivial query (FR-07's "database connectivity").
func DBConnectivityCheck(name, dsn string) Checker {
	return CheckFunc{CheckerName: name, Fn: func(ctx context.Context) Result {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return fail(name, err.Error())
		}
		defer db.Close()

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			return fail(name, err.Error())
		}
		return pass(name, "connected")
	}}
}
