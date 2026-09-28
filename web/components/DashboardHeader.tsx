"use client";

import { useAuth } from "@/lib/auth";

export function DashboardHeader() {
  const { credentials, logout } = useAuth();

  return (
    <header className="flex items-center justify-between border-b border-gray-200 bg-white px-6 py-4 dark:border-gray-800 dark:bg-gray-900">
      <div>
        <h1 className="text-base font-semibold text-gray-900 dark:text-gray-100">
          CampusCloud Dashboard
        </h1>
        <p className="text-xs text-gray-500 dark:text-gray-400">{credentials?.baseUrl}</p>
      </div>
      <button
        onClick={logout}
        className="rounded-md border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
      >
        Disconnect
      </button>
    </header>
  );
}
