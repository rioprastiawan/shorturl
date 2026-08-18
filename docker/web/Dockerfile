# syntax=docker/dockerfile:1
#
# Nuxt dashboard image. Build context is the repository root:
#   docker build -f docker/web/Dockerfile .
#
# The dashboard is an SPA (ssr: false), so the build produces plain static
# files and the runtime image is nginx rather than a Node or Bun process.

# The generated SPA is architecture-independent, so build it natively once
# instead of running Bun under QEMU for the arm64 image.
FROM --platform=$BUILDPLATFORM oven/bun:1.3-alpine AS build

WORKDIR /src

# --ignore-scripts skips the `nuxt prepare` postinstall, which needs the full
# source tree that has not been copied yet. `nuxt generate` prepares anyway.
COPY apps/web/package.json apps/web/bun.lock ./
RUN bun install --frozen-lockfile --ignore-scripts

COPY apps/web/ ./

# Baked into the bundle at build time. The default is a same-origin relative
# path, so the image stays portable across deployments and hostnames.
ARG NUXT_PUBLIC_API_BASE_URL=/api/v1
ARG VERSION=dev
ENV NUXT_PUBLIC_API_BASE_URL=${NUXT_PUBLIC_API_BASE_URL}
ENV NUXT_PUBLIC_APP_VERSION=${VERSION}

RUN bun run generate


FROM nginxinc/nginx-unprivileged:1.29-alpine

# This image already runs as uid 101 and listens on 8080, so no root is needed
# at any point and no capabilities have to be granted.
COPY docker/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/.output/public /usr/share/nginx/html

EXPOSE 8080
