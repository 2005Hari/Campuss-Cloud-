"use client";

import { useCallback, useEffect, useState } from "react";
import { api, ApiError, type Credentials } from "@/lib/api";
import type { JobView } from "@/lib/types";

const POLL_INTERVAL_MS = 1500;

/**
 * Runs a job-starting API call, then polls GET /api/v1/jobs/{id} until the
 * job leaves the "running" state. Deploy/backup/restore/etc. can take
 * minutes, so the API returns immediately with a job id (202 Accepted)
 * rather than holding the HTTP request open.
 */
export function useJobPoller(credentials: Credentials | null) {
  const [jobId, setJobId] = useState<string | null>(null);
  const [job, setJob] = useState<JobView | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Re-fetches the job on a timer for as long as it's still running. This
  // effect intentionally depends only on [credentials, jobId] — NOT on
  // job.status — so it sets up exactly one recurring interval per job and
  // keeps ticking through every "still running" poll; the interval stops
  // itself (via clearInterval) once a poll reports a terminal status,
  // rather than relying on a dependency-array change that, while status
  // stays "running", would never happen.
  useEffect(() => {
    if (!credentials || !jobId) return;

    let cancelled = false;
    const interval = setInterval(() => {
      api
        .job(credentials, jobId)
        .then((j) => {
          if (cancelled) return;
          setJob(j);
          if (j.status !== "running") clearInterval(interval);
        })
        .catch((err) => {
          if (!cancelled) {
            setError(err instanceof Error ? err.message : String(err));
            clearInterval(interval);
          }
        });
    }, POLL_INTERVAL_MS);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [credentials, jobId]);

  const run = useCallback(
    async (starter: (creds: Credentials) => Promise<JobView>) => {
      if (!credentials) return;
      setError(null);
      setJob(null);
      setJobId(null);
      try {
        const started = await starter(credentials);
        setJob(started);
        setJobId(started.id);
      } catch (err) {
        setError(err instanceof ApiError ? err.message : String(err));
      }
    },
    [credentials]
  );

  const reset = useCallback(() => {
    setJobId(null);
    setJob(null);
    setError(null);
  }, []);

  return { job, error, isRunning: job?.status === "running", run, reset };
}
