# syntax=docker/dockerfile:1
#
# Slouchcat container image (plan section 3.7, ADR 0001 "Auslieferung").
# Build context is the repository root:
#   docker buildx build -f deploy/Dockerfile .
# The Go binary is cross-compiled for TARGETARCH on the build platform; only the
# apt stage of the final image runs on the target platform.

ARG NODE_IMAGE=node:24-trixie-slim
ARG GO_IMAGE=golang:1.27-trixie
ARG BASE_IMAGE=debian:trixie-slim

# ---------------------------------------------------------------------------
# Stage 1: web app (platform independent output, runs on the build platform)
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM ${NODE_IMAGE} AS web

ENV CI=true \
    COREPACK_ENABLE_DOWNLOAD_PROMPT=0 \
    PNPM_STORE_DIR=/pnpm/store
WORKDIR /src

# Manifests first so the dependency layer is cached independently of sources.
# corepack installs exactly the pnpm version pinned in package.json#packageManager.
# `.npmr[c]` and `tsconfig*.json` are optional root files (globs may match nothing).
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmr[c] tsconfig*.json ./
RUN corepack enable && corepack install && pnpm --version
RUN --mount=type=cache,id=slouchcat-pnpm,target=/pnpm/store \
    pnpm fetch --store-dir "$PNPM_STORE_DIR"

COPY api/ ./api/
COPY packages/ ./packages/
COPY web/ ./web/
RUN --mount=type=cache,id=slouchcat-pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile --offline --store-dir "$PNPM_STORE_DIR" \
 && pnpm --filter "@slouchcat/web..." run build \
 && test -f web/dist/index.html

# ---------------------------------------------------------------------------
# Stage 2: server binary (cross-compiled, CGO off)
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS server

# BuildKit fills TARGETOS/TARGETARCH from --platform; a default value here would win over it
# (an earlier `ARG TARGETARCH=amd64` built amd64 binaries for linux/arm64).
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ENV GOTOOLCHAIN=local \
    GOFLAGS=-mod=readonly
WORKDIR /src/server

COPY server/ ./
# The web app is embedded into the binary (server/internal/webassets).
RUN rm -rf internal/webassets/dist && mkdir -p internal/webassets/dist
COPY --from=web /src/web/dist/ ./internal/webassets/dist/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    test -n "${TARGETARCH}" \
 && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/slouchcat ./cmd/slouchcat

# ---------------------------------------------------------------------------
# Stage 3: runtime image
# ---------------------------------------------------------------------------
FROM ${BASE_IMAGE} AS runtime

# Debian codename used for the Jellyfin apt repository. Must match BASE_IMAGE
# (trixie for debian:trixie-slim, bookworm for the documented fallback).
ARG FFMPEG_SUITE=trixie
# jellyfin-ffmpeg7 is the 7.x line; jellyfin-ffmpeg8 also exists for trixie.
ARG FFMPEG_PACKAGE=jellyfin-ffmpeg7
# Upstream version without the "-<suite>" suffix; empty installs the newest one.
ARG FFMPEG_VERSION=7.1.4-3
# Set by BuildKit from --platform (amd64, arm64); no default, see the server stage.
ARG TARGETARCH

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# jellyfin-ffmpeg ships its own libva, Intel iHD/i965 VA drivers, oneVPL runtime
# (QSV) and Mesa radeonsi (AMD VAAPI) under /usr/lib/jellyfin-ffmpeg/lib, so no
# Debian VA driver packages are needed. NVIDIA libraries are injected at runtime
# by the NVIDIA Container Toolkit (see compose.nvidia.yaml).
RUN apt-get update \
 && apt-get install --yes --no-install-recommends --no-install-suggests \
      ca-certificates curl gnupg \
 && install -d -m 0755 /etc/apt/keyrings \
 && curl -fsSL https://repo.jellyfin.org/jellyfin_team.gpg.key \
      | gpg --dearmor -o /etc/apt/keyrings/jellyfin.gpg \
 && printf 'Types: deb\nURIs: https://repo.jellyfin.org/debian\nSuites: %s\nComponents: main\nArchitectures: %s\nSigned-By: /etc/apt/keyrings/jellyfin.gpg\n' \
      "${FFMPEG_SUITE}" "${TARGETARCH}" > /etc/apt/sources.list.d/jellyfin.sources \
 && apt-get update \
 && apt-get install --yes --no-install-recommends --no-install-suggests \
      "${FFMPEG_PACKAGE}${FFMPEG_VERSION:+=${FFMPEG_VERSION}-${FFMPEG_SUITE}}" \
      tzdata \
 && apt-get purge --yes --auto-remove curl gnupg \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* /var/log/apt /var/log/dpkg.log \
 && ln -s /usr/lib/jellyfin-ffmpeg/ffmpeg /usr/local/bin/ffmpeg \
 && ln -s /usr/lib/jellyfin-ffmpeg/ffprobe /usr/local/bin/ffprobe \
 && ffmpeg -hide_banner -version | sed -n 1p

# License folder (plan section 9, "GPL-Pflichten"): AGPL for Slouchcat, GPL notice
# and written source offer for ffmpeg, plus the exact jellyfin-ffmpeg version.
COPY LICENSE /licenses/LICENSE
COPY deploy/licenses/FFMPEG-NOTICE.md /licenses/FFMPEG-NOTICE.md
RUN dpkg-query -W -f='${Package} ${Version}\n' "${FFMPEG_PACKAGE}" > /licenses/FFMPEG-VERSION \
 && if [ -f "/usr/share/doc/${FFMPEG_PACKAGE}/copyright" ]; then \
      cp "/usr/share/doc/${FFMPEG_PACKAGE}/copyright" /licenses/FFMPEG-COPYRIGHT; \
    fi

RUN groupadd --gid 1000 slouchcat \
 && useradd --uid 1000 --gid 1000 --no-create-home --home-dir /config \
      --shell /usr/sbin/nologin slouchcat \
 && install -d -o 1000 -g 1000 -m 0750 /config /cache

COPY --from=server /out/slouchcat /usr/local/bin/slouchcat

ENV SLOUCHCAT_CONFIG_DIR=/config \
    SLOUCHCAT_CACHE_DIR=/cache \
    SLOUCHCAT_LOG_FORMAT=json \
    SLOUCHCAT_LOG_LEVEL=info \
    NVIDIA_VISIBLE_DEVICES=all \
    NVIDIA_DRIVER_CAPABILITIES=compute,video,utility

# Declared this late on purpose: a build argument invalidates the cache of every RUN after it, and
# VERSION changes with each commit while the apt layer above does not.
ARG VERSION=dev
LABEL org.opencontainers.image.title="Slouchcat" \
      org.opencontainers.image.description="Self-hosted media server" \
      org.opencontainers.image.source="https://github.com/slouchcat/slouchcat" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.version="${VERSION}"

USER 1000:1000
VOLUME ["/config", "/cache"]
EXPOSE 8096

HEALTHCHECK --interval=5s --timeout=2s --start-period=1s --start-interval=1s --retries=3 \
  CMD ["/usr/local/bin/slouchcat", "healthcheck"]

ENTRYPOINT ["/usr/local/bin/slouchcat"]
CMD ["serve"]
