// Package dockercli is a thin, dependency-injectable wrapper around the
// `docker` and `docker compose` command-line tools. CampusCloud shells out
// to these binaries (rather than vendoring the full Docker Engine SDK) to
// keep the CLI's dependency footprint small and portable across any host
// that already has Docker installed — the same requirement FR-01 (doctor)
// checks for.
package dockercli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Runner executes an external command and captures its output. It exists
// so tests can substitute a fake implementation instead of shelling out to
// a real docker binary.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout, stderr string, err error)
	// Stream behaves like Run but connects the child process's stdin/stdout/
	// stderr directly to the given reader/writers, for long-running or
	// data-piping commands such as `docker compose logs -f`, streaming a
	// tar archive out of a container, or piping a SQL dump back in.
	// A nil stdin leaves the child's stdin unconnected (as if from /dev/null).
	Stream(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error
}

// ExecRunner is the real Runner backed by os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return out.String(), errBuf.String(), err
}

func (ExecRunner) Stream(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if stdin != nil {
		cmd.Stdin = stdin
	}
	return cmd.Run()
}

// Client is the CampusCloud-facing Docker/Compose client.
type Client struct {
	runner      Runner
	composeFile string
	projectName string
	envFile     string
}

// New builds a Client with an injected Runner (used in tests).
func New(runner Runner, composeFile, projectName string) *Client {
	return &Client{runner: runner, composeFile: composeFile, projectName: projectName}
}

// NewDefault builds a Client backed by the real docker/docker compose CLI.
func NewDefault(composeFile, projectName string) *Client {
	return New(ExecRunner{}, composeFile, projectName)
}

// WithEnvFile sets the --env-file passed to every `docker compose`
// invocation, so ${VAR} substitution in docker-compose.yml reads the same
// .env CampusCloud itself loaded — regardless of the compose file's
// location relative to the current working directory. It returns c for
// chaining.
func (c *Client) WithEnvFile(path string) *Client {
	c.envFile = path
	return c
}

func (c *Client) composeArgs(rest ...string) []string {
	args := []string{"compose", "-f", c.composeFile}
	if c.projectName != "" {
		args = append(args, "-p", c.projectName)
	}
	if c.envFile != "" {
		args = append(args, "--env-file", c.envFile)
	}
	return append(args, rest...)
}

// ExitError wraps a failed command with its captured stderr so callers can
// surface a useful message instead of a bare "exit status 1".
type ExitError struct {
	Command string
	Stderr  string
	Err     error
}

func (e *ExitError) Error() string {
	stderr := strings.TrimSpace(e.Stderr)
	if stderr == "" {
		return fmt.Sprintf("%s: %v", e.Command, e.Err)
	}
	return fmt.Sprintf("%s: %v: %s", e.Command, e.Err, stderr)
}

func (e *ExitError) Unwrap() error { return e.Err }

func (c *Client) run(ctx context.Context, name string, args ...string) (string, error) {
	stdout, stderr, err := c.runner.Run(ctx, name, args...)
	if err != nil {
		return stdout, &ExitError{Command: name + " " + strings.Join(args, " "), Stderr: stderr, Err: err}
	}
	return stdout, nil
}

// --- Environment checks (FR-01) ---

// DockerVersion returns the Docker Engine server version string.
func (c *Client) DockerVersion(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	return strings.TrimSpace(out), err
}

// ComposeVersion returns the `docker compose` plugin version string.
func (c *Client) ComposeVersion(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "docker", "compose", "version", "--short")
	return strings.TrimSpace(out), err
}

