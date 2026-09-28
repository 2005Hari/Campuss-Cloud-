package api

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func waitForStatus(t *testing.T, m *JobManager, id string, want JobStatus, timeout time.Duration) JobView {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		job, ok := m.Get(id)
		if !ok {
			t.Fatalf("job %s not found", id)
		}
		v := job.View()
		if v.Status == want {
			return v
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for job %s to reach %s, last status %s", id, want, v.Status)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestJobManagerSucceeds(t *testing.T) {
	m := NewJobManager()
	job := m.Start("noop", func(ctx context.Context, out io.Writer) error {
		io.WriteString(out, "doing work\n")
		return nil
	})

	v := waitForStatus(t, m, job.ID, JobSucceeded, time.Second)
	if v.Output != "doing work\n" {
		t.Errorf("output = %q", v.Output)
	}
	if v.Error != "" {
		t.Errorf("expected no error, got %q", v.Error)
	}
	if v.EndedAt == nil {
		t.Error("expected EndedAt to be set")
	}
}

func TestJobManagerFailure(t *testing.T) {
	m := NewJobManager()
	job := m.Start("noop", func(ctx context.Context, out io.Writer) error {
		io.WriteString(out, "about to fail\n")
		return errors.New("boom")
	})

	v := waitForStatus(t, m, job.ID, JobFailed, time.Second)
	if v.Error != "boom" {
		t.Errorf("error = %q, want boom", v.Error)
	}
	if v.Output != "about to fail\n" {
		t.Errorf("output = %q", v.Output)
	}
}

func TestJobManagerGetUnknown(t *testing.T) {
	m := NewJobManager()
	if _, ok := m.Get("does-not-exist"); ok {
		t.Error("expected ok=false for unknown job id")
	}
}

func TestJobIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := newJobID()
		if seen[id] {
			t.Fatalf("duplicate job id generated: %s", id)
		}
		seen[id] = true
	}
}

func TestJobViewBeforeCompletion(t *testing.T) {
	m := NewJobManager()
	block := make(chan struct{})
	job := m.Start("slow", func(ctx context.Context, out io.Writer) error {
		io.WriteString(out, "started\n")
		<-block
		return nil
	})

	v := job.View()
	if v.Status != JobRunning {
		t.Errorf("status = %v, want running", v.Status)
	}
	if v.EndedAt != nil {
		t.Error("expected EndedAt to be nil while running")
	}
	close(block)
	waitForStatus(t, m, job.ID, JobSucceeded, time.Second)
}
