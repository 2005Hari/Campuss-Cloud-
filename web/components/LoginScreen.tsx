"use client";

import { useState } from "react";
import { ping } from "@/lib/api";
import { useAuth } from "@/lib/auth";

const DEFAULT_API_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

export function LoginScreen() {
  const { login } = useAuth();
  const [baseUrl, setBaseUrl] = useState(DEFAULT_API_URL);
  const [token, setToken] = useState("");
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (!baseUrl.trim() || !token.trim()) {
      setError("Both the API URL and token are required.");
      return;
    }

    setChecking(true);
    const reachable = await ping(baseUrl.trim());
    setChecking(false);

    if (!reachable) {
      setError(
        "Could not reach that API URL. Check that `campuscloud serve` is running there and the URL is correct (including https://)."
      );
      return;
    }

    login({ baseUrl: baseUrl.trim(), token: token.trim() });
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4 dark:bg-gray-950">
      <form
        onSubmit={handleSubmit}
        className="w-full max-w-sm rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
      >
        <h1 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
          CampusCloud Dashboard
        </h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Connect to your <code>campuscloud serve</code> API.
        </p>

        <label className="mt-5 block text-sm font-medium text-gray-700 dark:text-gray-300">
          API URL
          <input
            type="url"
            required
            placeholder="https://campuscloud.example.edu:9090"
            value={baseUrl}
            onChange={(e) => setBaseUrl(e.target.value)}
            className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-950"
          />
        </label>

        <label className="mt-4 block text-sm font-medium text-gray-700 dark:text-gray-300">
          API token
          <input
            type="password"
            required
            placeholder="CAMPUSCLOUD_API_TOKEN"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-950"
          />
        </label>

        {error && <p className="mt-3 text-sm text-red-600 dark:text-red-400">{error}</p>}

        <button
          type="submit"
          disabled={checking}
          className="mt-5 w-full rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50 dark:bg-gray-100 dark:text-gray-900 dark:hover:bg-white"
        >
          {checking ? "Connecting…" : "Connect"}
        </button>

        <p className="mt-4 text-xs text-gray-400">
          The token is stored only in this browser&apos;s local storage and sent directly to your
          API URL — never to any third party.
        </p>
      </form>
    </div>
  );
}
