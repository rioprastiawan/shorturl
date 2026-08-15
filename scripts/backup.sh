#!/usr/bin/env bash
#
# Capture everything needed to rebuild this deployment somewhere else:
#
#   .env          configuration and secrets
#   postgres      full logical dump (links, users, workspaces, analytics)
#   acme.json     issued TLS certificates
#
# Redis is deliberately not backed up. It holds only the link cache, which
# rebuilds itself from Postgres, and the click-event stream, which the worker
# drains continuously. Restoring a stale Redis would be worse than an empty one.
#
# Usage:
#   ./scripts/backup.sh [output-directory]     # default: ./backups
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

OUT_DIR="${1:-backups}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARCHIVE="$OUT_DIR/shorturl-$STAMP.tar.gz"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

bold() { printf '\033[1m%s\033[0m\n' "$1"; }
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; }

if [[ ! -f .env ]]; then
  echo "error: .env not found - run this from a configured deployment" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

bold "Backing up ShortURL"

# --- Configuration -----------------------------------------------------------
cp .env "$STAGE/env"
ok ".env"

# --- Database ----------------------------------------------------------------
# --clean --if-exists makes the dump replayable over an existing database.
# Running inside the container avoids needing a matching pg_dump on the host.
POSTGRES_USER="$(grep -E '^POSTGRES_USER=' .env | tail -n1 | cut -d= -f2-)"
POSTGRES_DB="$(grep -E '^POSTGRES_DB=' .env | tail -n1 | cut -d= -f2-)"
POSTGRES_USER="${POSTGRES_USER:-shorturl}"
POSTGRES_DB="${POSTGRES_DB:-shorturl}"

if ! docker compose ps --status running --services 2>/dev/null | grep -qx postgres; then
  echo "error: the postgres service is not running - start it before backing up" >&2
  exit 1
fi

docker compose exec -T postgres \
  pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists \
  > "$STAGE/postgres.sql"
ok "postgres dump ($(wc -c < "$STAGE/postgres.sql" | tr -d ' ') bytes)"

# --- Certificates ------------------------------------------------------------
# Carrying acme.json across avoids re-issuing on the new server, which matters
# because Let's Encrypt rate-limits duplicate certificates to 5 per week.
if docker compose cp traefik:/letsencrypt/acme.json "$STAGE/acme.json" 2>/dev/null; then
  ok "traefik certificates"
else
  printf '  \033[33m!\033[0m no acme.json yet - certificates will be issued fresh\n'
fi

# --- Archive -----------------------------------------------------------------
tar -czf "$ARCHIVE" -C "$STAGE" .
chmod 600 "$ARCHIVE"

echo
bold "Wrote $ARCHIVE"
echo "  $(du -h "$ARCHIVE" | cut -f1)"
echo
echo "  This archive contains your secrets and TLS private keys."
echo "  Store it encrypted and transfer it over a channel you trust."
echo
echo "  Restore on the new server with:"
echo "    ./scripts/restore.sh $ARCHIVE"
echo
