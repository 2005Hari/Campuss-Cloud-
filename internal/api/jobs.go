package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"sync"
	"time"
)

// JobTimeout bounds how long a background job may run before its context
// is canceled. Deploy/backup/restore involve image pulls and large file
// transfers, so this is generous rather than tight.
const JobTimeout = 15 * time.Minute

// JobStatus is the lifecycle state of a background job (deploy, backup,
// restore, ...) started through the API.
type JobStatus string

const (
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
)

// Job tracks one long-running operation triggered over HTTP. Mutating
// CLI operations (deploy, backup, restore, ...) can take minutes — well
// past any sane HTTP request timeout — so the API starts them in the
// background and hands back a Job the client polls via GET /jobs/{id}.
// Job itself implements io.Writer so the same functions that print
// progress to stdout on the CLI can write into a Job's buffer instead.
type Job struct {
	ID        string
	Kind      string
	StartedAt time.Time

	mu      sync.Mutex
	status  JobStatus
	buf     bytes.Buffer
	errMsg  string
	endedAt time.Time
}

// Write appends to the job's captured output. Safe for concurrent use
// with View, since both take the same mutex.
func (j *Job) Write(p []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.buf.Write(p)
}

func (j *Job) finish(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.endedAt = time.Now().UTC()
	if err != nil {
		j.status = JobFailed
		j.errMsg = err.Error()
		return
	}
	j.status = JobSucceeded
}

// JobView is Job's JSON-serializable snapshot.
type JobView struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	Status    JobStatus  `json:"status"`
	Output    string     `json:"output"`
	Error     string     `json:"error,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// View takes a consistent snapshot of the job's current state.
func (j *Job) View() JobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	v := JobView{
		ID:        j.ID,
		Kind:      j.Kind,
		Status:    j.status,
		Output:    j.buf.String(),
		Error:     j.errMsg,
		StartedAt: j.StartedAt,
	}
	if !j.endedAt.IsZero() {
		t := j.endedAt
		v.EndedAt = &t
	}
	return v
}

// JobManager tracks every job started since the server process began.
// Jobs are kept in memory only — restarting `campuscloud serve` clears
// history, which is acceptable for an admin-triggered action log.
type JobManager struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

// NewJobManager returns an empty JobManager.
func NewJobManager() *JobManager {
	return &JobManager{jobs: make(map[string]*Job)}
}

// Start launches fn in the background and returns immediately with a Job
// the caller can poll. fn should write human-readable progress to out (the
// job itself, which implements io.Writer) and respect ctx's deadline.
func (m *JobManager) Start(kind string, fn func(ctx context.Context, out io.Writer) error) *Job {
	job := &Job{ID: newJobID(), Kind: kind, StartedAt: time.Now().UTC(), status: JobRunning}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), JobTimeout)
		defer cancel()
		err := fn(ctx, job)
		job.finish(err)
	}()

	return job
}

// Get looks up a job by ID.
func (m *JobManager) Get(id string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	return j, ok
}

func newJobID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is effectively unrecoverable; fall back to a
		// timestamp so job creation never panics.
		return hex.EncodeToString([]byte(time.Now().UTC().String()))
	}
	return hex.EncodeToString(b[:])
}
