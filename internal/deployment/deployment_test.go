package deployment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/2005Hari/campuscloud/internal/config"
	"github.com/2005Hari/campuscloud/internal/dockercli"
)

// scriptedRunner implements dockercli.Runner for tests, keyed by joined
// command line so each test only has to describe the commands it cares
// about; anything else returns an empty success.
type scriptedRunner struct {
	responses map[string]struct {
		stdout string
		err    error
	}
}

func key(name string, args ...string) string { return name + " " + strings.Join(args, " ") }

func (r *scriptedRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	if resp, ok := r.responses[key(name, args...)]; ok {
		return resp.stdout, "", resp.err
	}
	return "", "", nil
}

func (r *scriptedRunner) Stream(_ context.Context, _ io.Reader, _, _ io.Writer, _ string, _ ...string) error {
	return nil
}

func testConfig(t *testing.T) *config.Config {
	cfg := config.Default()
	cfg.Backup.Dir = t.TempDir()
	cfg.Nextcloud.ContainerName = "campuscloud-nextcloud"
	cfg.MariaDB.ContainerName = "campuscloud-mariadb"
	return cfg
}

func TestDoctorAllPassing(t *testing.T) {
	r := &scriptedRunner{responses: map[string]struct {
		stdout string
		err    error
	}{
		key("docker", "version", "--format", "{{.Server.Version}}"): {stdout: "27.3.1\n"},
		key("docker", "compose", "version", "--short"):              {stdout: "2.29.0\n"},
		key("docker", "info", "--format", "{{.ServerVersion}}"):     {stdout: "27.3.1\n"},
	}}
	client := dockercli.New(r, "docker/docker-compose.yml", "campuscloud")
	cfg := testConfig(t)

	report := Doctor(context.Background(), cfg, client)
	// RAM/storage checks read the real host, so they may warn on a small
	// sandbox VM but must never fail outright when docker checks pass.
	for _, res := range report.Results {
		if (res.Name == "docker-installed" || res.Name == "compose-installed" || res.Name == "docker-daemon") && res.Status != "pass" {
			t.Errorf("expected %s to pass, got %v: %s", res.Name, res.Status, res.Message)
		}
	}
}

func TestDoctorFailsWhenDaemonUnreachable(t *testing.T) {
	r := &scriptedRunner{responses: map[string]struct {
		stdout string
		err    error
	}{
		key("docker", "version", "--format", "{{.Server.Version}}"): {stdout: "27.3.1\n"},
		key("docker", "compose", "version", "--short"):              {stdout: "2.29.0\n"},
		key("docker", "info", "--format", "{{.ServerVersion}}"):     {err: errors.New("connection refused")},
	}}
	client := dockercli.New(r, "docker/docker-compose.yml", "campuscloud")
	cfg := testConfig(t)

	report := Doctor(context.Background(), cfg, client)
	if report.Overall != "fail" {
		t.Errorf("expected overall fail, got %v", report.Overall)
	}
}

func TestDeployAbortsWhenDoctorFails(t *testing.T) {
	r := &scriptedRunner{responses: map[string]struct {
		stdout string
		err    error
	}{
		key("docker", "version", "--format", "{{.Server.Version}}"): {err: errors.New("docker: command not found")},
	}}
	client := dockercli.New(r, "docker/docker-compose.yml", "campuscloud")
	cfg := testConfig(t)

	var out bytes.Buffer
	err := Deploy(context.Background(), cfg, client, &out, Options{HealthTimeout: time.Second, PollInterval: time.Millisecond})
	if err == nil {
		t.Fatal("expected Deploy to fail when doctor checks fail")
	}
	if strings.Contains(out.String(), "Starting MariaDB") {
		t.Error("expected deploy to abort before starting services")
	}
}

func TestDeploySucceedsAndReportsURL(t *testing.T) {
	r := &scriptedRunner{responses: map[string]struct {
		stdout string
		err    error
	}{
		key("docker", "inspect", "--format", "{{.State.Running}}", "campuscloud-nextcloud"):                                  {stdout: "true\n"},
		key("docker", "inspect", "--format", "{{.State.Running}}", "campuscloud-mariadb"):                                    {stdout: "true\n"},
		key("docker", "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{end}}", "campuscloud-nextcloud"): {stdout: "healthy\n"},
		key("docker", "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{end}}", "campuscloud-mariadb"):   {stdout: "healthy\n"},
	}}
	client := dockercli.New(r, "docker/docker-compose.yml", "campuscloud")
	cfg := testConfig(t)

	var out bytes.Buffer
	err := Deploy(context.Background(), cfg, client, &out, Options{SkipDoctor: true, HealthTimeout: time.Second, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}
	if !strings.Contains(out.String(), "http://localhost:8080") {
		t.Errorf("expected output to report the Nextcloud URL, got: %s", out.String())
	}
}

func TestWaitHealthyTimesOutWhenContainerNeverRuns(t *testing.T) {
	r := &scriptedRunner{responses: map[string]struct {
		stdout string
		err    error
	}{
		key("docker", "inspect", "--format", "{{.State.Running}}", "campuscloud-nextcloud"): {stdout: "false\n"},
	}}
	client := dockercli.New(r, "docker/docker-compose.yml", "campuscloud")
	cfg := testConfig(t)

	err := WaitHealthy(context.Background(), cfg, client, 20*time.Millisecond, 5*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout message, got: %v", err)
	}
}

func TestWaitHealthySucceedsOnceHealthy(t *testing.T) {
	r := &scriptedRunner{responses: map[string]struct {
		stdout string
		err    error
	}{
		key("docker", "inspect", "--format", "{{.State.Running}}", "campuscloud-nextcloud"):                                  {stdout: "true\n"},
		key("docker", "inspect", "--format", "{{.State.Running}}", "campuscloud-mariadb"):                                    {stdout: "true\n"},
		key("docker", "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{end}}", "campuscloud-nextcloud"): {stdout: ""},
		key("docker", "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{end}}", "campuscloud-mariadb"):   {stdout: ""},
	}}
	client := dockercli.New(r, "docker/docker-compose.yml", "campuscloud")
	cfg := testConfig(t)

	if err := WaitHealthy(context.Background(), cfg, client, 100*time.Millisecond, 5*time.Millisecond); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}
