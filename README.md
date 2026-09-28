# CampusCloud

**Automated deployment, configuration & management of a containerized private cloud.**

CampusCloud is a self-hosted private cloud platform for colleges and student
teams. It combines [Nextcloud](https://nextcloud.com/) (centralized file
storage and collaboration) with a custom **Go CLI** — `campuscloud` — that
automates deployment, monitoring, backup, and recovery of the underlying
Docker infrastructure.

| | |
|---|---|
| Project Type | Academic Mini Project / Group Project |
| Domain | Cloud Computing + Containerization + Automation |
| Primary Technologies | Go, Docker, Docker Compose, Nextcloud, MariaDB, Ubuntu |
| Target Users | Students, Faculty, Clubs, Project Teams, College Administrators |

## Why CampusCloud

Students and college organizations frequently manage project files through
personal cloud accounts, chat apps, email, USB drives, and individual
laptops — leading to scattered files, unclear ownership, and weak backup
practices. CampusCloud replaces that with a centralized, college-controlled
private cloud that a single command can deploy, monitor, and recover.

## Architecture

```
 User
  |
  v
 Nextcloud (web app, container)  <---->  MariaDB (container, internal network only)
  |                                             |
  +---------------------+----------------------+
                         |
                Docker network + persistent volumes
                         ^
                         |
              Docker CLI / Docker Compose CLI
                         |
                 CampusCloud Go CLI
                 (cmd/campuscloud)
                    |     |     |
                 deploy monitor backup
                 manage health  recovery
```

MariaDB is never published to the host — it is reachable only from
Nextcloud, over an internal Docker network — per the project's security
requirements. Both the Nextcloud application data and the MariaDB data
directory live on named Docker volumes that survive container restarts,
recreation, or a full `campuscloud deploy` re-run.

See [`docs/architecture.md`](docs/architecture.md) for the full breakdown.

## Requirements

- Ubuntu (or another Linux) VM/host with **~2–4 CPU cores, 4–8 GB RAM, 30+ GB storage**
- [Docker Engine](https://docs.docker.com/engine/install/) and the `docker compose` plugin
- Go 1.24+ (only if you want to build `campuscloud` from source)

Run `campuscloud doctor` at any time to check these automatically.

## Quickstart

```bash
git clone https://github.com/2005Hari/Campuss-Cloud- campuscloud
cd campuscloud

# 1. Build the CLI
go build -o bin/campuscloud ./cmd/campuscloud

# 2. Configure secrets (never commit this file)
cp .env.example .env
$EDITOR .env

# 3. Check the environment is ready
./bin/campuscloud doctor

# 4. Deploy Nextcloud + MariaDB
./bin/campuscloud deploy

# 5. Open the printed URL (default: http://localhost:8080) and finish
#    the Nextcloud setup wizard using the admin credentials from .env.
```

Full step-by-step instructions (including provisioning the standard user
groups and project-workspace folders) are in
[`docs/installation.md`](docs/installation.md).

## CLI Commands

| Command | Purpose |
|---|---|
| `campuscloud doctor` | Verify Docker, Compose, the daemon, RAM, and storage (FR-01) |
| `campuscloud deploy` | Validate the environment and deploy the stack (FR-02) |
| `campuscloud start` / `stop` / `restart` | Manage the running services (FR-03) |
| `campuscloud status` | Container state, CPU/RAM/storage, uptime, health (FR-04) |
| `campuscloud containers` | List containers and live resource usage (FR-05) |
| `campuscloud health` | Full health check across Docker, Nextcloud, MariaDB, network, HTTP (FR-07) |
| `campuscloud logs [service]` | Tail or follow service logs |
| `campuscloud backup` / `backup list` | Back up the database, config, and files (FR-08) |
| `campuscloud restore <id\|--latest>` | Validate and restore a backup (FR-09) |
| `campuscloud config [--validate]` | Show / validate the effective configuration (FR-10) |
| `campuscloud serve` | Run the HTTP API behind the web dashboard (see below) |
| `campuscloud version` | Print the CLI version |

Every command accepts `--config <path>` (default `config/config.yaml`) and
`--env <path>` (default `.env`).

## Web Dashboard

Everything above is also available from a browser: `campuscloud serve`
exposes a token-authenticated HTTP API, and `web/` is a Next.js dashboard
that talks to it — deployable straight to **Vercel**, since it's a plain
client-rendered app with no secrets of its own (the API token is entered
by the admin at login and kept only in their browser).

```bash
./bin/campuscloud serve --token "$(openssl rand -hex 32)" \
  --cors-origin https://your-dashboard.vercel.app
```

See [`docs/dashboard.md`](docs/dashboard.md) for the full setup (systemd
unit, HTTPS reverse proxy, CORS) and [`web/README.md`](web/README.md) for
deploying the dashboard itself.

## Project Structure

```
campuscloud/
├── cmd/campuscloud/       CLI entrypoint and command definitions (cobra), incl. `serve`
├── internal/
│   ├── config/            YAML + environment-variable configuration (FR-10)
│   ├── dockercli/         docker / docker compose CLI wrapper
│   ├── deployment/        doctor (FR-01) and deploy (FR-02) orchestration
│   ├── monitoring/        CPU / RAM / storage sampling + thresholds (FR-06)
│   ├── health/            composable health checks (FR-07)
│   ├── backup/            backup + restore (FR-08 / FR-09)
│   └── api/               HTTP/JSON API behind `campuscloud serve` — the web dashboard's backend
├── web/                    Next.js admin dashboard (deployable to Vercel)
├── config/config.yaml      structural defaults (no secrets)
├── docker/docker-compose.yml
├── scripts/                Nextcloud provisioning helpers (FR-11 / FR-12)
├── docs/                   installation, architecture, backup/recovery, dashboard docs
├── tests/integration/      Docker-dependent end-to-end tests (build tag)
├── .env.example
└── go.mod
```

## Monitoring thresholds

| Usage | State |
|---|---|
| < 70% | Normal |
| 70–80% | Warning |
| 80–90% | High |
| > 90% | Critical |

## Security

- Credentials live only in `.env` (git-ignored) or the real process
  environment — never in `config/config.yaml` or hardcoded in Go.
- MariaDB is not exposed on the host; only Nextcloud's HTTP port is published.
- Database dumps and restores use `MYSQL_PWD` so credentials never appear
  as a process argument.

See [`docs/backup-recovery.md`](docs/backup-recovery.md) for the backup
format and restore procedure, and [`docs/user-groups.md`](docs/user-groups.md)
/ [`docs/project-workspace.md`](docs/project-workspace.md) for provisioning
users, groups, and standardized project folders.

## Testing

```bash
go test ./...          # unit tests (no Docker required)
go vet ./...
```

Unit tests cover configuration parsing, resource-threshold classification,
storage/CPU/memory calculations, health-check aggregation, and backup
manifest validation — all without needing a live Docker daemon. Tests that
exercise a real deployment (stop a container, verify degraded health,
restart, recover) live under `tests/integration/` behind a build tag; see
that directory's README for how to run them on an actual VM.

## License

[MIT](LICENSE)
