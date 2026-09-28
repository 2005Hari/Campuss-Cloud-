import type { UsageView } from "@/lib/types";

const BAR_COLORS: Record<string, string> = {
  normal: "bg-emerald-500",
  warning: "bg-amber-500",
  high: "bg-orange-500",
  critical: "bg-red-500",
};

function formatBytes(bytes: number): string {
  if (!bytes) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  const exp = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** exp).toFixed(1)} ${units[exp]}`;
}

export function UsageGauge({ label, usage }: { label: string; usage: UsageView }) {
  if (usage.error) {
    return (
      <div>
        <p className="text-xs font-medium text-gray-500 dark:text-gray-400">{label}</p>
        <p className="mt-1 text-sm text-red-600 dark:text-red-400">{usage.error}</p>
      </div>
    );
  }

  const pct = Math.min(100, Math.max(0, usage.percent));
  const barColor = BAR_COLORS[usage.state] ?? "bg-gray-400";

  return (
    <div>
      <div className="flex items-baseline justify-between">
        <p className="text-xs font-medium text-gray-500 dark:text-gray-400">{label}</p>
        <p className="text-xs text-gray-500 dark:text-gray-400">{usage.state}</p>
      </div>
      <p className="mt-1 text-2xl font-semibold text-gray-900 dark:text-gray-100">
        {pct.toFixed(1)}%
      </p>
      <div className="mt-2 h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
        <div className={`h-full rounded-full ${barColor}`} style={{ width: `${pct}%` }} />
      </div>
      <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {formatBytes(usage.used_bytes)} / {formatBytes(usage.total_bytes)}
      </p>
    </div>
  );
}
