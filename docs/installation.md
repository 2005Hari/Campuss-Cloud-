# Installation & Deployment

This walks through the full CampusCloud Core User Journey (PRD section 6),
from a bare Ubuntu VM to a working, monitored Nextcloud deployment.

## 1. Prepare the host

- Ubuntu 22.04+ VM (VirtualBox, VMware, or bare metal), ~2–4 CPU cores,
  4–8 GB RAM, 30+ GB storage.
- Install Docker Engine and the Compose plugin:

  ```bash
  curl -fsSL https://get.docker.com | sh
  sudo usermod -aG docker "$USER"   # log out/in for this to take effect
  ```

- Install Go 1.24+ if you intend to build `campuscloud` from source (a
  prebuilt binary can also just be copied over).

## 2. Get CampusCloud

```bash
git clone https://github.com/2005Hari/Campuss-Cloud- campuscloud
cd campuscloud
go build -o bin/campuscloud ./cmd/campuscloud
```

## 3. Configure secrets

```bash
cp .env.example .env
$EDITOR .env
```

Set real values for `MYSQL_ROOT_PASSWORD`, `MYSQL_PASSWORD`, and
`NEXTCLOUD_ADMIN_PASSWORD`. **Never commit `.env`** — it is already listed
in `.gitignore`.

Structural settings (network name, ports, thresholds, backup retention)
live in `config/config.yaml` and can be edited directly; they contain no
secrets.

## 4. Validate the environment

```bash
./bin/campuscloud doctor
```

This checks Docker, Docker Compose, the daemon, available RAM, and
available storage (FR-01). Resolve anything reported `FAIL` before
continuing — a `WARN` (e.g. less RAM than recommended) is fine on a modest
VM.

## 5. Deploy

```bash
./bin/campuscloud deploy
```

`deploy` (FR-02) re-runs the doctor checks, creates the `campuscloud_net`
Docker network and the `campuscloud_nextcloud_data` /
`campuscloud_mariadb_data` volumes if they don't already exist, starts
MariaDB and Nextcloud via Docker Compose, waits for both to report
healthy, and prints the application URL (default `http://localhost:8080`).

## 6. Finish the Nextcloud setup

Open the printed URL in a browser and log in with the
`NEXTCLOUD_ADMIN_USER` / `NEXTCLOUD_ADMIN_PASSWORD` from `.env` — the
Nextcloud container's first-run bootstrap already creates that account and
the initial database schema, so there is no separate setup wizard to fill
in.

## 7. Create groups and project workspaces

```bash
./scripts/setup-groups.sh
./scripts/create-workspace.sh <username> "Capstone-Project"
```

See [`docs/user-groups.md`](user-groups.md) and
[`docs/project-workspace.md`](project-workspace.md) for details (FR-11 /
FR-12).

## 8. Monitor and operate

```bash
./bin/campuscloud status        # containers, CPU/RAM/storage, uptime
./bin/campuscloud containers    # per-container resource usage
./bin/campuscloud health        # full health report
./bin/campuscloud logs -f       # follow logs
```

## 9. Backup and recovery

See [`docs/backup-recovery.md`](backup-recovery.md).

## Redeploying on another VM

CampusCloud is designed to be portable (PRD section 15, Non-Functional
Requirements): repeat steps 1–5 on a fresh Ubuntu VM with the same repo and
a new `.env`, and — if you're carrying data over — restore the most recent
backup (step 9) once the fresh stack is healthy.
