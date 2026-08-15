# ShortURL — Build Report

**Date:** 15 August 2026
**Scope:** Milestones 2–12 of `shorturl-selfhost-plan.md` (Milestone 1 and the
production Docker stack were delivered earlier in the session)
**Status:** MVP complete. Every item in §30 of the plan is implemented and
verified against a running stack.

---

## 1. What was built

| Milestone | Delivered |
| --- | --- |
| 2 — Database | 13-table schema, pgx pool, sqlc query layer, embedded migrations |
| 3 — Authentication | Argon2id passwords, server-side sessions, HttpOnly cookies, setup wizard |
| 4 — Workspaces | CRUD, membership, role matrix, authorization middleware |
| 5 — Domains | DNS TXT + routing verification, Traefik dynamic router generation |
| 6 — Links | CRUD, slug generation, URL validation, cursor pagination, search |
| 7 — Redirects | Host + slug resolution, all four redirect codes, expiry, click limits, password gate |
| 8 — Cache | Redis-first resolution, negative caching, explicit invalidation |
| 9 — Analytics | Redis Streams producer, worker, rollup tables, reporting API |
| 10 — Public API | API keys with scopes, idempotency, external references, rate limiting, OpenAPI |
| 11 — Production | migrate + worker services, shared Traefik volume, non-root images |
| 12 — Docs | Deployment guide, API integration guide, OpenAPI spec, this report |

**Code:** 13,329 lines across 94 hand-written Go files (excluding the
sqlc-generated query layer) and 5,615 lines across 49 Vue/TypeScript files.
**Tests:** 427 Go test cases across 15 packages.

The dashboard implements all 13 pages from §15: setup wizard, login, register,
overview, links (list / create / detail), analytics, domains, members, API keys,
settings.

---

## 2. Verification

Nothing below is inferred from the code — each number is from a run against a
live stack.

### Unit and package tests
```
gofmt -l .        → clean
go vet ./...      → clean
go test ./...     → 15 packages, 427 cases, all passing
bun run typecheck → exit 0
```

### End-to-end API suite — 92 assertions, 0 failures
Walks the §46 acceptance scenario against a running server and worker:
setup wizard and its re-run refusal, login enumeration resistance, workspace
isolation, domain verification failure messaging, link CRUD, all redirect
outcomes, cache invalidation, click limits, API key lifecycle, idempotent
replay, scope enforcement, revocation, and the analytics pipeline landing in
PostgreSQL.

### Production stack — 11 assertions, 0 failures
From `docker compose down -v` to a working deployment: migrations run once via
the one-shot service, all seven services report healthy, TLS terminates at
Traefik, the API and dashboard route correctly, **a verified custom domain
starts serving without any Compose edit or container restart**, the worker
persists clicks, and PostgreSQL and Redis publish no host ports.

### Dashboard in a headless browser — 13 assertions, 0 failures
Chromium driving the real SPA against the production stack: the wizard creates
an administrator and signs them in, every dashboard page renders, a domain is
added through the UI and its DNS records displayed, and the console is free of
uncaught errors.

---

## 3. Bugs found by testing

Four defects were found only because the stack was actually exercised. All four
were invisible to `go build`, `go test`, and `bun run typecheck`.

### 3.1 No layout ever rendered (critical)

`app/app.vue` contained `<NuxtPage />` with no `<NuxtLayout>` wrapper. Nuxt
silently ignores `definePageMeta({ layout })` in that case, so the dashboard
rendered with no sidebar, no workspace switcher, and — because the layout is
what loads the workspace list — every page failed with "Could not load the
overview". The app compiled, typechecked, and built perfectly.

Found by: headless browser. Fixed in `app/app.vue`.

### 3.2 Custom domains 404'd (critical)

The generated Traefik router referenced `service: shorturl-server`. Traefik
resolves an unqualified service name *within the same provider*, so a router
written by the file provider looked for `shorturl-server@file`, found nothing,
and served 404 for every custom domain — logging only a service-not-found error
that nothing was watching.

Found by: production stack test. Fixed in `internal/domain/traefik.go`; the
constant now carries the required `@docker` suffix, with a test pinning it.

### 3.3 Every dashboard URL broke behind the proxy (critical)