// DaemonReachable returns nil if the Docker daemon responds to `docker info`.
func (c *Client) DaemonReachable(ctx context.Context) error {
	_, err := c.run(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	return err
}

// --- Networks & volumes (FR-02) ---

// NetworkExists reports whether a Docker network with the exact given name exists.
func (c *Client) NetworkExists(ctx context.Context, name string) (bool, error) {
	out, err := c.run(ctx, "docker", "network", "ls", "--filter", "name=^"+name+"$", "--format", "{{.Name}}")
	if err != nil {
		return false, err
	}
	return containsLine(out, name), nil
}

// EnsureNetwork creates the named bridge network if it does not already exist.
func (c *Client) EnsureNetwork(ctx context.Context, name string) error {
	exists, err := c.NetworkExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = c.run(ctx, "docker", "network", "create", name)
	return err
}

// VolumeExists reports whether a Docker volume with the exact given name exists.
func (c *Client) VolumeExists(ctx context.Context, name string) (bool, error) {
	out, err := c.run(ctx, "docker", "volume", "ls", "--filter", "name=^"+name+"$", "--format", "{{.Name}}")
	if err != nil {
		return false, err
	}
	return containsLine(out, name), nil
}

// EnsureVolume creates the named volume if it does not already exist.
func (c *Client) EnsureVolume(ctx context.Context, name string) error {
	exists, err := c.VolumeExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = c.run(ctx, "docker", "volume", "create", name)
	return err
}

func containsLine(output, target string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}

// --- Compose lifecycle (FR-02, FR-03) ---

// ComposeUp brings the stack up in detached mode, building nothing (images
// are pulled from the registry as declared in docker-compose.yml).
func (c *Client) ComposeUp(ctx context.Context) error {
	_, err := c.run(ctx, "docker", c.composeArgs("up", "-d", "--remove-orphans")...)
	return err
}

// ComposeDown stops and removes the stack's containers (but not the
// external volumes, which are managed separately and survive by design).
func (c *Client) ComposeDown(ctx context.Context) error {
	_, err := c.run(ctx, "docker", c.composeArgs("down")...)
	return err
}

// ComposeStart starts previously-created, stopped containers.
func (c *Client) ComposeStart(ctx context.Context) error {
	_, err := c.run(ctx, "docker", c.composeArgs("start")...)
	return err
}

// ComposeStop stops running containers without removing them.
func (c *Client) ComposeStop(ctx context.Context) error {
	_, err := c.run(ctx, "docker", c.composeArgs("stop")...)
	return err
}

// ComposeRestart restarts the stack's containers.
func (c *Client) ComposeRestart(ctx context.Context) error {
	_, err := c.run(ctx, "docker", c.composeArgs("restart")...)
	return err
}

// ComposeExec runs a command inside a running compose service and returns
// its combined stdout.
func (c *Client) ComposeExec(ctx context.Context, service string, cmdArgs ...string) (string, error) {
	args := c.composeArgs(append([]string{"exec", "-T", service}, cmdArgs...)...)
	return c.run(ctx, "docker", args...)
}

// ComposeLogs streams service logs to the given writer (FR: `campuscloud logs`).
func (c *Client) ComposeLogs(ctx context.Context, w io.Writer, service string, tail int, follow bool) error {
	rest := []string{"logs", "--tail", fmt.Sprintf("%d", tail)}
	if follow {
		rest = append(rest, "-f")
	}
	if service != "" {
		rest = append(rest, service)
	}
	return c.runner.Stream(ctx, nil, w, w, "docker", c.composeArgs(rest...)...)
}

// ComposeExecOut runs a command inside a running compose service and
// streams its stdout to w (used to pipe a database dump or a `tar` archive
// out of a container without buffering it in memory).
func (c *Client) ComposeExecOut(ctx context.Context, w io.Writer, service string, cmdArgs ...string) error {
	args := c.composeArgs(append([]string{"exec", "-T", service}, cmdArgs...)...)
	var stderr bytes.Buffer
	err := c.runner.Stream(ctx, nil, w, &stderr, "docker", args...)
	if err != nil {
		return &ExitError{Command: "docker " + strings.Join(args, " "), Stderr: stderr.String(), Err: err}
	}
	return nil
}

// ComposeExecIn runs a command inside a running compose service, feeding r
// to its stdin (used to restore a database dump or extract a `tar` archive
// into a container).
func (c *Client) ComposeExecIn(ctx context.Context, r io.Reader, service string, cmdArgs ...string) error {
	args := c.composeArgs(append([]string{"exec", "-T", service}, cmdArgs...)...)
	var stderr bytes.Buffer
	err := c.runner.Stream(ctx, r, io.Discard, &stderr, "docker", args...)
	if err != nil {
		return &ExitError{Command: "docker " + strings.Join(args, " "), Stderr: stderr.String(), Err: err}
	}
	return nil
}

// ComposeCp copies a file or directory between the host and a running
// compose service container (e.g. "nextcloud:/var/www/html/config/config.php"
// on one side and a host path on the other), matching `docker compose cp`.
func (c *Client) ComposeCp(ctx context.Context, src, dst string) error {
	_, err := c.run(ctx, "docker", c.composeArgs("cp", src, dst)...)
	return err
}

// --- Container introspection (FR-04, FR-05) ---

// ContainerSummary is one row of `docker compose ps`.
type ContainerSummary struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Health  string `json:"Health"`
}

// ComposePS lists the current state of every container in the stack.
func (c *Client) ComposePS(ctx context.Context) ([]ContainerSummary, error) {
	out, err := c.run(ctx, "docker", c.composeArgs("ps", "-a", "--format", "json")...)
	if err != nil {
		return nil, err
	}
	return parseJSONObjects[ContainerSummary](out)
}

// ContainerStats is one row of `docker stats --no-stream`.
type ContainerStats struct {
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
	NetIO    string `json:"NetIO"`
	BlockIO  string `json:"BlockIO"`
}

// Stats returns a one-shot resource-usage snapshot for the given containers
// (or every running container if none are named).
func (c *Client) Stats(ctx context.Context, containers ...string) ([]ContainerStats, error) {
	args := []string{"stats", "--no-stream", "--format", "json"}
	args = append(args, containers...)
	out, err := c.run(ctx, "docker", args...)
	if err != nil {
		return nil, err
	}
	return parseJSONObjects[ContainerStats](out)
}

// ContainerHealth returns the Docker health-check status ("healthy",
// "unhealthy", "starting", or "" if the container has no health check) for
// the named container.
func (c *Client) ContainerHealth(ctx context.Context, container string) (string, error) {
	out, err := c.run(ctx, "docker", "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{end}}", container)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ContainerRunning reports whether the named container is currently running.
func (c *Client) ContainerRunning(ctx context.Context, container string) (bool, error) {
	out, err := c.run(ctx, "docker", "inspect", "--format", "{{.State.Running}}", container)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

// parseJSONObjects parses docker CLI `--format json` output, which may be
// emitted either as a single JSON array or as newline-delimited JSON
// objects depending on the command and Docker version.
func parseJSONObjects[T any](output string) ([]T, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, nil
	}

	if trimmed[0] == '[' {
		var items []T
		if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
			return nil, fmt.Errorf("parsing docker JSON array output: %w", err)
		}
		return items, nil
	}

	var items []T
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parsing docker JSON line output %q: %w", line, err)
		}
		items = append(items, item)
	}
	return items, nil
}
