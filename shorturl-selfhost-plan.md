# Self-Hosted Short URL Platform — Implementation Plan

## 1. Objective

Build a lightweight, production-ready, self-hosted URL shortener platform that can be installed easily using Docker Compose.

The application must support:

- User authentication
- Multiple workspaces
- Workspace members and roles
- Multiple custom domains per workspace
- Domain verification from the dashboard
- Short URL creation and management
- Fast redirect handling
- Redis caching
- Asynchronous click analytics
- API keys per workspace
- First-class machine-to-machine REST API for creating short links from existing internal systems
- Idempotent short-link creation for safe retries
- Automatic HTTPS for custom domains
- Easy self-hosted deployment using Docker Compose

Primary design goals:

1. Lightweight runtime and low memory usage
2. Very fast redirects
3. Simple deployment
4. Clean architecture
5. API-first design
6. Easy to scale later without overengineering MVP

---

# 2. Technology Stack

## Frontend

Use:

- Nuxt 4
- Vue 3
- TypeScript
- Tailwind CSS
- Pinia only when global client state is actually needed

Use Bun as the package manager and script runner for the frontend workspace.

```bash
bun install
bun run dev
bun run build
```

Commit `bun.lock`. Do not commit `package-lock.json`, `pnpm-lock.yaml`, or `yarn.lock`, and do not mix package managers across the repository.

Do not use SSR unless it provides a clear benefit. The dashboard can be built as a client-heavy application consuming the backend API.

## Backend

Use Go.

Recommended libraries:

- Router: `github.com/go-chi/chi/v5`
- PostgreSQL driver: `github.com/jackc/pgx/v5`
- SQL generation: `sqlc`
- Migration: `golang-migrate/migrate`
- Redis: `github.com/redis/go-redis/v9`
- Validation: lightweight manual validation or `go-playground/validator`
- JWT/session implementation: prefer secure HTTP-only cookie session unless there is a strong architectural reason for stateless JWT
- Password hashing: Argon2id or bcrypt
- Structured logs: Go `slog`

Avoid heavy ORMs.

Prefer explicit SQL via `sqlc`.

## Infrastructure

Use:

- PostgreSQL 17
- Redis 7 Alpine
- Traefik v3
- Docker Compose

Initial architecture:

```text
Browser
   |
   v
Traefik
   |
   +------------------------------+
   |                              |
   v                              v
Nuxt Dashboard                  Go Server
                                  |
                         +--------+--------+
                         |                 |
                         v                 v
                    PostgreSQL           Redis
```

The Go server initially handles both API requests and redirects.

Do NOT create separate microservices during MVP.

---

# 3. Repository Structure

Use a monorepo.

Target structure:

```text
shorturl/
├── apps/
│   ├── web/
│   │   ├── app/
│   │   ├── components/
│   │   ├── composables/
│   │   ├── layouts/
│   │   ├── middleware/
│   │   ├── pages/
│   │   ├── public/
│   │   ├── server/
│   │   ├── nuxt.config.ts
│   │   ├── bun.lock
│   │   └── package.json
│   │
│   └── server/
│       ├── cmd/
│       │   └── server/
│       │       └── main.go
│       ├── internal/
│       │   ├── auth/
│       │   ├── user/
│       │   ├── workspace/
│       │   ├── member/
│       │   ├── domain/
│       │   ├── link/
│       │   ├── redirect/
│       │   ├── analytics/
│       │   ├── apikey/
│       │   ├── middleware/
│       │   ├── database/
│       │   ├── cache/
│       │   └── config/
│       │
│       ├── db/
│       │   ├── migrations/
│       │   ├── queries/
│       │   └── sqlc.yaml
│       │
│       ├── go.mod
│       └── go.sum
│
├── docker/
│   ├── server/
│   ├── web/
│   └── traefik/
│
├── scripts/
│   ├── install.sh
│   └── dev.sh
│
├── docker-compose.yml
├── docker-compose.dev.yml
├── .env.example
├── Makefile
└── README.md
```

Keep business logic inside feature packages.

Do not create unnecessary abstraction layers.

---

# 4. Core Data Model

Use UUIDs for public-facing entity identifiers.

Internal numeric IDs are optional. Prefer UUID primary keys if the implementation remains simple.

## users

```text
id UUID PK
name VARCHAR
email VARCHAR UNIQUE
password_hash TEXT
created_at TIMESTAMPTZ
updated_at TIMESTAMPTZ
```

## workspaces

```text
id UUID PK
name VARCHAR
slug VARCHAR UNIQUE
owner_user_id UUID FK users
created_at TIMESTAMPTZ
updated_at TIMESTAMPTZ
```

## workspace_members

```text
workspace_id UUID FK
user_id UUID FK
role VARCHAR
created_at TIMESTAMPTZ

UNIQUE(workspace_id, user_id)
```

Initial roles:

```text
owner
admin
member
```

Do not build granular RBAC in MVP.

## domains

```text
id UUID PK
workspace_id UUID FK
hostname VARCHAR UNIQUE
status VARCHAR
verification_token VARCHAR
verification_method VARCHAR
ssl_status VARCHAR
is_default BOOLEAN
verified_at TIMESTAMPTZ NULL
created_at TIMESTAMPTZ
updated_at TIMESTAMPTZ
```

Domain statuses:

```text
pending
verifying
active
failed
```

Verification methods:

```text
dns_cname
dns_a
dns_txt
```

## links

```text
id UUID PK
workspace_id UUID FK
domain_id UUID FK
slug VARCHAR
destination_url TEXT
title VARCHAR NULL
status VARCHAR
redirect_type SMALLINT
password_hash TEXT NULL
expires_at TIMESTAMPTZ NULL
max_clicks BIGINT NULL
external_reference VARCHAR NULL
metadata JSONB NULL
created_by UUID FK users NULL
created_via VARCHAR NOT NULL DEFAULT 'dashboard'
created_at TIMESTAMPTZ
updated_at TIMESTAMPTZ

UNIQUE(domain_id, slug)

-- optionally enforce per-workspace external reference uniqueness when non-null
-- with a partial unique index
```

Supported redirect types:

```text
301
302
307
308
```

Default redirect should be `302`.

## click_events

