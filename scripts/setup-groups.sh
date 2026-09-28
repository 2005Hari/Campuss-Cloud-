#!/usr/bin/env bash
# FR-11 (User & Group Structure): create the standard CampusCloud Nextcloud
# groups — Students, Faculty, Club Admins, System Admins (PRD section 11).
#
# Usage: scripts/setup-groups.sh
#
# Run this once after `campuscloud deploy` has finished and Nextcloud is
# reachable. It is safe to re-run: `occ group:add` is a no-op if the group
# already exists.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

COMPOSE_FILE="${COMPOSE_FILE:-docker/docker-compose.yml}"
NEXTCLOUD_SERVICE="${NEXTCLOUD_SERVICE:-nextcloud}"

groups=(
  "Students"
  "Faculty"
  "Club Admins"
  "System Admins"
)

for group in "${groups[@]}"; do
  echo "==> Ensuring group exists: ${group}"
  # `occ group:add` exits non-zero if the group already exists; that's
  # fine, this script is meant to be safe to re-run.
  docker compose -f "$COMPOSE_FILE" exec -T "$NEXTCLOUD_SERVICE" php occ group:add "$group" || true
done

echo
echo "Groups ready. Add a user to a group with:"
echo "  docker compose -f $COMPOSE_FILE exec -T $NEXTCLOUD_SERVICE php occ user:add --group=\"Students\" <username>"
