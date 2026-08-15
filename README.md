# ShortURL

A lightweight, self-hosted URL shortener with workspaces, custom domains,
asynchronous click analytics, and a machine-to-machine REST API.

Built for the case where an internal system already has a long document URL and
wants a short one before sending it over WhatsApp, email, or SMS — one API call,
one `short_url` back, safe to retry.

## Features

- **Workspaces** with owner / admin / member roles, enforced server-side
- **Custom domains** — add one, verify it with a DNS record, and it starts
  serving. No Compose edit, no container restart.
- **Automatic HTTPS** for the application domain and every verified custom
  domain, via Traefik and Let's Encrypt
- **Fast redirects** — Redis first, PostgreSQL only on a miss, with explicit
  invalidation on every change
- **Link controls** — custom or generated slugs, 301/302/307/308, expiry, click
  limits, password protection
- **Asynchronous analytics** — clicks queue in Redis Streams and never delay a
  redirect; a worker drains them into pre-aggregated rollup tables
- **Public REST API** with per-workspace API keys, scopes, and `Idempotency-Key`
  support
- **One-command install** with generated secrets and a first-run setup wizard

## Stack

| Layer          | Choice                                    |
| -------------- | ----------------------------------------- |
| Backend        | Go + [chi](https://github.com/go-chi/chi) |
| Queries        | [sqlc](https://sqlc.dev) — explicit SQL, no ORM |
| Frontend       | Nuxt 4 (SPA) + Vue 3 + Tailwind CSS 4     |
| Package runner | [Bun](https://bun.sh)                     |
| Database       | PostgreSQL 17                             |
| Cache / queue  | Redis 7                                   |
| Edge / TLS     | Traefik v3 + Let's Encrypt                |

Production images are small and unprivileged: **23 MB** for the Go binary
(distroless, static, non-root) and **54 MB** for the dashboard (static files on
nginx, non-root). The whole stack idles in a few hundred MB of RAM.

## Deploy

```bash
git clone <repository> shorturl
cd shorturl

cp .env.example .env
./scripts/install.sh      # generates secrets, checks prerequisites
docker compose up -d
```

`install.sh` tells you which values still need a real one — your domain and a
Let's Encrypt contact address. Then open the dashboard and the setup wizard
creates your administrator account and first workspace.

Read [docs/deployment.md](docs/deployment.md) for DNS setup, custom domains,
backups, upgrades, and **moving the whole deployment to another server**.

For a Dokploy deployment backed by images published from GitHub Actions, use
[`docker-compose.dokploy.yml`](docker-compose.dokploy.yml) and follow
[`docs/dokploy.md`](docs/dokploy.md).

## Use the API

```bash
curl -X POST https://short.example.com/api/v1/links \
  -H "Authorization: Bearer shr_live_..." \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: document:12345" \
  -d '{"destination_url":"https://internal.company.com/documents/very-long?token=abc",
       "external_reference":"document:12345"}'
```

```json
{ "data": { "short_url": "https://go.company.com/aB7kP2", "...": "..." } }
```

Full contract in [docs/openapi.yaml](docs/openapi.yaml); worked examples for
Laravel, Python, and Node.js in
[docs/api-integration.md](docs/api-integration.md).

## Develop

```bash
cp .env.example .env
make deps
make migrate
make dev
```

`make dev` starts Postgres and Redis in Docker, then runs the Go server and the
Nuxt dashboard on your host:

- Dashboard — <http://localhost:3000>
- API health — <http://localhost:8080/health>

Ctrl-C stops the host processes; the containers keep running (`make down`).

Run the analytics worker separately when you need it:

```bash
cd apps/server && go run ./cmd/server worker
```

## Commands

```
make help          List every target

Development
make dev           Start infrastructure, the API, and the dashboard
make up / down     Start or stop Postgres and Redis
make logs          Follow container logs
make deps          Install dashboard dependencies and download Go modules
make migrate       Apply pending migrations to the dev database
make migrate-down  Roll back the most recent migration
make sqlc          Regenerate the query layer from db/queries
make test          Run the Go test suite
make lint          gofmt check, go vet, and dashboard typecheck
make build         Build the server binary and the dashboard

Production
make install       Generate secrets and check prerequisites
make prod-build    Build the production images
make prod-up       Start the production stack
make prod-logs     Follow production logs
make backup        Archive .env, the database, and the TLS certificates
make restore       Restore a backup: make restore ARCHIVE=backups/...tar.gz
```

## Layout

```
apps/
  server/                  Go backend — one binary, four commands
    cmd/server/            serve · worker · migrate · healthcheck
    db/migrations/         SQL schema, embedded in the binary
    db/queries/            sqlc source queries
    internal/
      analytics/           Redis Streams producer, worker, reporting
      apikey/              Key issuance, hashing, authentication
      auth/                Sessions, cookies, password hashing
      authctx/             Current principal and the role permission matrix
      cache/               Redis client and key layout
      config/              Environment configuration
      database/            pgx pool and migrations
      domain/              Custom domains, DNS verification, Traefik sync
      httpx/               Response envelope and typed errors
      link/                Link CRUD, slug generation, redirect resolution
      middleware/          Real-IP, rate limiting, logging, security headers
      publicapi/           Machine-to-machine REST API, idempotency
      redirect/            The hot path
      security/            Argon2id, tokens, IP anonymisation
      server/              Router, health probes, dependency wiring
      setup/               First-run wizard
      slug/ urlx/ validate/  Pure helpers
      store/               sqlc-generated queries
  web/                     Nuxt 4 dashboard (SPA)
docker/                    Dockerfiles, nginx config, Traefik dynamic dir
scripts/                   dev · install · backup · restore
docs/                      deployment · openapi · api-integration
```

## Design notes

- **Analytics reads never scan the raw click log.** The worker maintains
  `click_hourly` and `click_dimension_daily` rollups, so a 90-day chart reads
  about two thousand rows instead of every click ever recorded. This is what
  makes PostgreSQL fast enough here and why ClickHouse is not needed — the
  reasoning, the measured triggers that would change the answer, and the
  migration path are in §17.1–17.2 of the implementation plan.
- **Custom domains route through a watched directory.** On verification the
  server writes a Traefik router file to a shared volume; Traefik hot-reloads.
  Certificates are requested only for verified domains, because a catch-all
  router would ask Let's Encrypt for a certificate for every hostname anyone
  points at the server and burn the rate limit.
- **The redirect path is deliberately thin.** On a cache hit: one Redis read,
  one non-blocking channel send for analytics, one redirect header. Misses are
  cached too — including "not found", so slug-scanning traffic cannot turn into
  a database query per request.
- **Click limits use an atomic Redis counter**, not the cached click count,
  which would be stale the moment a second request arrived.
- **Secrets go only where they are used.** `serve` requires `SESSION_SECRET`
  and `IP_HASH_SECRET`; `migrate` and `worker` neither require nor receive them.
- **Real-IP is trusted only from configured proxies.** chi's stock `RealIP`
  believes `X-Forwarded-For` from anyone, which would let callers choose the
  address that rate limiting keys on.
- **Rate limits are scoped to what they protect.** The strict per-IP limit
  applies to credential submissions only — putting it on the whole `/auth`
  group throttled the session check the dashboard makes on every page load.
- **Single origin.** Traefik serves the dashboard and API from one hostname,
  routing `/api` to Go at higher priority than the SPA. Nuxt's dev proxy mirrors
  that, so the API needs no CORS in either environment.

## Testing

```bash
make test     # 427 Go test cases
make lint     # gofmt, go vet, dashboard typecheck
```

Beyond unit tests, the build was verified end-to-end against a running stack:
the full acceptance scenario from §46 of the plan (92 assertions), the
production Compose stack including custom-domain routing (11 assertions), and
the dashboard driven in a headless browser (13 assertions). See
[docs/build-report.md](docs/build-report.md).