```text
id UUID PK
link_id UUID FK
clicked_at TIMESTAMPTZ
ip_hash VARCHAR NULL
country VARCHAR NULL
city VARCHAR NULL
device VARCHAR NULL
os VARCHAR NULL
browser VARCHAR NULL
referrer TEXT NULL
utm_source VARCHAR NULL
utm_medium VARCHAR NULL
utm_campaign VARCHAR NULL
```

Do not store raw IP addresses by default.

Hash or anonymize them.

## api_keys

```text
id UUID PK
workspace_id UUID FK
name VARCHAR
key_prefix VARCHAR
key_hash TEXT
last_used_at TIMESTAMPTZ NULL
expires_at TIMESTAMPTZ NULL
created_by UUID FK users
created_at TIMESTAMPTZ
revoked_at TIMESTAMPTZ NULL
scopes TEXT[] NOT NULL DEFAULT ARRAY['links:read', 'links:write']
```

Never store plaintext API keys.


## idempotency_keys

```text
id UUID PK
workspace_id UUID FK
idempotency_key VARCHAR
request_hash VARCHAR
link_id UUID FK links
created_at TIMESTAMPTZ
expires_at TIMESTAMPTZ

UNIQUE(workspace_id, idempotency_key)
```

Used by the machine-to-machine API to make POST `/api/v1/links` safe to retry.


---

# 5. Authentication

Implement email/password authentication.

Required endpoints:

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

Prefer secure cookie-based authentication.

Cookie requirements:

```text
HttpOnly=true
Secure=true in production
SameSite=Lax
```

Passwords must be securely hashed.

Rate-limit login and registration endpoints.

---

# 6. Workspace Behavior

A user can belong to multiple workspaces.

Required functionality:

```text
Create workspace
List workspaces
Get workspace
Update workspace
Delete workspace
Switch active workspace on frontend
Invite/add members
Remove members
Change member role
```

MVP permission behavior:

### owner

Full access.

### admin

Can manage:

- links
- domains
- analytics
- members except changing/removing owner

### member

Can:

- view links
- create links
- update links they are allowed to manage
- view analytics

Keep authorization checks server-side.

Never trust workspace IDs supplied by the frontend without checking membership.

---

# 7. Domain Management

Custom domains are a first-class feature.

Dashboard path:

```text
Workspace
  -> Domains
```

Users must be able to add domains such as:

```text
go.example.com
link.example.org
short.company.id
```

Backend workflow:

```text
Add domain
    |
    v
Generate verification token
    |
    v
Show DNS instructions
    |
    v
User configures DNS
    |
    v
Verify Domain
    |
    v
Backend performs DNS lookup
    |
    +---- invalid ---> pending/failed
    |
    +---- valid -----> active
```

Prefer supporting TXT verification.

Example:

```text
Type: TXT
Name: _shorturl-verification.go.example.com
Value: shorturl-verification=<token>
```

Also verify that traffic routing points to the current ShortURL installation before activating the domain.

Required endpoints:

```text
GET    /api/v1/workspaces/{workspaceId}/domains
POST   /api/v1/workspaces/{workspaceId}/domains
GET    /api/v1/workspaces/{workspaceId}/domains/{domainId}
POST   /api/v1/workspaces/{workspaceId}/domains/{domainId}/verify
DELETE /api/v1/workspaces/{workspaceId}/domains/{domainId}
```

Do not require Docker Compose changes when adding domains.

Do not require container restart.

---

# 8. Dynamic Custom Domain Routing

Traefik must route all approved/custom hosts to the Go application.

The Go server must determine the target link using:

```text
HTTP Host + URL path slug
```

Example request:

```text
Host: go.company.com
Path: /promo
```

Logical lookup:

```sql
SELECT links.*
FROM links
JOIN domains ON domains.id = links.domain_id
WHERE domains.hostname = $1
  AND links.slug = $2
  AND domains.status = 'active'
  AND links.status = 'active'
LIMIT 1;
```

Normalize host names to lowercase.

Strip port numbers before lookup.

Normalize slug carefully.

Reserved slugs must exist for application paths such as:

```text
api
admin
login
register
health
metrics
favicon.ico
robots.txt
```

---

# 9. Redirect Engine

Redirect performance is critical.

Request flow:

```text
GET https://go.example.com/promo
        |
        v
Read host + slug
        |
        v
Redis lookup
        |
   +----+----+
   |         |
  HIT       MISS
   |         |
   |         v
   |     PostgreSQL
   |         |
   |         v
   |      Redis SET
   |         |
   +----+----+
        |
        v
Validate link state
        |
        v
HTTP redirect
```

Redis key format:

```text
shorturl:link:{hostname}:{slug}
```

Recommended cached payload:

```json
{
  "link_id": "uuid",
  "url": "https://destination.example.com",
  "redirect_type": 302,
  "expires_at": null,
  "max_clicks": null
}
```

Cache TTL may be 1 hour initially.

When a link is updated/deleted/disabled, invalidate its Redis entry immediately.

When a domain is disabled/deleted, invalidate links for that domain.

---

# 10. Redirect Rules

Before redirecting, validate:

- Link exists
- Link is active
- Domain is active
- Link has not expired
- Max-click restriction has not been exceeded
- Password protection if enabled

Return sensible HTTP statuses:

```text
404 link not found
410 expired/disabled if appropriate
429 rate limited
```

Do not expose sensitive internals in redirect error responses.

---

# 11. Analytics Pipeline

Analytics must NEVER significantly delay redirects.

Do not synchronously insert every click into PostgreSQL before returning a redirect.

Use Redis Streams.

Flow:

```text
Redirect request
      |
      +---------------------> HTTP 302
      |
      v
XADD click event
      |
      v
Redis Stream
      |
      v
Background worker
      |
      v
PostgreSQL
```

Stream name:

```text
shorturl:click-events
```

Consumer group:

```text
analytics-workers
```

Initially run the worker from the same Go binary using a separate command:

```bash
./shorturl-server serve
./shorturl-server worker
```

Docker Compose can run separate containers using the same image.

Analytics should capture where available:

- link ID
- timestamp
- anonymized IP hash
- user agent
- referrer
- UTM source
- UTM medium
- UTM campaign

GeoIP is optional for MVP.

Design the event payload so GeoIP enrichment can be added later.

---

# 12. Link Management

Required frontend capabilities:

```text
List links
Search links
Filter links
Create link
Edit link
Delete link
Enable/disable link
Copy short URL
View analytics
```

Create-link form fields:

```text
Domain
Destination URL
Custom slug optional
Title optional
Redirect type
Expiration optional
Password optional
Click limit optional
```

If slug is empty, generate a random slug.

Recommended generated slug length:

```text
6-8 URL-safe characters
```

Use cryptographically secure random generation.

Avoid ambiguous characters if possible.

Example alphabet:

```text
23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz
```

Required endpoints:

```text
GET    /api/v1/workspaces/{workspaceId}/links
POST   /api/v1/workspaces/{workspaceId}/links
GET    /api/v1/workspaces/{workspaceId}/links/{linkId}
PATCH  /api/v1/workspaces/{workspaceId}/links/{linkId}
DELETE /api/v1/workspaces/{workspaceId}/links/{linkId}
```

---

# 13. API Keys and Machine-to-Machine Authentication

The platform must treat API access as a first-class product feature, not merely a dashboard helper.

The primary integration use case is an existing internal system that already owns a long document/file URL and wants to request a short URL automatically before sharing it to users.

Example flow:

```text
Internal ERP / App
      |
      | POST /api/v1/links
      | Authorization: Bearer shr_live_xxx
      v
ShortURL API
      |
      +--> validate workspace API key
      +--> validate destination URL
      +--> choose workspace default domain
      +--> generate unique short slug
      +--> persist link
      +--> warm Redis cache
      |
      v
Return https://go.company.com/aB7kP2
      |
      v
Internal ERP shares short URL to WhatsApp / Email / SMS / UI
```

This should feel similar to how consumer applications return compact share URLs, except the short URL is created programmatically by the user's own internal system.

Workspaces can create multiple API keys, for example:

```text
ERP Production
ERP Staging
Notification Service
Document Service
```

Example key format:

```text
shr_live_<random-secret>
shr_test_<random-secret>
```

Only display the complete API key once when created.

Store only:

```text
prefix
hash
```

Example API usage:

```http
Authorization: Bearer shr_live_xxxxx
```

Required API key scopes for MVP:

```text
links:read
links:write
analytics:read
```

Default machine-to-machine key may receive only:

```text
links:read
links:write
```

API key endpoints used by the dashboard:

```text
GET    /api/v1/workspaces/{workspaceId}/api-keys
POST   /api/v1/workspaces/{workspaceId}/api-keys
DELETE /api/v1/workspaces/{workspaceId}/api-keys/{apiKeyId}
```

API keys must be bound to exactly one workspace. The public API should derive the workspace from the API key rather than trusting an arbitrary workspace ID supplied by the caller.

Record `last_used_at` asynchronously or with a low-cost strategy so that it does not become a bottleneck for every API call.

---

# 14. Public REST API — Core Integration Contract

The machine-to-machine REST API is part of MVP and must remain stable and easy to consume from Laravel, Go, Node.js, Python, mobile backends, automation tools, and other internal services.

## 14.1 Create Short Link

Primary endpoint:

```http
POST /api/v1/links
Authorization: Bearer shr_live_xxxxx
Content-Type: application/json
Idempotency-Key: invoice-8f4e2b8a
```

Minimal request:

```json
{
  "destination_url": "https://internal.company.com/documents/very-long-url?token=..."
}
```

The API key determines the workspace. If `domain` is omitted, use the workspace default active domain. If `slug` is omitted, generate a cryptographically secure random slug.

Response:

```json
{
  "data": {
    "id": "01J...",
    "short_url": "https://go.company.com/aB7kP2",
    "slug": "aB7kP2",
    "domain": "go.company.com",
    "destination_url": "https://internal.company.com/documents/very-long-url?token=...",
    "expires_at": null,
    "created_at": "2026-08-13T05:30:00+07:00"
  }
}
```

The most important response field for integrations is:

```text
data.short_url
```

An existing system should be able to call this endpoint and immediately use that value in WhatsApp, email, SMS, push notification, QR code, or its own UI.

## 14.2 Create With Optional Controls

Fuller request example:

```json
{
  "destination_url": "https://dashboard.company.com/document/0198c7e2-...",
  "domain": "go.company.com",
  "slug": "invoice-12345",
  "title": "Invoice #12345",
  "redirect_type": 302,
  "expires_at": "2026-09-13T00:00:00Z",
  "external_reference": "invoice:12345",
  "metadata": {
    "source": "erp",
    "document_type": "invoice",
    "document_id": "12345"
  }
}
```

Fields:

```text
destination_url      required

domain               optional; defaults to workspace default domain
slug                 optional; random when omitted
title                optional
redirect_type        optional; default 302
expires_at           optional
external_reference   optional stable identifier from caller
metadata             optional JSON object for caller-owned metadata
```

`metadata` is for integration context only. It must not affect redirect routing and should have a reasonable size limit, for example 8 KB.

## 14.3 Idempotency

Safe retries are mandatory.

Support:

```http
Idempotency-Key: <caller-generated-unique-value>
```

Expected behavior:

```text
first request with key X
    -> create link
    -> return 201

same request retried with key X
    -> return the original result
    -> do NOT create a second link
```

Persist idempotency records with at least:

```text
workspace_id
idempotency_key
request_hash
link_id
created_at
expires_at
```

If the same key is reused with a materially different request body, return a conflict error rather than silently returning or creating another resource.

Suggested error:

```json
{
  "error": {
    "code": "idempotency_conflict",
    "message": "The idempotency key was already used with a different request"
  }
}
```

An idempotency retention window of 24 hours is sufficient for MVP.

`external_reference` provides an additional caller-controlled stable reference. It can be used to look up a link later and, when present, SHOULD be unique per workspace.

Example:

```text
external_reference = "invoice:12345"
external_reference = "travel-document:0198c7e2..."
external_reference = "payslip:2026-08:user-193"
```

## 14.4 Read Short Link

```http
GET /api/v1/links/{linkId}
Authorization: Bearer shr_live_xxxxx
```

Only return links belonging to the API key's workspace.

## 14.5 Lookup by External Reference

```http
GET /api/v1/links/by-reference/{externalReference}
Authorization: Bearer shr_live_xxxxx
```

This allows an internal system to ask:

```text
"Have I already created a short URL for invoice:12345?"
```

## 14.6 Update Short Link

```http
PATCH /api/v1/links/{linkId}
Authorization: Bearer shr_live_xxxxx
```

Allow controlled updates such as:

```text
destination_url
title
expires_at
status
metadata
```

