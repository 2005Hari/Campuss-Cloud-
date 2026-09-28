#!/usr/bin/env bash
# FR-12 (Project Workspace): create the standardized project folder
# structure — documentation, source code, research, presentations, and
# final submissions — inside a Nextcloud user's files (PRD section 11).
#
# Usage: scripts/create-workspace.sh <nextcloud-username> <project-name>
#
# The user must already exist (create one first with
# `docker compose exec nextcloud php occ user:add <username>`).
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [ $# -ne 2 ]; then
  echo "Usage: $0 <nextcloud-username> <project-name>" >&2
  exit 1
fi

USERNAME="$1"
PROJECT="$2"
COMPOSE_FILE="${COMPOSE_FILE:-docker/docker-compose.yml}"
NEXTCLOUD_SERVICE="${NEXTCLOUD_SERVICE:-nextcloud}"

BASE="/var/www/html/data/${USERNAME}/files/${PROJECT}"
FOLDERS=(
  "01-Documentation"
  "02-Source-Code"
  "03-Research"
  "04-Presentations"
  "05-Final-Submission"
)

echo "==> Creating workspace '${PROJECT}' for ${USERNAME}"
for folder in "${FOLDERS[@]}"; do
  docker compose -f "$COMPOSE_FILE" exec -T --user www-data "$NEXTCLOUD_SERVICE" \
    mkdir -p "${BASE}/${folder}"
done

echo "==> Registering the new folders with Nextcloud"
docker compose -f "$COMPOSE_FILE" exec -T "$NEXTCLOUD_SERVICE" \
  php occ files:scan --path="/${USERNAME}/files/${PROJECT}"

echo
echo "Workspace ready: ${PROJECT} (${#FOLDERS[@]} folders) for user ${USERNAME}."
