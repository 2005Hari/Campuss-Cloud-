package config

import (
	"os"
	"path/filepath"
	"testing"
)

func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		old, had := os.LookupEnv(k)
		os.Unsetenv(k)
		t.Cleanup(func() {
			if had {
				os.Setenv(k, old)
			} else {
				os.Unsetenv(k)
			}
		})
	}
}

func TestDefaultValues(t *testing.T) {
	c := Default()
	if c.Nextcloud.HTTPPort != 8080 {
		t.Errorf("expected default http port 8080, got %d", c.Nextcloud.HTTPPort)
	}
	if c.Monitoring.Thresholds.Warning != 70 || c.Monitoring.Thresholds.High != 80 || c.Monitoring.Thresholds.Critical != 90 {
		t.Errorf("unexpected default thresholds: %+v", c.Monitoring.Thresholds)
	}
	if c.Network.Name == "" || c.Compose.File == "" {
		t.Errorf("expected non-empty defaults, got %+v", c)
	}
}

func TestLoadYAMLOverridesDefaults(t *testing.T) {
	clearEnv(t, "CAMPUSCLOUD_HTTP_PORT", "CAMPUSCLOUD_BACKUP_DIR")

	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	content := []byte(`
nextcloud:
  http_port: 9090
monitoring:
  thresholds:
    warning: 60
`)
	if err := os.WriteFile(yamlPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(yamlPath, "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Nextcloud.HTTPPort != 9090 {
		t.Errorf("expected http_port 9090 from YAML, got %d", cfg.Nextcloud.HTTPPort)
	}
	if cfg.Monitoring.Thresholds.Warning != 60 {
		t.Errorf("expected warning threshold 60 from YAML, got %v", cfg.Monitoring.Thresholds.Warning)
	}
	// Fields not set in the YAML fragment must keep their defaults.
	if cfg.Monitoring.Thresholds.Critical != 90 {
		t.Errorf("expected critical threshold to keep default 90, got %v", cfg.Monitoring.Thresholds.Critical)
	}
}

func TestLoadMissingConfigFileFallsBackToDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"), "")
	if err != nil {
		t.Fatalf("expected no error for missing config file, got %v", err)
	}
	if cfg.Nextcloud.HTTPPort != 8080 {
		t.Errorf("expected default port, got %d", cfg.Nextcloud.HTTPPort)
	}
}

func TestEnvOverrideTakesPrecedenceOverYAML(t *testing.T) {
	clearEnv(t, "CAMPUSCLOUD_HTTP_PORT")
	os.Setenv("CAMPUSCLOUD_HTTP_PORT", "9999")

	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(yamlPath, []byte("nextcloud:\n  http_port: 9090\n"), 0o644)

	cfg, err := Load(yamlPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Nextcloud.HTTPPort != 9999 {
		t.Errorf("expected env var to override YAML, got %d", cfg.Nextcloud.HTTPPort)
	}
}

func TestLoadEnvFileDoesNotOverwriteExistingEnv(t *testing.T) {
	clearEnv(t, "MYSQL_PASSWORD")
	os.Setenv("MYSQL_PASSWORD", "real-env-wins")

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	os.WriteFile(envPath, []byte("MYSQL_PASSWORD=from-dotenv\n"), 0o644)

	if err := LoadEnvFile(envPath); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("MYSQL_PASSWORD"); got != "real-env-wins" {
		t.Errorf("expected real environment variable to win, got %q", got)
	}
}

func TestLoadEnvFileSetsUnsetVariables(t *testing.T) {
	clearEnv(t, "MYSQL_ROOT_PASSWORD")

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	os.WriteFile(envPath, []byte("# comment\nMYSQL_ROOT_PASSWORD=\"quoted-secret\"\n\n"), 0o644)

	if err := LoadEnvFile(envPath); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("MYSQL_ROOT_PASSWORD"); got != "quoted-secret" {
		t.Errorf("expected quotes to be stripped, got %q", got)
	}
}

func TestValidateReportsMissingSecrets(t *testing.T) {
	clearEnv(t, "MYSQL_ROOT_PASSWORD", "MYSQL_PASSWORD", "NEXTCLOUD_ADMIN_PASSWORD")

	cfg := Default()
	cfg.applyEnvOverrides()

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected Validate to fail when secrets are missing")
	}
}

func TestValidatePassesWithSecretsSet(t *testing.T) {
	clearEnv(t, "MYSQL_ROOT_PASSWORD", "MYSQL_PASSWORD", "NEXTCLOUD_ADMIN_PASSWORD")
	os.Setenv("MYSQL_ROOT_PASSWORD", "x")
	os.Setenv("MYSQL_PASSWORD", "y")
	os.Setenv("NEXTCLOUD_ADMIN_PASSWORD", "z")

	cfg := Default()
	cfg.applyEnvOverrides()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected Validate to pass, got %v", err)
	}
}