Slug/domain changes may be supported, but must correctly invalidate old Redis cache entries.

## 14.7 Delete / Disable Short Link

Prefer soft-disabling links for API integrations.

```http
DELETE /api/v1/links/{linkId}
Authorization: Bearer shr_live_xxxxx
```

or:

```http
PATCH /api/v1/links/{linkId}
{
  "status": "disabled"
}
```

The implementation may use soft delete internally so analytics/history can remain available.

## 14.8 List Links

```http
GET /api/v1/links?limit=50&cursor=...
Authorization: Bearer shr_live_xxxxx
```

Support filters where useful:

```text
status
external_reference
domain
created_after
created_before
```

Use cursor pagination rather than unbounded page-number queries when practical.

## 14.9 Consistent Response Format

Success:

```json
{
  "data": {}
}
```

List:

```json
{
  "data": [],
  "meta": {
    "next_cursor": null
  }
}
```

Error:

```json
{
  "error": {
    "code": "validation_error",
    "message": "Invalid request",
    "fields": {
      "destination_url": ["must be a valid http or https URL"]
    },
    "request_id": "req_..."
  }
}
```

Suggested status codes:

```text
200 read/update/replayed idempotent response
201 created
400 invalid request
401 invalid/missing API key
403 insufficient API-key scope
404 resource not found in workspace
409 duplicate slug / idempotency conflict
422 validation failure if preferred consistently
429 rate limited
500 unexpected server error
```

## 14.10 Public API Rate Limiting

Rate-limit by API key, not merely by client IP.

Initial defaults can be configurable, for example:

```text
120 requests/minute per API key
```

Make limits configurable using environment/application settings rather than hardcoding business assumptions.

Return standard rate-limit headers where practical.

## 14.11 API Documentation

Generate and maintain an OpenAPI specification for the public API.

Expected artifacts:

```text
docs/openapi.yaml
```

Expose optional interactive documentation from the main application domain, for example:

```text
/docs/api
```

Interactive documentation must not expose real API secrets.

## 14.12 Internal-System Integration Example

Pseudo flow from an existing application:

```text
User clicks Share Document
        |
        v
Existing backend already has long URL
        |
        v
POST ShortURL /api/v1/links
        |
        v
Receive data.short_url
        |
        v
Share the short URL
```

Example pseudocode:

```text
longUrl = buildSecureDocumentUrl(document)

result = shortUrlClient.create({
    destination_url: longUrl,
    external_reference: "document:" + document.id
}, {
    idempotency_key: "document:" + document.id
})

share(result.short_url)
```

The shortener must not need to understand the document itself. It only stores and redirects to the destination URL supplied by the trusted caller.

For long-lived sensitive document URLs, recommend that the destination system continue enforcing its own authentication/authorization. A short URL is an identifier and redirect mechanism, not an authorization boundary.

---

# 15. Dashboard Pages

Implement these pages.

## Authentication

```text
/login
/register
```

## Workspace

```text
/dashboard
```

## Links

```text
/dashboard/links
/dashboard/links/new
/dashboard/links/:id
```

## Domains

```text
/dashboard/domains
```

## Analytics

```text
/dashboard/analytics
```

## Members

```text
/dashboard/members
```

## API Keys

```text
/dashboard/api-keys
```

## Settings

```text
/dashboard/settings
```

Global dashboard layout should contain:

```text
Sidebar
Workspace selector
User menu
Primary content
```

Keep UI minimal and functional.

Do not spend excessive implementation time on animations or visual polish before core functionality is complete.

---

# 16. Dashboard Overview

Workspace overview should show:

```text
Total Links
Active Links
Total Clicks
Clicks Today
Active Domains
Recent Links
Recent Activity
```

Charts can be implemented after the redirect and analytics pipeline is functioning correctly.

---

# 17. Analytics API

Initial endpoints:

```text
GET /api/v1/workspaces/{workspaceId}/analytics/overview
GET /api/v1/workspaces/{workspaceId}/analytics/clicks
GET /api/v1/workspaces/{workspaceId}/links/{linkId}/analytics
```

Supported date ranges:

```text
24h
7d
30d
90d
custom
```

Initial reports:

- Clicks over time
- Top links
- Top referrers
- UTM sources
- Device breakdown if parsed

## 17.1 Storage Architecture — Rollup Tables, Not Raw Scans

PostgreSQL is the analytics store. It is fast enough — but only because
reporting queries never scan the raw click log.

Three tables:

```text
click_events           raw log, append-only, bigint identity PK
click_hourly           (workspace_id, link_id, bucket, clicks)
click_dimension_daily  (workspace_id, day, dimension, value, clicks)
```

The analytics worker maintains the two rollup tables as it drains the Redis
stream. It aggregates each batch **in memory first**, so a batch of 256 click
events collapses into a handful of `INSERT ... ON CONFLICT DO UPDATE`
statements rather than 256 of them.

Every reporting query reads a rollup table. None of them touch `click_events`.

Why this matters:

```text
90-day chart for one link
  scanning click_events    ~ every click ever recorded for that link
  reading click_hourly     ~ 2,160 rows  (24 x 90)

workspace top-referrers over 30 days
  scanning click_events    ~ every click in the workspace
  reading click_dimension_daily ~ 30 x (distinct referrers)
```

`click_events` is retained only for per-click detail views and export, and is
pruned after `CLICK_RETENTION_DAYS` (default 400). The rollups are never
pruned — they are small enough to keep forever.

`click_dimension_daily` uses one generic `(dimension, value)` pair rather than
a table per dimension. Adding a breakdown later — country, city, OS version —
becomes a new value in that column instead of a migration.

## 17.2 ClickHouse — Decision and Migration Path

**Decision: not in MVP, and not at the scale this platform is being built for.**

ClickHouse is genuinely 10–100x faster than PostgreSQL for large analytical
scans. That advantage is real, but it only pays off once there is a large scan
to do. With rollup tables, there is not: the reporting queries read hundreds of
rows, and at that size PostgreSQL's row store wins on latency because there is
no distributed query planner or part-merge overhead to amortise.

What ClickHouse would cost today:

```text
+1.0-2.0 GB RAM at idle          (the whole stack currently fits in ~500 MB)
+1 database to back up, migrate, monitor, and restore
+1 schema kept in sync with PostgreSQL
+  a second consistency model (eventual, no transactions, no FKs)
```

