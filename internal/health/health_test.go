package health

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/2005Hari/campuscloud/internal/config"
)

func fakeChecker(name string, status Status) Checker {
	return CheckFunc{CheckerName: name, Fn: func(context.Context) Result {
		return Result{Name: name, Status: status, Message: "fake"}
	}}
}

func TestRunOverallPassWhenAllPass(t *testing.T) {
	report := Run(context.Background(), []Checker{fakeChecker("a", StatusPass), fakeChecker("b", StatusPass)})
	if report.Overall != StatusPass {
		t.Errorf("overall = %v, want pass", report.Overall)
	}
	if len(report.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(report.Results))
	}
}

func TestRunOverallWarnWhenOneWarns(t *testing.T) {
	report := Run(context.Background(), []Checker{fakeChecker("a", StatusPass), fakeChecker("b", StatusWarn)})
	if report.Overall != StatusWarn {
		t.Errorf("overall = %v, want warn", report.Overall)
	}
}

func TestRunOverallFailBeatsWarn(t *testing.T) {
	report := Run(context.Background(), []Checker{fakeChecker("a", StatusWarn), fakeChecker("b", StatusFail)})
	if report.Overall != StatusFail {
		t.Errorf("overall = %v, want fail", report.Overall)
	}
}

func TestRunOverallFailStaysFailAfterLaterWarn(t *testing.T) {
	report := Run(context.Background(), []Checker{fakeChecker("a", StatusFail), fakeChecker("b", StatusWarn)})
	if report.Overall != StatusFail {
		t.Errorf("overall = %v, want fail", report.Overall)
	}
}

func TestStorageCheckOnRealFilesystem(t *testing.T) {
	dir := t.TempDir()
	thresholds := config.Thresholds{Warning: 70, High: 80, Critical: 90}
	res := StorageCheck(dir, thresholds).Check(context.Background())
	if res.Status != StatusPass && res.Status != StatusWarn && res.Status != StatusFail {
		t.Errorf("unexpected status %v", res.Status)
	}
}

func TestStorageCheckWalksUpForNonexistentPath(t *testing.T) {
	// A backup directory that doesn't exist yet (the normal state before
	// the first `campuscloud deploy`) must not make the check blow up —
	// HostStorage falls back to the nearest existing ancestor.
	thresholds := config.Thresholds{Warning: 70, High: 80, Critical: 90}
	res := StorageCheck("/this/path/does/not/exist/at/all", thresholds).Check(context.Background())
	if res.Status == StatusFail {
		t.Errorf("expected fallback to an existing ancestor directory, got fail: %s", res.Message)
	}
}

func TestHTTPCheckPassOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := HTTPCheck("nextcloud-http", srv.URL, time.Second).Check(context.Background())
	if res.Status != StatusPass {
		t.Errorf("status = %v, want pass: %s", res.Status, res.Message)
	}
}

func TestHTTPCheckFailOn500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	res := HTTPCheck("nextcloud-http", srv.URL, time.Second).Check(context.Background())
	if res.Status != StatusFail {
		t.Errorf("status = %v, want fail", res.Status)
	}
}

func TestHTTPCheckWarnOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	res := HTTPCheck("nextcloud-http", srv.URL, time.Second).Check(context.Background())
	if res.Status != StatusWarn {
		t.Errorf("status = %v, want warn", res.Status)
	}
}

func TestHTTPCheckFailOnUnreachable(t *testing.T) {
	res := HTTPCheck("nextcloud-http", "http://127.0.0.1:1", 200*time.Millisecond).Check(context.Background())
	if res.Status != StatusFail {
		t.Errorf("status = %v, want fail", res.Status)
	}
}

func TestTCPCheckPass(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	res := TCPCheck("mariadb-port", ln.Addr().String(), time.Second).Check(context.Background())
	if res.Status != StatusPass {
		t.Errorf("status = %v, want pass: %s", res.Status, res.Message)
	}
}

func TestTCPCheckFailOnClosedPort(t *testing.T) {
	res := TCPCheck("mariadb-port", "127.0.0.1:1", 200*time.Millisecond).Check(context.Background())
	if res.Status != StatusFail {
		t.Errorf("status = %v, want fail", res.Status)
	}
}

func TestMySQLDSN(t *testing.T) {
	dsn := MySQLDSN("nextcloud", "secret", "mariadb", 3306, "nextcloud")
	want := "nextcloud:secret@tcp(mariadb:3306)/nextcloud?timeout=5s"
	if dsn != want {
		t.Errorf("got %q, want %q", dsn, want)
	}
}
