# Traefik dynamic configuration

Traefik watches this directory and reloads changes without a restart. It exists
so verified custom domains can be routed **without editing `docker-compose.yml`
and without restarting any container**, which is a hard requirement of the
design (see §7 and §21 of the implementation plan).

Files here are applied automatically. Anything invalid is logged by Traefik and
ignored, so a bad file cannot take the stack down.

## Status

Routing for the main application domain is configured with Docker labels in
`docker-compose.yml` and needs nothing here.

Per-domain routers for **custom domains** are written to this directory once
domain verification exists (Milestone 5). Until then, add them by hand using the
template below.

## Adding a custom domain by hand

Create `go.example.com.yml` in this directory:

```yaml
http:
  routers:
    custom-go-example-com:
      rule: "Host(`go.example.com`)"
      entryPoints:
        - websecure
      service: shorturl-server
      tls:
        certResolver: letsencrypt
```

`shorturl-server` is the service Traefik derives from the `server` container's
Docker labels, so no `loadBalancer` block is needed.

## Why certificates are requested per verified domain

Let's Encrypt applies rate limits per registered domain and per account. A
catch-all router with `certResolver` would make Traefik request a certificate
for **every** hostname that resolves to this server, including ones a stranger
points at you, which burns the limit and can lock out real domains for a week.

Requesting a certificate only after DNS verification succeeds keeps issuance
tied to domains the operator actually controls.
