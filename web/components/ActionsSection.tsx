"use client";

import { useState } from "react";
import { api, type Credentials } from "@/lib/api";
import { useJobPoller } from "@/hooks/useJobPoller";
import { Card } from "./Card";
import { JobOutputPanel } from "./JobOutputPanel";

interface Action {
  label: string;
  kind: "primary" | "danger" | "neutral";
  confirm?: string;
  run: (creds: Credentials) => ReturnType<typeof api.deploy>;
}

const BUTTON_STYLES: Record<Action["kind"], string> = {
  primary: "bg-gray-900 text-white hover:bg-gray-800 dark:bg-gray-100 dark:text-gray-900",
  danger: "bg-red-600 text-white hover:bg-red-700",
  neutral:
    "border border-gray-300 text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800",
};

export function ActionsSection({ credentials }: { credentials: Credentials }) {
  const { job, error, isRunning, run } = useJobPoller(credentials);
  const [pendingConfirm, setPendingConfirm] = useState<Action | null>(null);

  const actions: Action[] = [
    { label: "Deploy", kind: "primary", run: api.deploy },
    { label: "Start", kind: "neutral", run: api.start },
    {
      label: "Stop",
      kind: "danger",
      confirm: "Stop the application services? Data is preserved, but Nextcloud will be unreachable.",
      run: api.stop,
    },
    {
      label: "Restart",
      kind: "neutral",
      confirm: "Restart the application services?",
      run: api.restart,
    },
  ];

  function trigger(action: Action) {
    if (action.confirm && !pendingConfirm) {
      setPendingConfirm(action);
      return;
    }
    setPendingConfirm(null);
    run(action.run);
  }

  return (
    <Card title="Actions">
      <div className="flex flex-wrap gap-2">
        {actions.map((action) => (
          <button
            key={action.label}
            disabled={isRunning}
            onClick={() => trigger(action)}
            className={`rounded-md px-4 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50 ${BUTTON_STYLES[action.kind]}`}
          >
            {action.label}
          </button>
        ))}
      </div>

      {pendingConfirm && (
        <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200">
          <p>{pendingConfirm.confirm}</p>
          <div className="mt-2 flex gap-2">
            <button
              onClick={() => trigger(pendingConfirm)}
              className="rounded-md bg-amber-600 px-3 py-1 text-xs font-medium text-white hover:bg-amber-700"
            >
              Confirm
            </button>
            <button
              onClick={() => setPendingConfirm(null)}
              className="rounded-md border border-amber-300 px-3 py-1 text-xs font-medium text-amber-800 hover:bg-amber-100 dark:text-amber-200"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      <JobOutputPanel job={job} error={error} />
    </Card>
  );
}
