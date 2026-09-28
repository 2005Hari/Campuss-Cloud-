package dockercli

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// fakeRunner records invocations and returns canned responses keyed by the
// joined command line, so tests never need a real docker binary.
type fakeRunner struct {
	responses map[string]fakeResponse
	calls     []string
}

type fakeResponse struct {
	stdout string
	stderr string
	err    error
}

func key(name string, args ...string) string {
	return name + " " + strings.Join(args, " ")
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	k := key(name, args...)
	f.calls = append(f.calls, k)
	if resp, ok := f.responses[k]; ok {
		return resp.stdout, resp.stderr, resp.err
	}
	return "", "", nil
}

func (f *fakeRunner) Stream(_ context.Context, _ io.Reader, stdout, _ io.Writer, name string, args ...string) error {
	k := key(name, args...)
	f.calls = append(f.calls, k)
	if resp, ok := f.responses[k]; ok {
		io.WriteString(stdout, resp.stdout)
		return resp.err
	}
	return nil
}

func newTestClient(f *fakeRunner) *Client {
	return New(f, "docker/docker-compose.yml", "campuscloud")
}

func TestNetworkExistsTrue(t *testing.T) {
	f := &fakeRunner{responses: map[string]fakeResponse{
		key("docker", "network", "ls", "--filter", "name=^campuscloud_net$", "--format", "{{.Name}}"): {stdout: "campuscloud_net\n"},
	}}
	c := newTestClient(f)

	exists, err := c.NetworkExists(context.Background(), "campuscloud_net")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected network to exist")
	}
}

func TestNetworkExistsFalse(t *testing.T) {
	f := &fakeRunner{responses: map[string]fakeResponse{}}
	c := newTestClient(f)

	exists, err := c.NetworkExists(context.Background(), "campuscloud_net")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("expected network to not exist")
	}
}

func TestEnsureNetworkSkipsCreateWhenExists(t *testing.T) {
	f := &fakeRunner{responses: map[string]fakeResponse{
		key("docker", "network", "ls", "--filter", "name=^campuscloud_net$", "--format", "{{.Name}}"): {stdout: "campuscloud_net\n"},
	}}
	c := newTestClient(f)

	if err := c.EnsureNetwork(context.Background(), "campuscloud_net"); err != nil {
		t.Fatal(err)
	}
	for _, call := range f.calls {
		if strings.Contains(call, "network create") {
			t.Errorf("expected no create call when network already exists, got calls: %v", f.calls)
		}
	}
}

func TestEnsureNetworkCreatesWhenMissing(t *testing.T) {
	f := &fakeRunner{responses: map[string]fakeResponse{}}
	c := newTestClient(f)

	if err := c.EnsureNetwork(context.Background(), "campuscloud_net"); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, call := range f.calls {
		if call == key("docker", "network", "create", "campuscloud_net") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a network create call, got: %v", f.calls)
	}
}

func TestComposeUpBuildsExpectedArgs(t *testing.T) {
	f := &fakeRunner{responses: map[string]fakeResponse{}}
	c := newTestClient(f)

	if err := c.ComposeUp(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := key("docker", "compose", "-f", "docker/docker-compose.yml", "-p", "campuscloud", "up", "-d", "--remove-orphans")
	if len(f.calls) != 1 || f.calls[0] != want {
		t.Errorf("got calls %v, want [%s]", f.calls, want)
	}
}

func TestWithEnvFileAddedToComposeArgs(t *testing.T) {
	f := &fakeRunner{responses: map[string]fakeResponse{}}
	c := newTestClient(f).WithEnvFile(".env")

	if err := c.ComposeUp(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := key("docker", "compose", "-f", "docker/docker-compose.yml", "-p", "campuscloud", "--env-file", ".env", "up", "-d", "--remove-orphans")
	if len(f.calls) != 1 || f.calls[0] != want {
		t.Errorf("got calls %v, want [%s]", f.calls, want)
	}
}

func TestRunWrapsFailureWithStderr(t *testing.T) {
	f := &fakeRunner{responses: map[string]fakeResponse{
		key("docker", "info", "--format", "{{.ServerVersion}}"): {stderr: "Cannot connect to the Docker daemon", err: errors.New("exit status 1")},
	}}
	c := newTestClient(f)

	err := c.DaemonReachable(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
		t.Errorf("expected error to include stderr, got: %v", err)
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("expected *ExitError, got %T", err)
	}
}

func TestComposePSParsesJSONArray(t *testing.T) {
	f := &fakeRunner{responses: map[string]fakeResponse{
		key("docker", "compose", "-f", "docker/docker-compose.yml", "-p", "campuscloud", "ps", "-a", "--format", "json"): {
			stdout: `[{"Name":"campuscloud-nextcloud","Service":"nextcloud","State":"running","Status":"Up 2 minutes","Health":"healthy"}]`,
		},
	}}
	c := newTestClient(f)

	got, err := c.ComposePS(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "campuscloud-nextcloud" || got[0].Health != "healthy" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestStatsParsesJSONLines(t *testing.T) {
	f := &fakeRunner{responses: map[string]fakeResponse{
		key("docker", "stats", "--no-stream", "--format", "json"): {
			stdout: "{\"Name\":\"a\",\"CPUPerc\":\"1.00%\"}\n{\"Name\":\"b\",\"CPUPerc\":\"2.00%\"}\n",
		},
	}}
	c := newTestClient(f)

	got, err := c.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestParseJSONObjectsEmptyOutput(t *testing.T) {
	got, err := parseJSONObjects[ContainerStats]("   \n  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil slice for empty output, got %v", got)
	}
}

func TestContainerHealthEmptyWhenNoHealthcheck(t *testing.T) {
	f := &fakeRunner{responses: map[string]fakeResponse{
		key("docker", "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{end}}", "mariadb"): {stdout: "\n"},
	}}
	c := newTestClient(f)

	status, err := c.ContainerHealth(context.Background(), "mariadb")
	if err != nil {
		t.Fatal(err)
	}
	if status != "" {
		t.Errorf("expected empty health status, got %q", status)
	}
}