`nuxt generate` prerenders each route into a directory (`setup/`, `login/`,
`dashboard/`). nginx therefore issues a 301 to append the trailing slash, and by
default builds that `Location` as an absolute URL from what it can see:
`http://host:8080/setup/` — the container's own internal plaintext port, which
no external client can reach. `curl` without `-L` reported a cheerful 301; a
browser hit connection-refused.

Found by: headless browser. Fixed in `docker/web/nginx.conf` with
`absolute_redirect off`.

### 3.4 Users were signed out after ~5 page loads (high)

The strict 10-per-minute credential rate limit was applied to the whole `/auth`
route group and to `/setup`. But `/auth/me` and `/setup/status` are called on
*every* page load by the session middleware, so ordinary navigation consumed the
budget meant to stop credential stuffing, and the dashboard started 429-ing
itself.

Found by: headless browser. Fixed in `internal/server/routes.go` — the limiter
now applies only to `POST` on the three endpoints that actually accept a
password, with a regression test.

### Also fixed
- **Rate limiter burst was `perMinute/10`**, so a limit of 10/min allowed one
  request then one every six seconds. A user mistyping a password could not
  retry. Burst now equals the full per-minute allowance, which is what "10 per
  minute" is normally taken to mean.
- **`NewRateLimiter(0)` panicked** with a divide-by-zero. Found by a unit test
  written for the burst fix.
- **`migrate` demanded `SESSION_SECRET`**, which it has no use for. Secret
  validation moved from `config.Load` to a `RequireSecrets()` call made only by
  `serve`, so migration and worker containers never receive signing keys.

---

## 4. The ClickHouse question

**Recommendation: not now — and the reason is not "later, maybe".**

ClickHouse is genuinely 10–100× faster than PostgreSQL for large analytical
scans. That advantage only pays when there is a large scan to do. The analytics
layer here is built so there is not.

**What was built instead.** The worker maintains two pre-aggregated tables as it
drains the Redis stream, aggregating each batch in memory first so 256 click
events collapse into a handful of upserts:

```
click_events           raw log — detail views and export only, pruned after 400 days
click_hourly           (workspace_id, link_id, bucket, clicks)
click_dimension_daily  (workspace_id, day, dimension, value, clicks)
```

Every reporting query reads a rollup. None touch the raw log.

```
90-day chart for one link
  scanning click_events    → every click ever recorded for that link
  reading click_hourly     → ~2,160 rows
```

At that size PostgreSQL's row store wins on latency, because there is no
distributed planner or part-merge overhead to amortise.

**What ClickHouse would cost today:** 1–2 GB of RAM against a stack that
currently idles in a few hundred; a second database to back up, migrate, and
restore; a second schema to keep in sync; and a second consistency model with no
transactions or foreign keys. That directly contradicts design goal #1 and makes
"move the deployment to another server" materially harder.

**When to revisit** — measured, not guessed:

```
click_events exceeds ~100 million rows
sustained ingest above ~2,000 clicks/second
p95 of an analytics endpoint exceeds 500 ms after EXPLAIN ANALYZE
    confirms the rollup indexes are being used
raw-event retention needs to exceed ~2 years
per-click drill-down over full history becomes a product requirement
```

**Cheaper steps to take first**, in order: partition `click_events` by month so
retention becomes `DROP PARTITION`; swap the `clicked_at` B-tree for a BRIN
index; add a `click_daily` rollup for ranges beyond 90 days; export cold raw
events to Parquet.

**How the code is prepared.** The reporting layer sits behind an interface, so
migrating means writing a second reader and a second worker sink — no handler,
route, or frontend change. The recommended path is dual-write with a week of
result comparison, not a cutover.

This is written up in full as **§17.1 and §17.2** of the plan, including the
migration sequence.

---

## 5. Deviations from the plan

All are documented in place; §48.14 asks for exactly this.

