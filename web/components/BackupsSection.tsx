"use client";

import { useCallback, useEffect, useState } from "react";
import { api, ApiError, type Credentials } from "@/lib/api";
import { useJobPoller } from "@/hooks/useJobPoller";
import { Card } from "./Card";
import { JobOutputPanel } from "./JobOutputPanel";

export function BackupsSection({ credentials }: { credentials: Credentials }) {
  const [backups, setBackups] = useState<string[]>([]);
  const [listError, setListError] = useState<string | null>(null);
  const [confirmTarget, setConfirmTarget] = useState<string | null>(null);
  const { job, error, isRunning, run } = useJobPoller(credentials);

  const refresh = useCallback(async () => {
    try {
      const res = await api.backups(credentials);
      setBackups(res.backups);
      setListError(null);
    } catch (err) {
      setListError(err instanceof ApiError ? err.message : String(err));
    }
  }, [credentials]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Once a backup/restore job finishes, refresh the list (a new backup
  // appears; nothing to add for restore, but it's a cheap no-op then).
  useEffect(() => {
    if (job && job.status !== "running") refresh();
  }, [job, refresh]);

  function createBackup() {
    run(api.createBackup);
  }

  function restore(id: string) {
    if (confirmTarget !== id) {
      setConfirmTarget(id);
      return;
    }
    setConfirmTarget(null);
    run((creds) => api.restore(creds, { backup_id: id }));
  }

  return (
    <Card
      title="Backups"
      action={
        <button
          onClick={createBackup}
          disabled={isRunning}
          className="rounded-md bg-gray-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-gray-800 disabled:opacity-50 dark:bg-gray-100 dark:text-gray-900"
        >
          Create backup
        </button>
      }
    >
      {listError && <p className="mb-3 text-sm text-red-600 dark:text-red-400">{listError}</p>}

      {backups.length === 0 ? (
        <p className="text-sm text-gray-400">No backups yet.</p>
      ) : (
        <ul className="divide-y divide-gray-100 dark:divide-gray-800">
          {backups.map((id) => (
            <li key={id} className="flex items-center justify-between gap-4 py-2 text-sm">
              <span className="font-mono text-gray-700 dark:text-gray-300">{id}</span>
              {confirmTarget === id ? (
                <div className="flex items-center gap-2">
                  <span className="text-xs text-amber-700 dark:text-amber-400">
                    Restore this backup? The app will stop briefly.
                  </span>
                  <button
                    onClick={() => restore(id)}
                    disabled={isRunning}
                    className="rounded-md bg-amber-600 px-2 py-1 text-xs font-medium text-white hover:bg-amber-700 disabled:opacity-50"
                  >
                    Confirm
                  </button>
                  <button
                    onClick={() => setConfirmTarget(null)}
                    className="rounded-md border border-gray-300 px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300"
                  >
                    Cancel
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => restore(id)}
                  disabled={isRunning}
                  className="rounded-md border border-gray-300 px-3 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
                >
                  Restore
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      <JobOutputPanel job={job} error={error} />
    </Card>
  );
}
