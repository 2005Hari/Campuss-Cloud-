import type { JobView } from "@/lib/types";
import { StatusBadge } from "./StatusBadge";

export function JobOutputPanel({ job, error }: { job: JobView | null; error: string | null }) {
  if (!job && !error) return null;

  return (
    <div className="mt-4 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-800 dark:bg-gray-950">
      {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
      {job && (
        <>
          <div className="flex items-center gap-2">
            <span className="text-xs font-medium text-gray-500 dark:text-gray-400">
              {job.kind}
            </span>
            <StatusBadge value={job.status} />
            {job.status === "running" && (
              <span className="text-xs text-gray-400">working…</span>
            )}
          </div>
          {job.output && (
            <pre className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap text-xs text-gray-700 dark:text-gray-300">
              {job.output}
            </pre>
          )}
          {job.error && <p className="mt-2 text-sm text-red-600 dark:text-red-400">{job.error}</p>}
        </>
      )}
    </div>
  );
}
