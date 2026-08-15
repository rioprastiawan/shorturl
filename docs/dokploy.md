# Deploy with Dokploy

The repository publishes the `server` and `web` images to GHCR after every
push to `main` and for tags matching `v*`. The Dokploy stack uses
`docker-compose.dokploy.yml`; it deliberately does not start another Traefik
because Dokploy already owns the host proxy and TLS termination.

## 1. Publish the images

Push to `main`, then confirm that the **Publish container images** workflow
completed. Make both GHCR packages public, or configure GHCR registry
credentials in Dokploy when the repository is private.

## 2. Create the Compose application

In Dokploy, create a Compose application from this repository and set the
Compose path to:

```text
docker-compose.dokploy.yml
```

Add these environment variables (replace the owner and secrets):

```dotenv
SERVER_IMAGE=ghcr.io/rioprastiawan/shorturl-server:latest
WEB_IMAGE=ghcr.io/rioprastiawan/shorturl-web:latest
APP_DOMAIN=short.example.com
POSTGRES_DB=shorturl
POSTGRES_USER=shorturl
POSTGRES_PASSWORD=generate-a-long-random-value
REDIS_PASSWORD=generate-a-long-random-value
SESSION_SECRET=generate-at-least-32-random-characters
IP_HASH_SECRET=generate-at-least-32-random-characters
```

Generate each secret locally with `openssl rand -hex 32`. Do not commit them.

## 3. Configure the domain

In Dokploy, attach `https://short.example.com` to service `web`, container port
`8080`, with path `/`. Enable HTTPS. The web container proxies `/api/*` to the
server over the private Compose network, so no public server port or second
domain is needed.

Deploy the application and open `/setup` to create the first administrator.

If the images use a mutable tag such as `latest`, add `pull_policy: always` to
the shared server image anchor and to the `web` service. A redeploy then pulls
the current GHCR digest instead of reusing the image cached on the host.

The in-app automatic custom-domain feature writes configuration for the
standalone Traefik stack and is therefore disabled in this Dokploy Compose.
To serve an additional redirect domain in Dokploy, attach that hostname to the
`server` service on port `8080` through Dokploy's Domains screen after the
domain has been verified in the application.

## Cloudflare

Dokploy owns TLS termination in this deployment. Prefer Cloudflare **Full
(strict)**, enable HTTPS for the hostname in Dokploy, and use a valid origin
certificate. If Cloudflare Flexible is unavoidable, disable HTTPS redirects at
the Dokploy proxy for that hostname and route Cloudflare to HTTP port 80;
ShortURL cannot change those proxy settings from its Domains page. Flexible
leaves the Cloudflare-to-origin hop unencrypted and can create redirect loops,
so it is not the recommended default.
