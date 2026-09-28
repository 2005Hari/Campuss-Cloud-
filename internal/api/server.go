// Package api exposes CampusCloud's deployment, monitoring, health, and
// backup/restore functionality as an HTTP/JSON API — the backend for a
// web dashboard (PRD section 17, "Web-based admin dashboard"). It runs on
// the same host as Docker (it shells out to `docker`/`docker compose`
// exactly like the CLI does), while the dashboard itself can be deployed
// anywhere, e.g. Vercel, and simply calls this API over HTTPS.
//
// Every mutating endpoint (deploy, start, stop, restart, backup, restore)
// starts a background Job and returns immediately (202 Accepted); poll
// GET /api/v1/jobs/{id} for progress and the final result. Read-only
// endpoints (status, containers, health, config, backups, logs) respond
// directly.
package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/2005Hari/campuscloud/internal/config"
	"github.com/2005Hari/campuscloud/internal/dockercli"
)

// Server holds everything the API handlers need.
type Server struct {
	cfg    *config.Config
	client *dockercli.Client
	jobs   *JobManager

	token          string
	allowedOrigins []string
}

// Options configures a Server.
type Options struct {
	// Token is the bearer token every /api/v1/* request must present via
	// `Authorization: Bearer <token>`. Required — NewServer refuses to
	// build an unauthenticated control plane over Docker.
	Token string
	// AllowedOrigins lists the exact Origin values (e.g. your Vercel
	// deployment's URL) allowed to call this API from a browser. "*"
	// allows any origin. An empty list disables CORS headers entirely
	// (same-origin and non-browser clients still work).
	AllowedOrigins []string
}

// NewServer builds a Server. It returns an error if opts.Token is empty.
func NewServer(cfg *config.Config, client *dockercli.Client, opts Options) (*Server, error) {
	if strings.TrimSpace(opts.Token) == "" {
		return nil, errEmptyToken
	}
	return &Server{
		cfg:            cfg,
		client:         client,
		jobs:           NewJobManager(),
		token:          opts.Token,
		allowedOrigins: opts.AllowedOrigins,
	}, nil
}

var errEmptyToken = apiError("refusing to start the API without an auth token (set --token or CAMPUSCLOUD_API_TOKEN)")

type apiError string

func (e apiError) Error() string { return string(e) }

// Handler builds the full, middleware-wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Unauthenticated liveness probe — lets a reverse proxy or the
	// frontend check the API is reachable at all before asking for a token.
	mux.HandleFunc("GET /healthz", s.handleLiveness)

	mux.HandleFunc("GET /api/v1/status", s.requireAuth(s.handleStatus))
	mux.HandleFunc("GET /api/v1/containers", s.requireAuth(s.handleContainers))
	mux.HandleFunc("GET /api/v1/health", s.requireAuth(s.handleHealth))
	mux.HandleFunc("GET /api/v1/doctor", s.requireAuth(s.handleDoctor))
	mux.HandleFunc("GET /api/v1/config", s.requireAuth(s.handleConfig))
	mux.HandleFunc("GET /api/v1/logs", s.requireAuth(s.handleLogs))
	mux.HandleFunc("GET /api/v1/backups", s.requireAuth(s.handleBackupsList))
	mux.HandleFunc("POST /api/v1/backups", s.requireAuth(s.handleBackupCreate))
	mux.HandleFunc("POST /api/v1/restore", s.requireAuth(s.handleRestore))
	mux.HandleFunc("POST /api/v1/deploy", s.requireAuth(s.handleDeploy))
	mux.HandleFunc("POST /api/v1/start", s.requireAuth(s.handleStart))
	mux.HandleFunc("POST /api/v1/stop", s.requireAuth(s.handleStop))
	mux.HandleFunc("POST /api/v1/restart", s.requireAuth(s.handleRestart))
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.requireAuth(s.handleJobGet))

	return s.withCORS(mux)
}

// requireAuth wraps h so it only runs when the request carries a valid
// bearer token, compared in constant time to avoid leaking the token's
// length/prefix via response timing.
func (s *Server) requireAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			writeError(w, http.StatusUnauthorized, "missing or invalid API token")
			return
		}
		h(w, r)
	}
}

// withCORS adds CORS headers for requests from an allowed origin and
// answers preflight OPTIONS requests. With no allowed origins configured,
// it is a no-op passthrough.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	for _, allowed := range s.allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}
