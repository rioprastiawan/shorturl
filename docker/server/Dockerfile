# syntax=docker/dockerfile:1
#
# Go server image. Build context is the repository root:
#   docker build -f docker/server/Dockerfile .

FROM golang:1.26-alpine AS build

WORKDIR /src

# Dependencies first so source edits do not invalidate the module cache.
COPY apps/server/go.mod apps/server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY apps/server/ ./

# An empty directory copied with the right owner below, so the named volume
# Docker initialises from it is writable by the non-root runtime user.
RUN mkdir -p /out/traefik-dynamic

ARG VERSION=dev
# CGO off produces a static binary that runs on distroless/static.
# timetzdata embeds the zone database, which the scratch-like base lacks.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -tags timetzdata \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/shorturl ./cmd/server


FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/shorturl /app/shorturl

# Where the server writes Traefik routers for verified custom domains. Docker
# seeds a fresh named volume from this path, inheriting its ownership, which is
# how a distroless container with no shell ends up able to write to a volume.
COPY --from=build --chown=nonroot:nonroot /out/traefik-dynamic /etc/shorturl/traefik-dynamic

# distroless :nonroot runs as uid 65532 and ships no shell, package manager,
# or writable filesystem paths, so the container has almost no attack surface.
USER nonroot:nonroot
WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["/app/shorturl"]
CMD ["serve"]
