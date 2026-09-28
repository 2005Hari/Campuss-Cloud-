import type {
  BackupsResponse,
  ConfigResponse,
  ContainersResponse,
  HealthReport,
  JobView,
  LogsResponse,
  StatusResponse,
} from "./types";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

export interface Credentials {
  baseUrl: string;
  token: string;
}

function normalizeBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

async function request<T>(
  { baseUrl, token }: Credentials,
  path: string,
  init?: RequestInit
): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${normalizeBaseUrl(baseUrl)}${path}`, {
      ...init,
      headers: {
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        Authorization: `Bearer ${token}`,
        ...init?.headers,
      },
    });
  } catch {
    throw new ApiError(
      0,
      "Could not reach the CampusCloud API. Check the API URL and that campuscloud serve is running and reachable."
    );
  }

  if (!res.ok) {
    let message = `Request failed with status ${res.status}`;
    try {
      const body = await res.json();
      if (body && typeof body.error === "string") message = body.error;
    } catch {
      // response wasn't JSON; keep the generic message
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

/** Unauthenticated liveness probe — used to validate an API URL before asking for a token. */
export async function ping(baseUrl: string): Promise<boolean> {
  try {
    const res = await fetch(`${normalizeBaseUrl(baseUrl)}/healthz`);
    return res.ok;
  } catch {
    return false;
  }
}

export const api = {
  status: (creds: Credentials) => request<StatusResponse>(creds, "/api/v1/status"),
  containers: (creds: Credentials) => request<ContainersResponse>(creds, "/api/v1/containers"),
  health: (creds: Credentials) => request<HealthReport>(creds, "/api/v1/health"),
  doctor: (creds: Credentials) => request<HealthReport>(creds, "/api/v1/doctor"),
  config: (creds: Credentials) => request<ConfigResponse>(creds, "/api/v1/config"),
  backups: (creds: Credentials) => request<BackupsResponse>(creds, "/api/v1/backups"),
  logs: (creds: Credentials, service: string, tail: number) =>
    request<LogsResponse>(
      creds,
      `/api/v1/logs?${new URLSearchParams({ service, tail: String(tail) })}`
    ),

  deploy: (creds: Credentials) => request<JobView>(creds, "/api/v1/deploy", { method: "POST" }),
  start: (creds: Credentials) => request<JobView>(creds, "/api/v1/start", { method: "POST" }),
  stop: (creds: Credentials) => request<JobView>(creds, "/api/v1/stop", { method: "POST" }),
  restart: (creds: Credentials) => request<JobView>(creds, "/api/v1/restart", { method: "POST" }),
  createBackup: (creds: Credentials) =>
    request<JobView>(creds, "/api/v1/backups", { method: "POST" }),
  restore: (creds: Credentials, body: { backup_id?: string; latest?: boolean }) =>
    request<JobView>(creds, "/api/v1/restore", { method: "POST", body: JSON.stringify(body) }),

  job: (creds: Credentials, id: string) => request<JobView>(creds, `/api/v1/jobs/${id}`),
};
