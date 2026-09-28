"use client";

import { useCallback, useEffect, useState } from "react";
import { api, ApiError, type Credentials } from "@/lib/api";
import type { ContainerStats, StatusResponse } from "@/lib/types";
import { Card } from "./Card";
import { StatusBadge } from "./StatusBadge";
import { UsageGauge } from "./UsageGauge";

const REFRESH_MS = 8000;

export function OverviewSection({ credentials }: { credentials: Credentials }) {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [stats, setStats] = useState<ContainerStats[]>([]);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [statusRes, containersRes] = await Promise.all([
        api.status(credentials),
        api.containers(credentials),
      ]);
      setStatus(statusRes);
      setStats(containersRes.stats ?? []);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    }
  }, [credentials]);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, REFRESH_MS);
    return () => clearInterval(id);
  }, [refresh]);

  const statsByName = new Map(stats.map((s) => [s.name, s]));

  return (
    <Card
      title="Overview"
      action={
        <button
          onClick={refresh}
          className="text-xs font-medium text-gray-500 hover:text-gray-900 dark:hover:text-gray-100"
        >
          Refresh
        </button>
      }
    >
      {error && <p className="mb-4 text-sm text-red-600 dark:text-red-400">{error}</p>}

      {status && (
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-3">
          <UsageGauge label="CPU" usage={status.cpu} />
          <UsageGauge label="Memory" usage={status.memory} />
          <UsageGauge label="Storage" usage={status.storage} />
        </div>
      )}

      <div className="mt-6 overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b border-gray-100 text-xs uppercase text-gray-400 dark:border-gray-800">
              <th className="py-2 pr-4">Container</th>
              <th className="py-2 pr-4">State</th>
              <th className="py-2 pr-4">Health</th>
              <th className="py-2 pr-4">CPU</th>
              <th className="py-2 pr-4">Memory</th>
            </tr>
          </thead>
          <tbody>
            {(status?.containers ?? []).map((c) => {
              const s = statsByName.get(c.name);
              return (
                <tr key={c.name} className="border-b border-gray-50 dark:border-gray-900">
                  <td className="py-2 pr-4 font-medium text-gray-900 dark:text-gray-100">
                    {c.name}
                  </td>
                  <td className="py-2 pr-4">
                    <StatusBadge value={c.state} />
                  </td>
                  <td className="py-2 pr-4">
                    <StatusBadge value={c.health || "n/a"} />
                  </td>
                  <td className="py-2 pr-4 text-gray-600 dark:text-gray-400">
                    {s?.cpu_percent ?? "—"}
                  </td>
                  <td className="py-2 pr-4 text-gray-600 dark:text-gray-400">
                    {s?.mem_usage ?? "—"}
                  </td>
                </tr>
              );
            })}
            {status && status.containers.length === 0 && (
              <tr>
                <td colSpan={5} className="py-4 text-center text-gray-400">
                  No containers found. Run Deploy to get started.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </Card>
  );
}