That contradicts design goal #1 (lightweight runtime, low memory usage) and
makes "move the deployment to another server" materially harder.

### When to revisit

Switch when **any** of these is true, measured not guessed:

```text
click_events exceeds ~100 million rows
sustained ingest above ~2,000 clicks/second
p95 of an analytics API endpoint exceeds 500 ms after
    EXPLAIN ANALYZE confirms the rollup indexes are being used
raw-event retention needs to exceed ~2 years
per-click drill-down over the full history becomes a product requirement
```

Below those thresholds, adding ClickHouse makes the system slower to operate
and no faster to use.

### Intermediate steps to take first

In order of cost, each of these buys headroom without a new dependency:

1. **Partition `click_events` by month** (`PARTITION BY RANGE (clicked_at)`).
   Retention pruning becomes `DROP PARTITION` — instant, no vacuum churn.
2. **`BRIN` index on `clicked_at`** instead of B-tree. Append-only time-series
   data is physically ordered by time, which is exactly what BRIN exploits, at
   a fraction of the index size.
3. **Add `click_daily`** alongside `click_hourly` for ranges beyond 90 days.
4. **Compress old raw events** or export them to Parquet on object storage and
   drop them from PostgreSQL.

### How the code is prepared for the switch

The analytics reporting layer sits behind an interface. Handlers depend on the
interface, not on `store.Queries`:

```go
type Reader interface {
    Overview(ctx context.Context, workspaceID uuid.UUID) (Overview, error)
    Series(ctx context.Context, q SeriesQuery) ([]Point, error)
    TopLinks(ctx context.Context, q TopQuery) ([]LinkStat, error)
    TopDimension(ctx context.Context, q DimensionQuery) ([]ValueStat, error)
}
```

Migrating means writing a second `Reader` backed by ClickHouse and a second
sink in the worker. No handler, no route, and no frontend code changes.

The recommended migration is **dual-write, not cutover**:

```text
1. Add ClickHouse to Compose behind an optional Compose profile.
2. Worker writes each batch to PostgreSQL AND ClickHouse.
3. Backfill history from click_events.
4. Compare query results between the two Readers for a week.
5. Flip ANALYTICS_BACKEND=clickhouse.
6. Keep the PostgreSQL rollups as the fallback for one more release.
```

If ClickHouse is ever adopted, record the trigger metric that justified it in
an ADR, so the decision stays reviewable.

---

# 18. Security Requirements

Security must be part of the initial implementation.

## Destination URL validation

Allow only:

```text
http://
https://
```

Reject dangerous schemes:

```text
javascript:
data:
file:
ftp:
```

## SSRF protection

If the server ever fetches destination URLs for metadata or previews, block private/internal networks including:

```text
127.0.0.0/8
10.0.0.0/8
172.16.0.0/12
192.168.0.0/16
169.254.0.0/16
::1
fc00::/7
fe80::/10
```

For MVP, do not fetch destination URLs server-side unless necessary.

## Rate limiting

Apply rate limiting to:

- Login
- Registration
- Link creation
- Domain verification
- Public API

Redirect rate limiting should be conservative so normal redirect traffic is not accidentally blocked.

## API keys

Never log full API keys.

## Logging

Never log:

- passwords
- auth cookies
- full API keys
- sensitive authorization headers

---

# 19. Docker Compose Requirements

The application must install with:

```bash
git clone <repository>
cd shorturl
cp .env.example .env
docker compose up -d
```

Target services:

```text
web
server
worker
postgres
redis
traefik
```

Use the same Go image for `server` and `worker`.

Example conceptual Compose configuration:

```yaml
services:
  server:
    build:
      context: ./apps/server
    command: ["/app/shorturl", "serve"]
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  worker:
    build:
      context: ./apps/server
    command: ["/app/shorturl", "worker"]
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  web:
    build:
      context: ./apps/web

  postgres:
    image: postgres:17-alpine

  redis:
    image: redis:7-alpine

  traefik:
    image: traefik:v3
```

Use health checks.

Use named volumes for PostgreSQL and Traefik certificate storage.

Do not expose PostgreSQL or Redis publicly.

---

# 20. Environment Variables

Provide `.env.example`.

Suggested environment variables:

```env
APP_ENV=production
APP_NAME=ShortURL
APP_DOMAIN=short.example.com
APP_URL=https://short.example.com

SERVER_PORT=8080

POSTGRES_DB=shorturl
POSTGRES_USER=shorturl
POSTGRES_PASSWORD=change-this
DATABASE_URL=postgres://shorturl:change-this@postgres:5432/shorturl?sslmode=disable

REDIS_ADDR=redis:6379
REDIS_PASSWORD=

SESSION_SECRET=change-this
IP_HASH_SECRET=change-this

TRAEFIK_ACME_EMAIL=admin@example.com

NUXT_PUBLIC_API_BASE_URL=https://short.example.com/api/v1
```

Do not commit production secrets.

`DATABASE_URL` is assembled in `docker-compose.yml` from `POSTGRES_USER`,
`POSTGRES_PASSWORD`, and `POSTGRES_DB` rather than being written out in full, so
rotating the password cannot leave a stale copy behind. Setting it explicitly
overrides the composed value and is the way to point at an external database.

Every secret uses Compose's `${VAR:?message}` form: a missing value stops the
stack with a readable error instead of silently starting with a weak default.

---

# 21. Automatic HTTPS

Use Traefik ACME / Let's Encrypt.

The platform must support HTTPS for domains added from the dashboard without editing Compose manually.

Required behavior:

```text
User adds domain
     |
     v
Domain verification succeeds
     |
     v
DNS routes to this server
     |
     v
Traefik serves custom host
     |
     v
ACME certificate issued automatically
```

Be careful with certificate issuance limits.

Do not request certificates repeatedly for invalid or unverified domains.

If fully dynamic certificate routing through Traefik becomes awkward during implementation, prioritize a clean working design over hacks.

Caddy may be considered as an alternative ONLY if it significantly simplifies dynamic domain TLS management.

Document the decision in an ADR if Traefik is replaced.

---

# 22. Installation Experience

Installation must be friendly for self-hosters.

Target:

```bash
cp .env.example .env
./scripts/install.sh
docker compose up -d
```

`install.sh` may:

1. Generate SESSION_SECRET
2. Generate IP_HASH_SECRET
3. Generate a strong PostgreSQL password
4. Update `.env`
5. Check Docker availability
6. Print next steps

