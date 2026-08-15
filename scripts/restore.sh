#!/usr/bin/env bash
#
# Restore a backup produced by scripts/backup.sh onto this server.
#
# Usage:
#   ./scripts/restore.sh shorturl-20260815T051500Z.tar.gz
#
# This overwrites .env and replaces the contents of the database, so it asks
# for confirmation first.
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ARCHIVE="${1:-}"
if [[ -z "$ARCHIVE" ]]; then
  echo "usage: ./scripts/restore.sh <backup.tar.gz>" >&2
  exit 1
fi
if [[ ! -f "$ARCHIVE" ]]; then
  echo "error: $ARCHIVE not found" >&2
  exit 1
fi

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

bold() { printf '\033[1m%s\033[0m\n' "$1"; }
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; }

tar -xzf "$ARCHIVE" -C "$STAGE"

if [[ ! -f "$STAGE/env" || ! -f "$STAGE/postgres.sql" ]]; then
  echo "error: $ARCHIVE does not look like a ShortURL backup" >&2
  exit 1
fi

bold "Restoring from $ARCHIVE"
echo
echo "  This will overwrite:"
echo "    - .env in $ROOT_DIR"
echo "    - every table in the shorturl database"
echo
read -r -p "  Continue? [y/N] " reply
if [[ ! "$reply" =~ ^[Yy]$ ]]; then
  echo "  Aborted."
  exit 0
fi
echo

# --- Configuration -----------------------------------------------------------
if [[ -f .env ]]; then
  cp .env ".env.before-restore-$(date -u +%Y%m%dT%H%M%SZ)"
  ok "existing .env saved as .env.before-restore-*"
fi
cp "$STAGE/env" .env
chmod 600 .env
ok ".env"

# --- Database ----------------------------------------------------------------
# Postgres must be up, but nothing may be writing while the dump replays.
bold "Starting Postgres"
docker compose up -d --wait postgres
ok "postgres healthy"

POSTGRES_USER="$(grep -E '^POSTGRES_USER=' .env | tail -n1 | cut -d= -f2-)"
POSTGRES_DB="$(grep -E '^POSTGRES_DB=' .env | tail -n1 | cut -d= -f2-)"
POSTGRES_USER="${POSTGRES_USER:-shorturl}"
POSTGRES_DB="${POSTGRES_DB:-shorturl}"

# The dump was written with --clean --if-exists, so it drops and recreates as
# it goes. ON_ERROR_STOP turns a partial restore into a loud failure.
docker compose exec -T postgres \
  psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  < "$STAGE/postgres.sql" > /dev/null
ok "database restored"

# --- Certificates ------------------------------------------------------------
if [[ -f "$STAGE/acme.json" ]]; then
  docker compose up -d --wait traefik >/dev/null
  # acme.json holds private keys; Traefik refuses to use it unless it is 0600.
  chmod 600 "$STAGE/acme.json"
  docker compose cp "$STAGE/acme.json" traefik:/letsencrypt/acme.json
  docker compose restart traefik >/dev/null
  ok "certificates restored"
else
  printf '  \033[33m!\033[0m no certificates in the archive - new ones will be issued\n'
fi

echo
bold "Bringing the stack up"
docker compose up -d
echo
ok "done"
echo
echo "  Check it over:"
echo "    docker compose ps"
echo "    docker compose logs -f traefik"
echo
echo "  If DNS still points at the old server, update it now. Certificate"
echo "  renewal needs this host to answer the HTTP challenge."
echo
