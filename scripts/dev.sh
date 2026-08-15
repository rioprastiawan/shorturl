#!/usr/bin/env bash
#
# Start the full development environment: Postgres and Redis in Docker, the Go
# server and the Nuxt dashboard on the host. Ctrl-C stops the host processes;
# the containers keep running (stop them with `make down`).
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="docker-compose.dev.yml"

if [[ ! -f .env ]]; then
  echo "==> .env not found, creating it from .env.example"
  cp .env.example .env
fi

# Values published by the dev stack on loopback. The Go server runs on the host
# here, so it cannot use the in-network "postgres"/"redis" hostnames from .env.
set -a
# shellcheck disable=SC1091
source .env
set +a
DEV_PG_USER="${POSTGRES_USER:-shorturl}"
DEV_PG_PASS="${POSTGRES_PASSWORD:-shorturl}"
DEV_PG_DB="${POSTGRES_DB:-shorturl}"

export APP_ENV="${APP_ENV_DEV:-development}"
export LOG_LEVEL="${LOG_LEVEL_DEV:-debug}"
export DATABASE_URL="postgres://${DEV_PG_USER}:${DEV_PG_PASS}@localhost:${POSTGRES_PORT:-5432}/${DEV_PG_DB}?sslmode=disable"
export REDIS_ADDR="localhost:${REDIS_PORT:-6379}"

echo "==> Starting Postgres and Redis"
docker compose -f "$COMPOSE_FILE" up -d --wait

# Migrate before anything connects. A fresh clone otherwise gets a server with
# no tables, which surfaces as confusing 500s rather than an obvious error.
echo "==> Applying migrations"
(cd apps/server && go run ./cmd/server migrate)

pids=()
cleanup() {
  echo
  echo "==> Stopping host processes (containers left running)"
  for pid in "${pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "==> Starting Go server on :${SERVER_PORT:-8080}"
(cd apps/server && go run ./cmd/server serve) &
pids+=($!)

# Without the worker, clicks pile up in the Redis stream and every analytics
# page reads zero — which looks like a bug rather than a missing process.
echo "==> Starting analytics worker"
(cd apps/server && go run ./cmd/server worker) &
pids+=($!)

echo "==> Starting Nuxt dashboard on :3000"
(cd apps/web && bun run dev) &
pids+=($!)

echo
echo "    Dashboard  http://localhost:3000"
echo "    API        http://localhost:${SERVER_PORT:-8080}/health"
echo
echo "    Ctrl-C stops these processes. Postgres and Redis keep running;"
echo "    stop them with 'make down'."
echo

wait
