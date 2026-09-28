"use client";

import { useState } from "react";
import { api, ApiError, type Credentials } from "@/lib/api";
import { Card } from "./Card";

const SERVICES = [
  { value: "", label: "All services" },
  { value: "nextcloud", label: "Nextcloud" },
  { value: "mariadb", label: "MariaDB" },
];

export function LogsSection({ credentials }: { credentials: Credentials }) {
  const [service, setService] = useState("");
  const [tail, setTail] = useState(200);
  const [logs, setLogs] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function fetchLogs() {
    setLoading(true);
    try {
      const res = await api.logs(credentials, service, tail);
      setLogs(res.logs);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Card title="Logs">
      <div className="flex flex-wrap items-end gap-3">
        <label className="text-sm text-gray-700 dark:text-gray-300">
          Service
          <select
            value={service}
            onChange={(e) => setService(e.target.value)}
            className="mt-1 block rounded-md border border-gray-300 px-2 py-1.5 text-sm dark:border-gray-700 dark:bg-gray-950"
          >
            {SERVICES.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm text-gray-700 dark:text-gray-300">
          Tail lines
          <input
            type="number"
            min={10}
            max={5000}
            value={tail}
            onChange={(e) => setTail(Number(e.target.value) || 200)}
            className="mt-1 block w-24 rounded-md border border-gray-300 px-2 py-1.5 text-sm dark:border-gray-700 dark:bg-gray-950"
          />
        </label>
        <button
          onClick={fetchLogs}
          disabled={loading}
          className="rounded-md bg-gray-900 px-4 py-1.5 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50 dark:bg-gray-100 dark:text-gray-900"
        >
          {loading ? "Loading…" : "Fetch logs"}
        </button>
      </div>

      {error && <p className="mt-3 text-sm text-red-600 dark:text-red-400">{error}</p>}

      {logs !== null && (
        <pre className="mt-4 max-h-96 overflow-auto rounded-lg bg-gray-950 p-4 text-xs text-gray-100">
          {logs || "(no output)"}
        </pre>
      )}
    </Card>
  );
}
