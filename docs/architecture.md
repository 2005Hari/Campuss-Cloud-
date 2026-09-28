# Architecture

## Components (PRD section 7 / 12)

```
                       User
                        |
                        v
              Nextcloud Web Application  (container: campuscloud-nextcloud)
                   |            |
                   v            v
               MariaDB    Persistent Storage
              (container:   (Docker volumes:
             campuscloud-    campuscloud_nextcloud_data,
               mariadb)       campuscloud_mariadb_data)
                   ^
                   |  internal-only Docker network (campuscloud_net)
                   |
        Docker API / Docker Compose CLI
                   |
                   v
           CampusCloud Go CLI (cmd/campuscloud)
             |      |      |       |
          Deploy  Monitor Backup  Health
          Manage         Recovery
```

- **Nextcloud** is the only service published to the host (default port
  `8080`, configurable via `CAMPUSCLOUD_HTTP_PORT`).
- **MariaDB** has no published port; it is reachable only from Nextcloud
  over `campuscloud_net`, per PRD section 18 (Security Requirements).
- **campuscloud_nextcloud_data** and **campuscloud_mariadb_data** are
  named Docker volumes, created once by the CLI (not by Compose — see
  below) and referenced as `external` in `docker/docker-compose.yml`, so
  they survive `campuscloud stop`/`start`, container replacement, and even
  a full `docker compose down`.

## Why the network/volumes are created outside Compose

FR-02 (Automated Deployment) explicitly separates "create the network and
volumes" from "start MariaDB and Nextcloud" as two steps, and PRD section 7
draws the network/volumes as infrastructure the Go CLI manages directly via
the Docker API/CLI. `campuscloud deploy` therefore:

1. Creates `campuscloud_net` (`docker network create`) if it doesn't exist.
2. Creates both volumes (`docker volume create`) if they don't exist.
3. Only then runs `docker compose up -d`, whose compose file declares the
   network/volumes as `external: true` rather than owning their lifecycle.

This means volumes are never deleted by a compose teardown, only by an
explicit, intentional action outside `campuscloud`'s normal command set.

## Go module layout

| Package | Responsibility |
|---|---|
| `internal/config` | Parses `config.yaml`, loads `.env`, layers environment-variable overrides, validates required secrets (FR-10) |
| `internal/dockercli` | Dependency-injectable wrapper around the `docker` / `docker compose` binaries — network/volume management, compose lifecycle, `exec`/`cp`, stats, inspection |
| `internal/monitoring` | Host CPU (via `/proc/stat` sampling), RAM (via `/proc/meminfo`), and storage (via `statfs`) usage, classified against configurable thresholds (FR-06) |
| `internal/health` | Composable `Checker`s (Docker daemon, container running/healthy, network, storage, HTTP, database connectivity) and aggregation into a `Report` (FR-07) |
| `internal/deployment` | `Doctor` (FR-01) and `Deploy` (FR-02): orchestrates the above into the full environment-check-then-deploy-then-wait-for-health flow |
| `internal/backup` | `Create`/`Restore` (FR-08/FR-09): database dump/restore, config and file archiving, checksummed manifests, retention pruning |
| `cmd/campuscloud` | Cobra CLI wiring each command (section 10) to the packages above |

Every package that talks to Docker does so through the `dockercli.Runner`
interface, which is a thin seam over `os/exec` — this is what lets the unit
test suite exercise deploy/health/backup logic deterministically without a
real Docker daemon (see the `*_test.go` files alongside each package).

## Non-functional properties (PRD section 15)

- **Reliability** — container restarts never touch the external volumes;
  only explicit backup/restore operations move data.
- **Security** — see `docs/backup-recovery.md` and the README's Security
  section.
- **Performance** — designed for a modest VM (2–4 CPU, 4–8 GB RAM, 30+ GB
  storage); `doctor` warns (not fails) below that.
- **Maintainability** — deployment, monitoring, Docker, backup, health, and
  configuration concerns are separated into their own Go packages.
- **Portability** — redeploying on a new Ubuntu VM is `git clone` + `.env`
  + `campuscloud deploy` (see `docs/installation.md`).
