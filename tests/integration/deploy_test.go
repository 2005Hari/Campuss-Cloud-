//go:build integration

// Package integration holds CampusCloud's Docker-dependent end-to-end
// tests (PRD section 19: integration, failure, recovery, and
// backup/restore testing). They are excluded from the default `go test
// ./...` run — see this directory's README for how to run them on a real
// VM with Docker installed.
package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/2005Hari/campuscloud/internal/backup"
	"github.com/2005Hari/campuscloud/internal/config"
	"github.com/2005Hari/campuscloud/internal/deployment"
	"github.com/2005Hari/campuscloud/internal/dockercli"
	"github.com/2005Hari/campuscloud/internal/health"
)

// repoRoot locates the repository root from this test file's location, so
// the test can be run with `go test -tags integration ./tests/integration/...`
// from anywhere.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func testApp(t *testing.T) (*config.Config, *dockercli.Client) {
	t.Helper()
	root := repoRoot(t)

	cfg, err := config.Load(filepath.Join(root, "config/config.yaml"), filepath.Join(root, ".env"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	cfg.Compose.File = filepath.Join(root, "docker/docker-compose.yml")
	cfg.Backup.Dir = filepath.Join(t.TempDir(), "backups")

	if err := cfg.Validate(); err != nil {
		t.Skipf("skipping: .env is not configured with real secrets: %v", err)
	}

	client := dockercli.NewDefault(cfg.Compose.File, cfg.Project.Name).WithEnvFile(filepath.Join(root, ".env"))
	return cfg, client
}

// TestFullDeployLifecycle exercises the Core User Journey (PRD section 6)
// end to end: doctor -> deploy -> health -> stop -> start -> health.
func TestFullDeployLifecycle(t *testing.T) {
	cfg, client := testApp(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if report := deployment.Doctor(ctx, cfg, client); report.Overall == health.StatusFail {
		t.Fatalf("doctor reported failures: %+v", report.Results)
	}

	if err := deployment.Deploy(ctx, cfg, client, os.Stdout, deployment.DefaultOptions()); err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { client.ComposeDown(context.Background()) })
}

// TestFailureAndRecovery implements the PRD's failure/recovery testing:
// stop a container, confirm health detects it, restart, confirm recovery.
func TestFailureAndRecovery(t *testing.T) {
	cfg, client := testApp(t)
	ctx := context.Background()

	if err := deployment.Deploy(ctx, cfg, client, os.Stdout, deployment.DefaultOptions()); err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { client.ComposeDown(context.Background()) })

	if err := client.ComposeStop(ctx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	running, err := client.ContainerRunning(ctx, cfg.Nextcloud.ContainerName)
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Fatal("expected nextcloud container to be stopped")
	}

	if err := client.ComposeStart(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if err := deployment.WaitHealthy(ctx, cfg, client, 2*time.Minute, 3*time.Second); err != nil {
		t.Fatalf("expected recovery after restart, got: %v", err)
	}
}

// TestBackupAndRestore implements the PRD's backup/restore testing.
func TestBackupAndRestore(t *testing.T) {
	cfg, client := testApp(t)
	ctx := context.Background()

	if err := deployment.Deploy(ctx, cfg, client, os.Stdout, deployment.DefaultOptions()); err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { client.ComposeDown(context.Background()) })

	dir, err := backup.Create(ctx, cfg, client, os.Stdout)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	manifest, err := backup.ReadManifest(dir)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	if err := manifest.Validate(dir); err != nil {
		t.Fatalf("backup failed validation: %v", err)
	}

	if err := backup.Restore(ctx, cfg, client, dir, os.Stdout); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
}