Do not overwrite user-defined values silently.

---

# 23. Initial Setup Wizard

On first install, if no user exists, redirect dashboard access to setup.

Suggested flow:

```text
Welcome
  -> Create administrator
  -> Create first workspace
  -> Confirm application domain
  -> Finish
```

After setup is complete, the setup endpoint must not allow a second initial administrator to be created.

Persist setup state in the database, not only an environment variable.

---

# 24. Health Checks

Implement:

```text
GET /health
GET /ready
```

`/health` checks application process health.

`/ready` checks required dependencies:

- PostgreSQL
- Redis

Do not expose secrets in health output.

---

# 25. Observability

Use structured JSON logs in production.

Recommended fields:

```text
timestamp
level
message
request_id
method
path
status
latency_ms
workspace_id when available
user_id when available
```

Add request IDs.

Do not implement Prometheus unless simple to add without delaying MVP.

Prepare architecture so metrics can be added later.

---

# 26. Database Indexes

At minimum create indexes for:

```sql
CREATE UNIQUE INDEX domains_hostname_unique
ON domains (LOWER(hostname));

CREATE UNIQUE INDEX links_domain_slug_unique
ON links (domain_id, slug);

CREATE INDEX links_workspace_created_idx
ON links (workspace_id, created_at DESC);

CREATE INDEX click_events_link_clicked_idx
ON click_events (link_id, clicked_at DESC);

CREATE INDEX click_events_clicked_at_idx
ON click_events (clicked_at DESC);

CREATE INDEX workspace_members_user_idx
ON workspace_members (user_id, workspace_id);

CREATE UNIQUE INDEX links_workspace_external_reference_unique
ON links (workspace_id, external_reference)
WHERE external_reference IS NOT NULL;

CREATE UNIQUE INDEX api_keys_prefix_unique
ON api_keys (key_prefix);
```

Review query plans if analytics queries become slow.

---

# 27. Cache Invalidation

Caching must be deterministic.

On:

```text
link create
link update
link delete
link enable
link disable
domain disable
domain delete
```

invalidate affected Redis keys.

Do not rely solely on TTL for correctness.

---

# 28. Development Commands

Provide a Makefile.

Expected commands:

```bash
make dev
make up
make down
make logs
make test
make lint
make migrate
make migrate-down
make sqlc
make build
```

README must document them.

---

# 29. Testing Strategy

## Backend unit tests

Test business logic for:

- workspace authorization
- slug generation
- domain normalization
- URL validation
- link expiration
- redirect status
- cache behavior
- API key validation

## Backend integration tests

Use PostgreSQL and Redis test containers where practical.

Test:

- registration/login
- workspace CRUD
- domain CRUD
- link CRUD
- redirect lookup
- cache miss -> database -> cache
- cache invalidation
- analytics event enqueue

## Frontend tests

At minimum test critical composables/stores and important forms.

Do not spend excessive time chasing arbitrary coverage percentages.

Prioritize critical paths.

---

# 30. MVP Scope

**Status: complete.** Every item below is implemented and verified end-to-end
against a running stack. See `docs/build-report.md` for what was tested and how,
including the three items marked with a note.

The MVP is complete when all items below work.

## Authentication

- [x] Register
- [x] Login
- [x] Logout
- [x] Current user

## Workspace

- [x] Create workspace
- [x] List workspace
- [x] Workspace switcher
- [x] Workspace membership authorization
- [x] Owner/admin/member roles

## Domain

- [x] Add domain
- [x] DNS verification instructions
- [x] Verify domain
- [x] Domain active state
- [x] Custom-domain requests resolve correctly

## Links

- [x] Create short URL
- [x] Custom slug
- [x] Random slug
- [x] Update link
- [x] Delete link
- [x] Enable/disable
- [x] Expiration
- [x] Redirect type
- [x] Copy short URL

## Redirect

- [x] Host + slug lookup
- [x] Redis caching
- [x] PostgreSQL fallback
- [x] Correct 301/302/307/308 redirect
- [x] Expired link handling
- [x] Disabled link handling
- [x] Cache invalidation

## Analytics

- [x] Redis Streams event enqueue
- [x] Background analytics worker
- [x] Persist click events
- [x] Workspace overview click count
- [x] Link click count
- [x] Clicks-over-time query

## API Keys

- [x] Create key
- [x] Reveal once
- [x] Hash storage
- [x] Revoke key
- [x] API authentication

## Public API Integration

- [x] `POST /api/v1/links` with API-key authentication
- [x] Workspace derived from API key
- [x] Default domain when domain omitted
- [x] Random slug when slug omitted
- [x] Return immediately usable `data.short_url`
- [x] `Idempotency-Key` support
- [x] `external_reference` support
- [x] Lookup by external reference
- [x] Read/update/disable link through API
- [x] API-key scopes
- [x] API-key based rate limiting
- [x] OpenAPI documentation

## Deployment

- [x] Production Dockerfiles
- [x] docker-compose.yml
- [x] `.env.example`
- [x] health checks
- [x] persistent volumes
- [x] automatic migration execution strategy
- [x] Traefik proxy
- [x] HTTPS for main application domain
- [x] documented custom-domain DNS setup

---

# 31. Features Explicitly Out of MVP

Do NOT implement these until MVP is stable:

- ClickHouse — see §17.2 for the decision, the measured triggers that would
  justify it, and the migration path. The reporting layer is written behind an
  interface so the switch does not touch handlers.
- Kafka
- NATS
- Kubernetes
- Microservices
- PostgreSQL replicas
- Redis Cluster
- A/B redirects
- Geo-targeting
- Device-targeting redirects
- Link-in-bio pages
- AI features
- Billing/subscriptions
- SSO/SAML
- Complex granular RBAC
- Custom OpenGraph scraper
- Browser extension
- Mobile applications

Do not overengineer.

---

# 32. Phase 2

After MVP is stable, add:

- QR codes
- Bulk link creation
- CSV import/export
- Tags
- Link folders
- Webhooks
- Better analytics
- GeoIP enrichment
- Browser/device parser
- Audit logs
- Custom social metadata
- Per-link password protection UI
- Link scheduling
- Better API-key scopes
- Backup/restore documentation

---

# 33. Phase 3 — High Traffic Architecture

Only when proven necessary, separate Go processes into logical services:

```text
API service
Redirect service
Analytics worker
```

Potential architecture:

