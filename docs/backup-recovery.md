# Backup & Recovery

Implements FR-08 (Backup) and FR-09 (Restore), per PRD section 14.

## What gets backed up

Each backup set is a timestamped directory under the configured
`backup.dir` (default `./backups`, override with `CAMPUSCLOUD_BACKUP_DIR`):

```
backups/
└── 20260928-153000/
    ├── database.sql       mysqldump of the Nextcloud database
    ├── config.php         Nextcloud's configuration file
    ├── user-files.tar.gz  everything under Nextcloud's data directory
    └── manifest.json      SHA-256 checksum + size for every file above
```

The database credential is never passed as a command-line argument (which
would otherwise be visible via `docker top`/`ps` inside the container): the
dump and restore commands set `MYSQL_PWD` from the container's own
environment instead.

## Creating a backup

```bash
./bin/campuscloud backup
```

Output ends with the path to the new backup directory. Old backups beyond
`backup.retain` (default 5, in `config/config.yaml`) are pruned
automatically, oldest first.

```bash
./bin/campuscloud backup list      # most recent first
```

## Restoring a backup

```bash
./bin/campuscloud restore 20260928-153000
# or:
./bin/campuscloud restore --latest
```

`restore` (FR-09) runs, in order:

1. **Validate** — re-read `manifest.json` and re-hash every file; abort if
   anything is missing or its checksum doesn't match (a corrupted or
   truncated backup is never applied).
2. **Stop** the application containers.
3. **Restore the database** — pipe `database.sql` into MariaDB.
4. **Restore files and configuration** — copy `config.php` back into the
   Nextcloud container and extract `user-files.tar.gz` into its data
   directory.
5. **Start** the application again.
6. **Health check** — wait for both containers to report healthy before
   returning success.

If any step fails, `restore` exits non-zero with the specific error rather
than silently leaving the stack half-restored.

## Testing the procedure

The Testing Strategy (PRD section 19) calls for backup/restore testing
using a sample project folder and database, and for failure/recovery
testing:

```bash
# Failure + recovery
docker compose -f docker/docker-compose.yml stop nextcloud
./bin/campuscloud health          # shows nextcloud-running: FAIL
./bin/campuscloud restart
./bin/campuscloud health          # back to pass

# Backup + restore
./bin/campuscloud backup
# ... make some changes in Nextcloud ...
./bin/campuscloud restore --latest
```
