// Mirrors the JSON shapes served by internal/api (see
// campuscloud/internal/api/handlers.go). Keep these in sync with the Go
// structs if the API response shape changes.

export type CheckStatus = "pass" | "warn" | "fail";

export interface HealthResult {
  name: string;
  status: CheckStatus;
  message: string;
}

export interface HealthReport {
  results: HealthResult[];
  overall: CheckStatus;
}

export interface UsageView {
  percent: number;
  used_bytes: number;
  total_bytes: number;
  state: "normal" | "warning" | "high" | "critical" | "";
  error?: string;
}

export interface ContainerSummary {
  name: string;
  service: string;
  state: string;
  status: string;
  health: string;
}

export interface StatusResponse {
  containers: ContainerSummary[];
  cpu: UsageView;
  memory: UsageView;
  storage: UsageView;
}

export interface ContainerStats {
  name: string;
  cpu_percent: string;
  mem_usage: string;
  mem_percent: string;
  net_io: string;
  block_io: string;
}

export interface ContainersResponse {
  containers: ContainerSummary[];
  stats: ContainerStats[] | null;
  stats_error?: string;
}

export interface ConfigResponse {
  project_name: string;
  network_name: string;
  http_port: number;
  nextcloud_health: string;
  thresholds: { Warning: number; High: number; Critical: number };
  backup_dir: string;
  backup_retain: number;
}

export type JobStatus = "running" | "succeeded" | "failed";

export interface JobView {
  id: string;
  kind: string;
  status: JobStatus;
  output: string;
  error?: string;
  started_at: string;
  ended_at?: string;
}

export interface BackupsResponse {
  backups: string[];
}

export interface LogsResponse {
  logs: string;
}
