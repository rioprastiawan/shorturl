# Deployment guide

Everything ShortURL needs in production is described by one file,
`docker-compose.yml`, plus one configuration file, `.env`. There is nothing to
install on the host beyond Docker.

- [Prerequisites](#prerequisites)
- [First install](#first-install)
- [How TLS works](#how-tls-works)
- [Custom domains](#custom-domains)
- [Backups](#backups)
- [Moving to another server](#moving-to-another-server)
- [Upgrades](#upgrades)
- [Operations reference](#operations-reference)
- [Troubleshooting](#troubleshooting)

## Prerequisites

- A Linux server with a public IPv4 address
- Docker Engine 24+ with Compose v2
- **Ports 80 and 443 free.** Traefik binds both. If the host already runs
  nginx, Apache, or Caddy, stop it first — Let's Encrypt's HTTP challenge needs
  port 80.
- A domain whose A record points at the server

Check the ports before you start:

```bash
sudo ss -lntp '( sport = :80 or sport = :443 )'
```

## First install

```bash
git clone <repository> shorturl
cd shorturl

cp .env.example .env
./scripts/install.sh
```

`install.sh` verifies Docker, generates `POSTGRES_PASSWORD`, `REDIS_PASSWORD`,
`SESSION_SECRET`, and `IP_HASH_SECRET`, sets `.env` to mode 600, and tells you
what still needs a real value. It never overwrites a value that is already set,
so re-running it is safe.

Edit `.env` and set at minimum:

```env
APP_DOMAIN=short.yourcompany.com
APP_URL=https://short.yourcompany.com
TRAEFIK_ACME_EMAIL=ops@yourcompany.com
```

Then start everything:

```bash
docker compose up -d
```

The first run builds both images, which takes a few minutes. Watch the
certificate get issued:

```bash
docker compose logs -f traefik
```

Once `docker compose ps` shows every service healthy, open
`https://short.yourcompany.com`.

> **Test with the Let's Encrypt staging CA first.** Set `ACME_CA_SERVER` in
> `.env` to `https://acme-staging-v02.api.letsencrypt.org/directory`. Staging
> issues untrusted certificates but has far higher rate limits, so a
> misconfigured DNS record costs you nothing. The production CA allows only
> **5 duplicate certificates per week**, and hitting that locks you out until
> the window rolls. Switch back and run
> `docker compose restart traefik` when the stack is working.

## How TLS works

Traefik terminates TLS and obtains certificates from Let's Encrypt over the
HTTP-01 challenge. Certificates live in the `traefik_acme` volume as
`acme.json`, and renewal is automatic roughly 30 days before expiry.

Requests reach the right place by hostname and path:

```
https://short.yourcompany.com/api/...   →  Go server   (priority 100)
https://short.yourcompany.com/...       →  dashboard   (priority 1)
http://...                              →  301 to https
```

The API router carries the higher priority so `/api` can never fall through to
the dashboard's SPA fallback.

Postgres and Redis sit on a Docker network declared `internal: true`. They
publish no host ports and have no route to or from the internet — the Go server
is the only thing that can reach them.

## Custom domains

Adding a redirect domain such as `go.yourcompany.com` must not require editing
`docker-compose.yml` or restarting a container. Traefik watches
`docker/traefik/dynamic/` and reloads on change, so a verified domain becomes a
file in that directory.

Add the hostname in **Domains**, publish the displayed ownership and routing
DNS records, then press **Verify**. ShortURL writes the Traefik router after
both checks pass; no Compose edit or restart is required.

Certificates are deliberately requested only for verified domains. A catch-all
router would make Traefik ask Let's Encrypt for a certificate for every hostname
anyone points at your server, which burns the rate limit and can block your real
domains for a week.

### Cloudflare in front of ShortURL

Prefer Cloudflare **Full (strict)** mode. Traffic is encrypted on both hops and
Cloudflare validates the origin certificate issued by Let's Encrypt. Keep the
DNS record proxied and leave ShortURL's HTTPS configuration enabled.

Cloudflare **Flexible** mode sends HTTP from Cloudflare to the origin and does
not require an origin certificate, but the origin hop is unencrypted and an
origin HTTP-to-HTTPS redirect can cause `ERR_TOO_MANY_REDIRECTS`. The bundled
standalone Traefik stack redirects HTTP globally, so Flexible mode is not a
supported per-domain switch. See Cloudflare's official
[Full (strict)](https://developers.cloudflare.com/ssl/origin-configuration/ssl-modes/full-strict/),
[Flexible](https://developers.cloudflare.com/ssl/origin-configuration/ssl-modes/flexible/),
and [redirect-loop](https://developers.cloudflare.com/ssl/troubleshooting/too-many-redirects/)
documentation.

When Dokploy is the reverse proxy, configure HTTPS and certificates in
Dokploy's Domains screen. Domain settings inside ShortURL cannot mutate
Dokploy's proxy configuration.

## Backups

```bash
./scripts/backup.sh                 # writes backups/shorturl-<timestamp>.tar.gz
```

The archive holds three things:

| Item           | Why                                                    |
| -------------- | ------------------------------------------------------ |
| `.env`         | Configuration and every secret                          |
| `postgres.sql` | Full logical dump: users, workspaces, links, analytics  |
| `acme.json`    | Issued certificates, so a move does not re-issue them   |

Redis is **not** backed up on purpose. It holds the link cache, which rebuilds
itself from Postgres on the next request, and the click-event stream, which the
analytics worker drains continuously. Restoring a stale Redis would replay old
events; starting empty loses nothing that matters.

The archive contains your secrets and TLS private keys in plaintext. It is
written mode 600 — keep it that way, encrypt it at rest, and move it over a
channel you trust.

A nightly cron entry:

```cron
0 3 * * * cd /opt/shorturl && ./scripts/backup.sh >> /var/log/shorturl-backup.log 2>&1
```

`backup.sh` does not prune old archives. Add a retention step that suits your
storage, for example `find backups -name '*.tar.gz' -mtime +30 -delete`.

## Moving to another server

The whole deployment is one archive plus one Git checkout.

**On the old server:**

```bash
./scripts/backup.sh
```

**Copy the archive across:**

```bash
scp backups/shorturl-<timestamp>.tar.gz newserver:/opt/shorturl/
```

**On the new server:**

```bash
git clone <repository> /opt/shorturl
cd /opt/shorturl
./scripts/restore.sh shorturl-<timestamp>.tar.gz
```

`restore.sh` asks for confirmation, saves any existing `.env` as
`.env.before-restore-<timestamp>`, replays the database dump, reinstalls the
certificates, and brings the stack up.

**Then repoint DNS** at the new server's IP address.

Order matters for a near-zero-downtime move:

1. Lower the DNS TTL to 60 seconds, at least a day ahead.
2. Back up and restore onto the new server while DNS still points at the old one.
   Carrying `acme.json` across means the new host already holds valid
   certificates and does not need to pass an ACME challenge to serve traffic.
3. Repoint DNS.
4. Keep the old server running until traffic stops arriving, then take a final
   backup from it and restore again to pick up anything written in between.
5. Shut the old server down and restore the TTL.

Step 4 exists because any link created after the first backup lives only on the
old server. If you can accept a short outage instead, stop the old stack with
`docker compose down` before the final backup and skip the second restore.

## Upgrades

```bash
git pull
docker compose build
docker compose up -d
```

Compose recreates only the containers whose configuration or image changed.
Volumes survive. Take a backup first.

To deploy prebuilt images instead of building on the server — worth it on a
small VPS, where the Nuxt build is the heaviest thing that will ever run — push
them from CI and set the registry paths in `.env`:

```env
SERVER_IMAGE=ghcr.io/you/shorturl-server:1.4.0
WEB_IMAGE=ghcr.io/you/shorturl-web:1.4.0
```

Then `docker compose pull && docker compose up -d` skips building entirely.

## Operations reference

```bash
docker compose ps                    # service status and health
docker compose logs -f server        # structured JSON logs from the API
docker compose logs -f traefik       # certificate issuance and routing
docker compose restart server        # restart one service
docker compose down                  # stop everything, keep the data
docker compose down -v               # stop everything and DELETE ALL DATA
```

The Makefile wraps the common ones: `make prod-up`, `make prod-logs`,
`make prod-ps`, `make backup`, `make restore ARCHIVE=...`.

Open a database shell:

```bash
docker compose exec postgres psql -U shorturl -d shorturl
```

## Troubleshooting

**Certificate never issues.** Confirm the A record resolves to this server
(`dig +short short.yourcompany.com`), that port 80 reaches Traefik from the
public internet, and that no firewall or upstream proxy intercepts
`/.well-known/acme-challenge/`. Then read `docker compose logs traefik` — the
ACME error message names the cause.

**`Bind for 0.0.0.0:80 failed: port is already allocated`.** Another web server
owns port 80. Stop it, or put ShortURL behind it — but the second option means
that proxy has to handle TLS, and the ACME setup here no longer applies.

**A service is stuck unhealthy.** `docker compose ps` shows which one.
`docker compose logs <service>` shows why. The server waits for Postgres and
Redis to report healthy before it starts, so a failure there cascades visibly.

**`set REDIS_PASSWORD in .env`, or a similar startup error.** A required
variable is missing. Run `./scripts/install.sh` to fill in the generated ones.
Compose refuses to start rather than fall back to an insecure default.

**Dashboard loads but the API returns 404.** Confirm that `/api/*` is routed to
the `server` service. In the bundled stack Traefik handles it; in Dokploy the
web container proxies `/api/*` internally. `/health` should return
`{"status":"ok"}` from the public web service.
