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

  // /api/v1/health always responds 200 with a full report — even when
  // every check fails, that's "overall": "fail" in the body, not an HTTP
  // error — so anything caught here is a genuine request failure (API
  // unreachable, bad token, etc.), not a normal failing health check.
  const check = useCallback(async () => {
    setLoading(true);
    try {
      const r = await api.health(credentials);
      setReport(r);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
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
