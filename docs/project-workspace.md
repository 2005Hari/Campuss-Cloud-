# Project Workspace (FR-12)

CampusCloud provisions a standardized folder structure for each student or
club project, so every team's files land in the same layout regardless of
who set the project up:

```
<project-name>/
├── 01-Documentation
├── 02-Source-Code
├── 03-Research
├── 04-Presentations
└── 05-Final-Submission
```

## Creating a workspace

```bash
./scripts/create-workspace.sh <nextcloud-username> "<project-name>"
```

For example:

```bash
./scripts/create-workspace.sh jsmith "Capstone-Project"
```

The user must already exist (see `docs/user-groups.md`). The script
creates the five folders directly under that user's Nextcloud files, then
runs `occ files:scan` so Nextcloud immediately picks them up — no manual
rescan or page refresh delay.

## Sharing a workspace with a team

Once created, share `<project-name>` (or any subfolder) with the
appropriate group from the Nextcloud web UI, or via `occ`:

```bash
docker compose -f docker/docker-compose.yml exec nextcloud \
  php occ sharing:... # see `occ sharing --help` for the exact subcommand
                       # available in your Nextcloud version
```

Sharing with a group (e.g. `Club Admins`) rather than individual members
means the share stays correct as team membership changes.