```text
                    +----------------+
                    | Nuxt Dashboard |
                    +-------+--------+
                            |
                            v
                       +---------+
                       | Go API  |
                       +----+----+
                            |
                     PostgreSQL

Internet
   |
   v
Traefik
   |
   v
+----------------+
| Redirect Go    |
+-------+--------+
        |
      Redis
        |
        +---------> PostgreSQL fallback
        |
        +---------> event stream
                       |
                       v
                Analytics Worker
                       |
                       v
                   ClickHouse
```

Do not implement this architecture prematurely.

---

# 34. Coding Principles

Follow these rules throughout implementation:

1. Keep code straightforward.
2. Favor explicit SQL over ORM magic.
3. Avoid premature abstraction.
4. Do not introduce microservices during MVP.
5. Keep HTTP handlers thin.
6. Put business rules in feature/domain packages.
7. Always enforce workspace authorization server-side.
8. Keep redirect path extremely lightweight.
9. Avoid synchronous analytics writes during redirect.
10. Use context cancellation correctly in Go.
11. Use database transactions where multiple writes must be atomic.
12. Return typed, consistent API errors.
13. Validate all user input.
14. Never expose stack traces in production responses.
15. Keep Docker images small using multi-stage builds.
16. Run containers as non-root wherever practical.
17. Keep dependencies minimal.

---

# 35. Backend Package Pattern

Each major feature may follow a structure similar to:

```text
internal/link/
├── handler.go
├── service.go
├── repository.go
├── model.go
├── validation.go
└── errors.go
```

However, do not mechanically create all files for tiny features.

Use the simplest structure that remains maintainable.

Expected flow:

```text
HTTP Handler
    |
    v
Service / Business Logic
    |
    v
Generated sqlc Queries / Repository helper
    |
    v
PostgreSQL
```

---

# 36. API Middleware Order

Recommended middleware:

```text
Request ID
Real IP handling
Recovery
Structured logging
Security headers
CORS where required
Rate limiter
Authentication
Workspace authorization
Handler
```

Be careful trusting forwarded headers.

Only trust proxy headers when requests originate from the configured reverse proxy/network.

---

# 37. Frontend API Architecture

Create a reusable API client.

Example:

```text
apps/web/composables/useApi.ts
```

Do not scatter raw `$fetch` calls randomly throughout pages.

Organize business API functions such as:

```text
services/auth.ts
services/workspaces.ts
services/domains.ts
services/links.ts
services/analytics.ts
services/apiKeys.ts
```

Keep server state on the server/API rather than duplicating it unnecessarily in Pinia.

---

# 38. UX Requirements

Important UX behaviors:

- Workspace switcher must be accessible globally.
- Copy-short-link action should be one click.
- Domain status should be visually clear.
- Domain verification instructions should be copyable.
- Failed verification should explain what DNS record was found versus expected.
- Link creation should default to the workspace default domain.
- Random slug should be automatically generated if custom slug is empty.
- Destructive actions must require confirmation.
- API keys must clearly state that the secret is shown only once.

---

# 39. Domain Verification UX

Example dashboard flow:

```text
Add Domain

Domain:
go.example.com

Expected DNS

CNAME
Name: go
Target: short.example.com

TXT Verification
Name: _shorturl-verification.go.example.com
Value: shorturl-verification=abc123

[Verify Domain]
```

After verification:

```text
Status: Active
DNS: Verified
SSL: Active
```

Display SSL state separately from DNS state.

---

# 40. Main Application Domain

The main application domain serves the dashboard and API.

Example:

```text
https://short.example.com
```

Routes:

```text
https://short.example.com/dashboard
https://short.example.com/api/v1/*
```

Do not allow users to create arbitrary short links on the main application domain if they conflict with dashboard/API routes unless explicitly designed around reserved paths.

Prefer requiring at least one redirect domain.

Optionally allow a configured default short domain later.

---

# 41. Main Go Server Commands

Use one compiled binary with commands.

Expected commands:

```bash
shorturl serve
shorturl worker
shorturl migrate
shorturl version
```

Optional:

```bash
shorturl create-admin
```

This makes Docker images and operations simpler.

---

# 42. Migration Strategy

Production startup must not have multiple containers racing migrations.

Preferred options:

1. Dedicated one-shot `migrate` Compose service, or
2. Explicit installation command before app services start.

Recommended Compose dependency flow:

```text
postgres healthy
      |
      v
migrate completed
      |
      +----> server
      |
      +----> worker
```

---

# 43. Docker Image Goals

Go server image:

- Multi-stage build
- Compile static binary where possible
- Final image should use distroless or Alpine if compatible
- Non-root user

Nuxt image:

- Multi-stage Bun build (`oven/bun` builder stage)
- Install with `bun install --frozen-lockfile`
- Only production runtime dependencies in final image

Because the dashboard runs as an SPA (`ssr: false`), `bun run generate` produces
plain static files and the runtime stage is nginx rather than a Bun or Node
process. This removes a JavaScript runtime from the production stack entirely.
Revisit only if SSR is ever adopted.

Do not ship source/build caches in production images.

---

# 44. Backup Strategy

Document basic backup procedures.

At minimum:

```bash
pg_dump
```

Redis data is not the primary source of truth for links.

Click analytics events still waiting in Redis Streams may be lost if Redis persistence is disabled, so configure Redis persistence appropriately or clearly document the tradeoff.

Recommended Redis persistence:

```text
AOF enabled
```

---

# 45. Performance Targets

These are goals, not hard guarantees.

Redirect path on Redis hit should minimize work and avoid unnecessary allocations/database calls.

Target behavior:

```text
Redis HIT
-> no PostgreSQL lookup
-> enqueue lightweight analytics event
-> redirect immediately
```

Do not synchronously resolve GeoIP during redirects.

Do not synchronously parse expensive metadata during redirects.

---

# 46. Acceptance Criteria

The application can be considered MVP-ready when the following end-to-end scenario works:

