# User & Group Structure (FR-11)

CampusCloud uses Nextcloud's built-in groups to model the target users
from PRD section 5:

| Group | Maps to |
|---|---|
| `Students` | Student |
| `Faculty` | Faculty |
| `Club Admins` | Club / Project Administrator |
| `System Admins` | System Administrator |

## Creating the groups

```bash
./scripts/setup-groups.sh
```

This runs `occ group:add` for each of the four groups inside the running
Nextcloud container. It is safe to re-run.

## Creating users

User creation isn't scripted (it needs an interactive password prompt, or
a per-user password you choose), so create users directly with `occ`:

```bash
docker compose -f docker/docker-compose.yml exec nextcloud \
  php occ user:add --group="Students" jsmith
```

Repeat with `--group="Faculty"`, `--group="Club Admins"`, or
`--group="System Admins"` as appropriate. A user can belong to more than
one group — add them again with a different `--group` value.

## Access control

Nextcloud's group-based sharing controls who can see a given folder or
project workspace (see `docs/project-workspace.md`): share a folder with a
group (e.g. `Club Admins`) rather than individual users so membership
changes automatically propagate, matching PRD section 18's requirement to
"Use Nextcloud authentication and group-based access control."
