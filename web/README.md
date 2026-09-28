# CampusCloud Dashboard

A browser dashboard for the `campuscloud` CLI's HTTP API (`campuscloud
serve`) — deploy, monitor, back up, and restore your CampusCloud stack
without SSHing into the VM. Deployable to Vercel.

This app is a plain client-rendered Next.js SPA: it has no server-side
component of its own and stores no secrets at build time. At login, you
give it the URL of your `campuscloud serve` API and its bearer token; both
are kept only in your browser's local storage and sent straight to that
API — never through Vercel or any other intermediary.

## Prerequisites

The actual backend — `campuscloud serve` — must be running somewhere with
Docker (your VM), reachable over HTTPS, with CORS configured to allow this
dashboard's origin. See [`../docs/dashboard.md`](../docs/dashboard.md) for
the full setup (systemd unit, reverse proxy, CORS).

## Local development

```bash
npm install
cp .env.local.example .env.local   # optional: pre-fill the API URL
npm run dev
```

Open http://localhost:3000, then log in with your API's URL and token.

## Deploying to Vercel

1. Push this repository to GitHub (or your Git provider of choice).
2. In Vercel, **Add New Project** → import the repo → set **Root
   Directory** to `web/` (this is a monorepo; the Next.js app lives in
   this subdirectory, not the repo root).
3. (Optional) Set the environment variable `NEXT_PUBLIC_API_URL` to your
   `campuscloud serve` URL — this only pre-fills the login form's API URL
   field, it is not a secret.
4. Deploy. Vercel auto-detects the Next.js framework and needs no other
   configuration.
5. On your VM, add this deployment's URL (e.g.
   `https://campuscloud-dashboard.vercel.app`) to `campuscloud serve`'s
   `--cors-origin` flag and restart it.
6. Open the deployed URL, log in with your API token.

## Project structure

```
web/
├── app/               Next.js App Router: layout + the single dashboard page
├── components/        Dashboard sections (Overview, Health, Backups, Logs, Actions)
├── hooks/useJobPoller.ts   Polls a background job (deploy/backup/restore/...) to completion
└── lib/
    ├── api.ts         Typed fetch client for the campuscloud HTTP API
    ├── auth.tsx        Stores {apiUrl, token} in localStorage via React context
    └── types.ts        TypeScript mirrors of the Go API's JSON response shapes
```