1. Fresh machine has Docker and Docker Compose.
2. Operator clones the repository.
3. Operator creates `.env` from example.
4. Operator starts Docker Compose.
5. Database migrations complete.
6. User opens the dashboard.
7. User creates the first administrator account.
8. User creates a workspace.
9. User adds `go.example.com`.
10. Dashboard displays DNS records required for verification.
11. User configures DNS.
12. User clicks Verify.
13. Domain becomes Active.
14. HTTPS certificate becomes available.
15. User creates slug `hello` pointing to `https://example.org`.
16. Opening `https://go.example.com/hello` redirects correctly.
17. A second request is served from Redis cache.
18. Click analytics are queued asynchronously.
19. Worker persists analytics to PostgreSQL.
20. Dashboard shows the click count.
21. User edits the destination URL.
22. Redis cache is invalidated.
23. Next request redirects to the new destination.
24. Workspace API key can create a new short URL via API.
25. API caller can send only `destination_url` and receive an immediately usable `data.short_url`.
26. Retrying the same request with the same `Idempotency-Key` does not create a duplicate link.
27. API caller can attach `external_reference` and later look up the same short link.
28. An API key from workspace A cannot read or mutate links from workspace B.
29. Revoked API keys stop working immediately.
30. Restarting containers does not lose PostgreSQL data or configuration.

---

# 47. Implementation Order for Coding Agent

The coding agent MUST implement the project incrementally in the following order.

## Milestone 1 — Bootstrap

- Initialize repository structure
- Create Go module
- Create Nuxt application
- Add Docker Compose development stack
- Add PostgreSQL
- Add Redis
- Add basic Go health endpoint
- Add Makefile
- Add `.env.example`

Stop and verify all containers start correctly before continuing.

## Milestone 2 — Database Foundation

- Add migrations
- Configure pgx
- Configure sqlc
- Implement database connection
- Add user/workspace/member tables
- Add domain/link tables
- Add analytics/API key tables

Run migrations and tests.

## Milestone 3 — Authentication

- Register
- Login
- Logout
- Session middleware
- Current user endpoint
- Authentication frontend

Verify authentication end-to-end.

## Milestone 4 — Workspace

- Workspace CRUD
- Membership model
- Workspace authorization middleware
- Workspace switcher UI

Verify one user cannot access another workspace.

## Milestone 5 — Domains

- Domain CRUD
- DNS verification token generation
- DNS lookup implementation
- Domain verification UI
- Domain status handling

Verify domain ownership/routing workflow.

## Milestone 6 — Links

- Link CRUD
- Slug generation
- URL validation
- Domain/slug uniqueness
- Dashboard links UI

Verify short URL generation.

## Milestone 7 — Redirect Engine

- Host extraction
- Slug extraction
- Database link resolution
- Correct redirect responses
- Expiration handling
- Disabled link handling

Benchmark basic redirect path.

## Milestone 8 — Redis Cache

- Cache link resolutions
- Implement cache miss fallback
- Implement invalidation

Verify updates immediately change redirects.

## Milestone 9 — Analytics

- Redis Stream producer
- Worker consumer group
- Click event persistence
- Basic analytics API
- Dashboard stats

Verify redirects are not blocked by PostgreSQL analytics inserts.

## Milestone 10 — Public API and API Keys

- API key creation and reveal-once UX
- Secure API key hashing
- API-key scopes
- Authentication middleware
- Derive workspace from API key
- `POST /api/v1/links` machine-to-machine endpoint
- Default-domain resolution
- Random slug generation
- `Idempotency-Key` persistence and replay handling
- `external_reference` uniqueness and lookup endpoint
- Read/update/disable/list public API endpoints
- API-key based rate limiting
- OpenAPI specification
- Revocation

Verify the full internal-system integration scenario: long URL -> API call -> short URL -> redirect.

## Milestone 11 — Production Docker

- Production Dockerfiles
- Traefik
- Main-domain HTTPS
- Dynamic custom host routing
- Health checks
- Persistent volumes
- Migration service

Verify from a clean deployment.

## Milestone 12 — Documentation and Hardening

- README
- Installation guide
- DNS setup guide
- Backup guide
- Security review
- Rate limits
- Error handling review
- Integration tests

---

# 48. Rules for the Coding Agent

When executing this plan:

1. Work milestone-by-milestone.
2. Do not implement future milestones early unless required by a dependency.
3. After each milestone, run the relevant tests/build/lint commands.
4. Fix failures before continuing.
5. Keep the repository runnable at every milestone.
6. Do not silently change the selected technology stack.
7. Do not introduce an ORM.
8. Do not introduce a new infrastructure dependency without a strong reason.
9. Do not replace PostgreSQL or Redis.
10. Do not create Kubernetes manifests.
11. Do not create unnecessary microservices.
12. Prefer standard library Go functionality whenever reasonable.
13. Keep dependencies current and actively maintained.
14. Document architectural decisions that materially deviate from this plan.
15. Never hardcode secrets.
16. Never expose PostgreSQL or Redis ports publicly in production Compose.
17. Do not weaken authentication or domain verification for convenience.
18. Before declaring MVP finished, execute the full Acceptance Criteria scenario.

---

# 49. Suggested First Instruction to Codex / Coding Agent

Use this repository plan as the source of truth.

Start by reading this entire document before making changes.

Then execute **Milestone 1 — Bootstrap only**.

Requirements:

- Create the monorepo structure described here.
- Bootstrap the Go backend using Chi.
- Bootstrap the Nuxt 4 frontend with TypeScript and Tailwind CSS, managed with Bun.
- Add PostgreSQL 17 and Redis 7 to `docker-compose.dev.yml`.
- Create `.env.example`.
- Add a Go `/health` endpoint.
- Add a Makefile with at least `dev`, `up`, `down`, `logs`, `test`, and `build` commands.
- Add minimal README instructions for local startup.
- Use production-minded defaults but do not implement authentication, database schema, workspaces, domains, links, redirects, or analytics yet.
- Run builds/tests and resolve all errors.

At the end:

1. Summarize files created or changed.
2. Show commands used to verify the milestone.
3. State any architectural decisions made.
4. Do not proceed to Milestone 2 until explicitly instructed.

---

# 50. Final Product Vision

The completed application should feel like a lightweight self-hosted alternative to managed URL shortening platforms.

A typical self-hoster should be able to:

```text
docker compose up -d
```

then open a dashboard, create a workspace, connect a domain, create links, and immediately use that domain for redirects without manually editing proxy configuration every time a new custom domain is added. Existing internal systems must also be able to create short links automatically through a stable REST API and immediately receive the generated share URL.

The most important characteristics are:

```text
Simple
Fast
Lightweight
Reliable
Secure
Easy to self-host
Easy to maintain
```

When tradeoffs arise, prioritize those properties over adding more features.
