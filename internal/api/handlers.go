package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/2005Hari/campuscloud/internal/backup"
	"github.com/2005Hari/campuscloud/internal/deployment"
	"github.com/2005Hari/campuscloud/internal/monitoring"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// usageView is monitoring.Usage flattened for JSON, tolerating a failed
// read (e.g. /proc unavailable) without failing the whole response.
type usageView struct {
	Percent float64 `json:"percent"`
	Used    uint64  `json:"used_bytes"`
	Total   uint64  `json:"total_bytes"`
	State   string  `json:"state"`
	Error   string  `json:"error,omitempty"`
}

func toUsageView(u monitoring.Usage, err error) usageView {
	if err != nil {
		return usageView{Error: err.Error()}
	}
	return usageView{Percent: u.Percent, Used: u.Used, Total: u.Total, State: string(u.State)}
}

type statusResponse struct {
	Containers []dockerContainer `json:"containers"`
	CPU        usageView         `json:"cpu"`
	Memory     usageView         `json:"memory"`
	Storage    usageView         `json:"storage"`
}

// dockerContainer mirrors dockercli.ContainerSummary; kept as its own type
// so the API's JSON shape doesn't silently change if that struct does.
type dockerContainer struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	State   string `json:"state"`
	Status  string `json:"status"`
	Health  string `json:"health"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	containers, err := s.client.ComposePS(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	t := s.cfg.Monitoring.Thresholds
	cpu, cpuErr := monitoring.HostCPU(200*time.Millisecond, t)
	mem, memErr := monitoring.HostMemory(t)
	storage, stErr := monitoring.HostStorage(s.cfg.Backup.Dir, t)

	resp := statusResponse{CPU: toUsageView(cpu, cpuErr), Memory: toUsageView(mem, memErr), Storage: toUsageView(storage, stErr)}
	for _, c := range containers {
		resp.Containers = append(resp.Containers, dockerContainer{Name: c.Name, Service: c.Service, State: c.State, Status: c.Status, Health: c.Health})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	containers, err := s.client.ComposePS(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	var names []string
	for _, c := range containers {
		names = append(names, c.Name)
	}
	rawStats, statsErr := s.client.Stats(ctx, names...)

	views := make([]dockerContainer, 0, len(containers))
	for _, c := range containers {
		views = append(views, dockerContainer{Name: c.Name, Service: c.Service, State: c.State, Status: c.Status, Health: c.Health})
	}
	stats := make([]containerStatsView, 0, len(rawStats))
	for _, s := range rawStats {
		stats = append(stats, containerStatsView{
			Name: s.Name, CPUPercent: s.CPUPerc, MemUsage: s.MemUsage,
			MemPercent: s.MemPerc, NetIO: s.NetIO, BlockIO: s.BlockIO,
		})
	}

	resp := map[string]any{"containers": views, "stats": stats}
	if statsErr != nil {
		resp["stats_error"] = statsErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// containerStatsView is dockercli.ContainerStats with lowercase JSON tags,
// consistent with the rest of this API (dockercli.ContainerStats itself
// uses Docker CLI's own capitalized field names, since that's what
// `docker stats --format json` actually emits).
type containerStatsView struct {
	Name       string `json:"name"`
	CPUPercent string `json:"cpu_percent"`
	MemUsage   string `json:"mem_usage"`
	MemPercent string `json:"mem_percent"`
	NetIO      string `json:"net_io"`
	BlockIO    string `json:"block_io"`
}

// handleHealth and handleDoctor always respond 200 OK: the HTTP status
// reflects whether the API successfully ran the checks, not whether the
// checks passed — that result lives in the body's "overall" field. A
// non-2xx here would otherwise be indistinguishable, to a generic HTTP
// client, from the request itself failing, which would discard the
// detailed per-check report the caller actually wants on a failure.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	report := deployment.FullHealth(r.Context(), s.cfg, s.client)
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	report := deployment.Doctor(r.Context(), s.cfg, s.client)
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg
	writeJSON(w, http.StatusOK, map[string]any{
		"project_name":     cfg.Project.Name,
		"network_name":     cfg.Network.Name,
		"http_port":        cfg.Nextcloud.HTTPPort,
		"nextcloud_health": cfg.Nextcloud.HealthPath,
		"thresholds":       cfg.Monitoring.Thresholds,
		"backup_dir":       cfg.Backup.Dir,
		"backup_retain":    cfg.Backup.Retain,
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	tail := 200
	if v := r.URL.Query().Get("tail"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			tail = n
		}
	}

	var buf bytes.Buffer
	if err := s.client.ComposeLogs(r.Context(), &buf, service, tail, false); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"logs": buf.String()})
}

func (s *Server) handleBackupsList(w http.ResponseWriter, r *http.Request) {
	backups, err := backup.List(s.cfg.Backup.Dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if backups == nil {
		backups = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
}

// --- Jobs: deploy, start, stop, restart, backup, restore ---

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	job := s.jobs.Start("deploy", func(ctx context.Context, out io.Writer) error {
		return deployment.Deploy(ctx, s.cfg, s.client, out, deployment.DefaultOptions())
	})
	writeJSON(w, http.StatusAccepted, job.View())
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	job := s.jobs.Start("start", func(ctx context.Context, out io.Writer) error {
		return s.client.ComposeStart(ctx)
	})
	writeJSON(w, http.StatusAccepted, job.View())
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	job := s.jobs.Start("stop", func(ctx context.Context, out io.Writer) error {
		return s.client.ComposeStop(ctx)
	})
	writeJSON(w, http.StatusAccepted, job.View())
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	job := s.jobs.Start("restart", func(ctx context.Context, out io.Writer) error {
		return s.client.ComposeRestart(ctx)
	})
	writeJSON(w, http.StatusAccepted, job.View())
}

func (s *Server) handleBackupCreate(w http.ResponseWriter, r *http.Request) {
	job := s.jobs.Start("backup", func(ctx context.Context, out io.Writer) error {
		_, err := backup.Create(ctx, s.cfg, s.client, out)
		return err
	})
	writeJSON(w, http.StatusAccepted, job.View())
}

type restoreRequest struct {
	BackupID string `json:"backup_id"`
	Latest   bool   `json:"latest"`
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	var req restoreRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}
	}

	id := req.BackupID
	if req.Latest {
		backups, err := backup.List(s.cfg.Backup.Dir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if len(backups) == 0 {
			writeError(w, http.StatusBadRequest, "no backups found")
			return
		}
		id = backups[0]
	}
	if id == "" {
		writeError(w, http.StatusBadRequest, "backup_id or latest is required")
		return
	}

	dir := id
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(s.cfg.Backup.Dir, id)
	}

	job := s.jobs.Start("restore", func(ctx context.Context, out io.Writer) error {
		return backup.Restore(ctx, s.cfg, s.client, dir, out)
	})
	writeJSON(w, http.StatusAccepted, job.View())
}

func (s *Server) handleJobGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, ok := s.jobs.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job.View())
}
