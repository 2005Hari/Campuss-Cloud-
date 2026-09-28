"use client";

import { useCallback, useEffect, useState } from "react";
import { api, ApiError, type Credentials } from "@/lib/api";
import type { HealthReport } from "@/lib/types";
import { Card } from "./Card";
import { StatusBadge } from "./StatusBadge";

export function HealthSection({ credentials }: { credentials: Credentials }) {
  const [report, setReport] = useState<HealthReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const check = useCallback(async () => {
    setLoading(true);
    try {
      const r = await api.health(credentials);
      setReport(r);
      setError(null);
    } catch (err) {
      // A failing health check is a normal, expected result (503) — the
      // API still returns a full report body in that case, but a network-
      // level failure (ApiError with no report) should surface as an error.
      if (err instanceof ApiError && err.status === 0) {
        setError(err.message);
      } else {
        setError(err instanceof Error ? err.message : String(err));
      }
    } finally {
      setLoading(false);
    }
  }, [credentials]);

  useEffect(() => {
    check();
  }, [check]);

  return (
    <Card
      title="Health"
      action={
        <button
          onClick={check}
          disabled={loading}
          className="text-xs font-medium text-gray-500 hover:text-gray-900 disabled:opacity-50 dark:hover:text-gray-100"
        >
          {loading ? "Checking…" : "Run health check"}
        </button>
      }
    >
      {error && <p className="mb-3 text-sm text-red-600 dark:text-red-400">{error}</p>}
      {report && (
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Overall:</span>
            <StatusBadge value={report.overall} />
          </div>
          <ul className="divide-y divide-gray-100 dark:divide-gray-800">
            {report.results.map((r) => (
              <li key={r.name} className="flex items-center justify-between gap-4 py-2 text-sm">
                <span className="font-medium text-gray-800 dark:text-gray-200">{r.name}</span>
                <span className="flex-1 truncate text-right text-gray-500 dark:text-gray-400">
                  {r.message}
                </span>
                <StatusBadge value={r.status} />
              </li>
            ))}
          </ul>
        </div>
      )}
    </Card>
  );
}
