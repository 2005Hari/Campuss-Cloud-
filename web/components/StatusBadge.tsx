const COLORS: Record<string, string> = {
  pass: "bg-emerald-100 text-emerald-800 border-emerald-300",
  normal: "bg-emerald-100 text-emerald-800 border-emerald-300",
  warn: "bg-amber-100 text-amber-800 border-amber-300",
  warning: "bg-amber-100 text-amber-800 border-amber-300",
  high: "bg-orange-100 text-orange-800 border-orange-300",
  fail: "bg-red-100 text-red-800 border-red-300",
  critical: "bg-red-100 text-red-800 border-red-300",
  running: "bg-blue-100 text-blue-800 border-blue-300",
  succeeded: "bg-emerald-100 text-emerald-800 border-emerald-300",
  failed: "bg-red-100 text-red-800 border-red-300",
};

export function StatusBadge({ value }: { value: string }) {
  const key = value.toLowerCase();
  const classes = COLORS[key] ?? "bg-gray-100 text-gray-700 border-gray-300";
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium ${classes}`}
    >
      {value || "unknown"}
    </span>
  );
}
