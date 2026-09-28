# Web Dashboard (PRD section 17: Web-based admin dashboard)

The `web/` directory is a Next.js dashboard, deployable to Vercel, for
deploying, monitoring, and backing up CampusCloud from a browser instead
of the CLI. It talks to a small HTTP API — `campuscloud serve` — that runs
on your VM, right alongside Docker.

```
 Browser  --https-->  Vercel (Next.js dashboard, static/client-rendered)
                              |
                              | HTTPS + Bearer token
                              v
                    Your VM: campuscloud serve  --docker/compose-->  Nextcloud + MariaDB
```

The dashboard itself holds no secrets and runs no server code of its own —
it only needs the API's URL and a bearer token, both entered by the admin
at login and kept in the browser's local storage.

## 1. Run `campuscloud serve` on the VM

```bash
export CAMPUSCLOUD_API_TOKEN=$(openssl rand -hex 32)   # generate and save this — it's your dashboard password
echo "Save this token: $CAMPUSCLOUD_API_TOKEN"

./bin/campuscloud serve \
  --addr 127.0.0.1:9090 \
  --cors-origin https://<your-dashboard>.vercel.app
```

`serve` refuses to start without a token (`--token` or
`CAMPUSCLOUD_API_TOKEN`) — there is no unauthenticated mode. It binds to
`127.0.0.1` in this example because production traffic should go through a
reverse proxy that terminates TLS (see step 2); the API itself only speaks
plain HTTP.

### Running it as a service

Keep it running across reboots with systemd:

```ini
# /etc/systemd/system/campuscloud-api.service
[Unit]
Description=CampusCloud API
After=docker.service
Requires=docker.service

[Service]
WorkingDirectory=/opt/campuscloud
EnvironmentFile=/opt/campuscloud/.env
Environment=CAMPUSCLOUD_API_TOKEN=replace-with-your-generated-token
ExecStart=/opt/campuscloud/bin/campuscloud serve --addr 127.0.0.1:9090 --cors-origin https://your-dashboard.vercel.app
Restart=on-failure
User=campuscloud

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now campuscloud-api
```

Put the real token in the unit file's `Environment=` line (or better, in
a root-only-readable file referenced by `EnvironmentFile=`) rather than
committing it anywhere — same rule as every other CampusCloud secret (PRD
section 18).

## 2. Put it behind HTTPS

The dashboard runs in the browser, so the API must be served over HTTPS
(browsers block a mixed-content or CORS request from an HTTPS page to a
plain-HTTP API on a different origin). Use a reverse proxy — Caddy is the
least configuration for a single domain:

```
# /etc/caddy/Caddyfile
campuscloud-api.example.edu {
    reverse_proxy 127.0.0.1:9090
}
```

```bash
sudo systemctl reload caddy
```

Caddy automatically obtains and renews a TLS certificate. With nginx,
you'd instead terminate TLS (e.g. via certbot) and `proxy_pass
http://127.0.0.1:9090;`.

## 3. Deploy the dashboard to Vercel

See [`../web/README.md`](../web/README.md). In short: import the repo into
Vercel with **Root Directory** set to `web/`, deploy, then set
`--cors-origin` on `campuscloud serve` (step 1) to the resulting
`https://<project>.vercel.app` URL and restart the service.

## 4. Log in

Open the deployed dashboard, enter:
- **API URL**: `https://campuscloud-api.example.edu` (your reverse proxy's URL)
- **API token**: the value of `CAMPUSCLOUD_API_TOKEN`

From there you can deploy, start/stop/restart, watch CPU/RAM/storage and
container health, tail logs, and create/restore backups — everything the
CLI does, from a browser.

## Security notes

- The API token is the only thing standing between the internet and your
  Docker host's control plane (deploy/start/stop/restore). Treat it like a
  root password: generate it randomly, store it in a password manager, and
  rotate it (restart `campuscloud serve` with a new
  `CAMPUSCLOUD_API_TOKEN`) if you suspect it leaked.
- `--cors-origin` is an allow-list, not a suggestion — only browsers on
  listed origins can call the API at all; everything else gets no CORS
  headers and a same-origin browser request will be blocked.
- `campuscloud serve` binds to `127.0.0.1` by default in every example
  here; only the reverse proxy (with TLS) should be reachable from the
  internet.
- The API never exposes MariaDB directly — the `/api/v1/health` endpoint
  checks database connectivity via `docker compose exec`, the same way the
  CLI does, so the database stays off the network entirely (PRD section 18).
