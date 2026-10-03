package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/2005Hari/campuscloud/internal/config"
	"github.com/2005Hari/campuscloud/internal/dockercli"
)

// fakeRunner is a minimal dockercli.Runner double so tests never shell out
// to a real docker binary. Keyed by joined command line, like the
// dockercli/deployment package tests.
type fakeRunner struct {
	responses map[string]fakeResponse
}

type fakeResponse struct {
	stdout string
	err    error
}

func cmdKey(name string, args ...string) string { return name + " " + strings.Join(args, " ") }

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	if resp, ok := f.responses[cmdKey(name, args...)]; ok {
		return resp.stdout, "", resp.err
	}
	return "", "", nil
}

func (f *fakeRunner) Stream(_ context.Context, _ io.Reader, stdout, _ io.Writer, name string, args ...string) error {
	if resp, ok := f.responses[cmdKey(name, args...)]; ok {
		io.WriteString(stdout, resp.stdout)
		return resp.err
	}
	return nil
}

func testServer(t *testing.T, token string, origins []string, responses map[string]fakeResponse) *Server {
	t.Helper()
	if responses == nil {
		responses = map[string]fakeResponse{}
	}
	cfg := config.Default()
	cfg.Backup.Dir = t.TempDir()
	client := dockercli.New(&fakeRunner{responses: responses}, cfg.Compose.File, cfg.Project.Name)

	s, err := NewServer(cfg, client, Options{Token: token, AllowedOrigins: origins})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewServerRequiresToken(t *testing.T) {
	cfg := config.Default()
	client := dockercli.New(&fakeRunner{}, cfg.Compose.File, cfg.Project.Name)
	if _, err := NewServer(cfg, client, Options{Token: ""}); err == nil {
		t.Fatal("expected an error when no token is configured")
	}
	if _, err := NewServer(cfg, client, Options{Token: "   "}); err == nil {
		t.Fatal("expected an error for a whitespace-only token")
	}
}

func TestHealthzIsUnauthenticated(t *testing.T) {
	s := testServer(t, "secret", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAPIRejectsMissingToken(t *testing.T) {
	s := testServer(t, "secret", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAPIRejectsWrongToken(t *testing.T) {
	s := testServer(t, "secret", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAPIAcceptsCorrectToken(t *testing.T) {
	s := testServer(t, "secret", nil, map[string]fakeResponse{
		cmdKey("docker", "compose", "-f", "docker/docker-compose.yml", "-p", "campuscloud", "ps", "-a", "--format", "json"): {stdout: "[]"},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestCORSPreflightAllowedOrigin(t *testing.T) {
	s := testServer(t, "secret", []string{"https://dashboard.example.edu"}, nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/status", nil)
	req.Header.Set("Origin", "https://dashboard.example.edu")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://dashboard.example.edu" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestCORSHeaderOmittedForDisallowedOrigin(t *testing.T) {
	s := testServer(t, "secret", []string{"https://dashboard.example.edu"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS header for disallowed origin, got %q", got)
	}
}

func TestCORSWildcardAllowsAnyOrigin(t *testing.T) {
	s := testServer(t, "secret", []string{"*"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://anything.example.com")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://anything.example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the request's origin echoed back", got)
	}
}

func TestHandleHealthReturns200EvenWhenChecksFail(t *testing.T) {
	// docker-daemon check fails since no fake response is registered for
	// `docker info`, which drives the whole report's overall to "fail" —
	// the HTTP status must still be 200 so the caller gets the full body.
	s := testServer(t, "secret", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 regardless of check outcome", rec.Code)
	}
	var body struct {
		Overall string `json:"overall"`
		Results []any  `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Overall != "fail" {
		t.Errorf("overall = %q, want fail", body.Overall)
	}
	if len(body.Results) == 0 {
		t.Error("expected a non-empty results list in the body")
	}
}

func TestHandleContainersUsesLowercaseJSON(t *testing.T) {
	s := testServer(t, "secret", nil, map[string]fakeResponse{
		cmdKey("docker", "compose", "-f", "docker/docker-compose.yml", "-p", "campuscloud", "ps", "-a", "--format", "json"): {
			stdout: `[{"Name":"campuscloud-nextcloud","Service":"nextcloud","State":"running","Status":"Up","Health":"healthy"}]`,
		},
		cmdKey("docker", "stats", "--no-stream", "--format", "json", "campuscloud-nextcloud"): {
			stdout: `{"Name":"campuscloud-nextcloud","CPUPerc":"1.23%","MemUsage":"10MiB / 100MiB","MemPerc":"10%","NetIO":"1kB / 2kB","BlockIO":"0B / 0B"}`,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Containers []map[string]any `json:"containers"`
		Stats      []map[string]any `json:"stats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Containers) != 1 || body.Containers[0]["name"] != "campuscloud-nextcloud" {
		t.Errorf("containers = %+v, want lowercase \"name\" field", body.Containers)
	}
	if len(body.Stats) != 1 || body.Stats[0]["cpu_percent"] != "1.23%" {
		t.Errorf("stats = %+v, want lowercase \"cpu_percent\" field", body.Stats)
	}
}

func TestHandleConfig(t *testing.T) {
	s := testServer(t, "secret", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["project_name"] != "campuscloud" {
		t.Errorf("project_name = %v", body["project_name"])
	}
	// Secrets must never appear in the config response.
	for _, forbidden := range []string{"password", "Password", "root_password"} {
		if _, ok := body[forbidden]; ok {
			t.Errorf("config response leaked a %s field", forbidden)
		}
	}
}

func TestHandleBackupsListEmpty(t *testing.T) {
	s := testServer(t, "secret", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Backups []string `json:"backups"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Backups == nil || len(body.Backups) != 0 {
		t.Errorf("backups = %v, want empty (non-null) array", body.Backups)
	}
}

func TestHandleJobNotFound(t *testing.T) {
	s := testServer(t, "secret", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/does-not-exist", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleStartTriggersJobAndCompletes(t *testing.T) {
	s := testServer(t, "secret", nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/start", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var created JobView
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Kind != "start" {
		t.Errorf("kind = %q, want start", created.Kind)
	}

	final := waitForStatus(t, s.jobs, created.ID, JobSucceeded, time.Second)

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+created.ID, nil)
	req2.Header.Set("Authorization", "Bearer secret")
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d", rec2.Code)
	}
	var polled JobView
	if err := json.Unmarshal(rec2.Body.Bytes(), &polled); err != nil {
		t.Fatal(err)
	}
	if polled.Status != JobSucceeded || polled.Status != final.Status {
		t.Errorf("polled status = %v", polled.Status)
	}
}

func TestHandleRestoreRequiresBackupIDOrLatest(t *testing.T) {
	s := testServer(t, "secret", nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/restore", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleRestoreLatestWithNoBackups(t *testing.T) {
	s := testServer(t, "secret", nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/restore", strings.NewReader(`{"latest":true}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
