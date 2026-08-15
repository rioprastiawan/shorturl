#!/usr/bin/env bash
#
# Force-activate a domain for local testing, bypassing DNS verification.
#
# Domain verification checks real public DNS (a TXT record and A/CNAME
# routing), which nothing on localhost can satisfy. In production this step
# is what proves you control the domain; locally there is no domain to prove,
# so this script writes the "verified" result directly instead of faking DNS
# records that would only work behind additional setup.
#
# Usage:
#   ./scripts/dev-activate-domain.sh go.local.test
#
# The activated domain works immediately with curl using a Host header:
#   curl -H 'Host: go.local.test' http://localhost:8080/some-slug
#
set -euo pipefail

HOSTNAME="${1:-}"
if [[ -z "$HOSTNAME" ]]; then
  echo "usage: $0 <hostname>" >&2
  echo "example: $0 go.local.test" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

CONTAINER="shorturl-dev-postgres-1"
if ! docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
  echo "error: $CONTAINER is not running — start it with 'make up' or 'make dev'" >&2
  exit 1
fi

# Load .env for the Postgres credentials the dev container was created with.
if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi
PG_USER="${POSTGRES_USER:-shorturl}"
PG_DB="${POSTGRES_DB:-shorturl}"

EXISTS=$(docker exec -i "$CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -tAc \
  "SELECT count(*) FROM domains WHERE LOWER(hostname) = LOWER('$HOSTNAME');")

if [[ "$EXISTS" == "0" ]]; then
  echo "error: no domain '$HOSTNAME' found." >&2
  echo "Add it first, from the dashboard or:" >&2
  echo "  curl -b /tmp/shorturl-dev.cookies -X POST http://localhost:8080/api/v1/workspaces/<id>/domains \\" >&2
  echo "       -H 'Content-Type: application/json' -d '{\"hostname\":\"$HOSTNAME\"}'" >&2
  exit 1
fi

docker exec -i "$CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -q -c \
  "UPDATE domains SET status = 'active', ssl_status = 'active', verified_at = now(), verification_error = NULL
   WHERE LOWER(hostname) = LOWER('$HOSTNAME');"

echo "✓ $HOSTNAME is now active."
echo
echo "Create a link on it from the dashboard, then follow it with:"
echo "  curl -I -H 'Host: $HOSTNAME' http://localhost:8080/<slug>"