| Plan said | Built | Why |
| --- | --- | --- |
| §4 `click_events.id UUID PK` | `BIGINT GENERATED ALWAYS AS IDENTITY` | Highest-write table, never addressed publicly; random UUID keys fragment the B-tree badly at volume |
| §17 "PostgreSQL is sufficient" | PostgreSQL **plus rollup tables** | Sufficient is not the same as fast; rollups are what make it fast |
| §20 `DATABASE_URL` written out | Composed in Compose from its parts | Rotating the password cannot leave a stale copy behind |
| §43 Nuxt image | Static files on nginx, no JS runtime | The dashboard is an SPA; this removes a Node process from the stack |
| §14.9 400 for invalid request | 400 for parse failures, **422** for field validation | Lets clients distinguish "malformed" from "rejected" |
| §11 worker writes raw events | Raw events **and** rollups, in one transaction | Rollups must not drift from the log |

Additions not in the plan: `sessions` and `app_settings` tables (server-side
sessions and durable setup state, both required by §5 and §23); an
`internal/security` package for shared primitives; negative caching on the
redirect path; a Redis counter for click-limit enforcement.

---

## 6. Known limitations

Honest list of what is not done or not proven.

1. **DNS verification was not tested against real public DNS.** The resolver
   logic is unit-tested with a fake resolver covering TXT, A/AAAA, and CNAME
   paths, and the failure-message content is asserted. But no test used a real
   domain, because that needs public DNS this environment does not have. The
   production test simulates a successful verification by setting the row to
   `active`, then exercises everything downstream for real.

2. **Let's Encrypt issuance was not exercised.** ACME is wired and reaches the
   CA — the logs show a real rejection of the placeholder `example.com` contact
   address — but no certificate was issued, since that requires a public domain
   pointing at the host. **Test with `ACME_CA_SERVER` set to the staging URL
   before going live**; the production CA allows only 5 duplicate certificates
   per week.

3. **`ssl_status` never leaves `pending`.** Nothing observes ACME completion and
   flips it. The dashboard displays DNS and SSL state separately as §39
   requires, but the SSL half is not yet driven by reality. Wiring it means
   polling Traefik's API or watching `acme.json`.

4. **Rate limiting is per-container.** The limiter is in-memory, which is right
   for one server container and no Redis round-trip on the hot path. Scaling to
   several replicas would give each its own share of the limit.

5. **The API omits `X-RateLimit-Remaining` and `-Reset`.** A client cannot pace
   itself; it can only discover the wall by hitting it.

6. **Per-link analytics have no dimensional breakdowns.** The rollups are keyed
   by workspace, so per-link referrer/device charts would require scanning the
   raw log — the one query the design rules out. Adding them means a `link_id`
   column on the dimension rollup.

7. **`/docs/api` is not mounted.** The OpenAPI spec exists at
   `docs/openapi.yaml`; §14.11's interactive viewer is not served. The path is
   reserved in the slug list, so it stays free.

8. **A poison event fails its whole batch.** One event referencing a deleted
   link fails the `CopyFrom` for up to 255 good events alongside it. Redelivery
   retires it after 5 attempts. Acceptable for MVP; the fix is a per-row
   fallback on constraint violations.

9. **No ownership transfer.** `owner` is set at workspace creation and cannot be
   moved, so the members API accepts only `admin` and `member`.

---

## 7. Recommended next steps

**Before you go live**
1. Point a real domain at the server and set `ACME_CA_SERVER` to the Let's
   Encrypt **staging** URL. Confirm a certificate is issued, then switch to
   production and restart Traefik.
2. Run a real domain through the verification flow end to end — that is the one
   path this build could not exercise.
3. Set up the nightly backup cron from `docs/deployment.md` and **practise a
   restore onto a second machine** before you need it.

**Soon after**
4. Drive `ssl_status` from actual ACME state (limitation 3).
5. Add `X-RateLimit-Remaining` and `-Reset` (limitation 5).
6. Serve the OpenAPI spec at `/docs/api`.

**When traffic justifies it** — partition `click_events`, then re-read §17.2
before reaching for ClickHouse.

---

## 8. How to run the verification yourself

```bash
# Unit tests and static checks
make test && make lint

# Full stack from nothing
cp .env.example .env && ./scripts/install.sh
docker compose up -d --wait
docker compose ps          # all services healthy
docker compose logs migrate # migrations applied once
```

The end-to-end scripts used for this report exercise the API, the production
stack, and the dashboard. They assume a freshly started server, because the
in-memory rate limiter carries state between runs.
