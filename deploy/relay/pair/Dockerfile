# syntax=docker/dockerfile:1
#
# Image of the meeting point service (concept 27 section 7, relay/cmd/pair): one static binary on
# an empty base, no shell, not root. It holds no secret and no code that handles one (E-21, D-6).
# Build context is the repository root:
#   docker buildx build -f deploy/relay/pair/Dockerfile .
# The binary is cross-compiled for TARGETARCH on the build platform, so amd64 and arm64 build
# without emulation.

# golang:1.27-trixie, pinned by the digest of its image index. Raise tag and digest together:
#   docker buildx imagetools inspect golang:<version>-trixie
ARG GO_IMAGE=golang:1.27-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
# No defaults: BuildKit fills them from --platform.
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ENV GOTOOLCHAIN=local \
    GOFLAGS=-mod=readonly \
    CGO_ENABLED=0
WORKDIR /src/relay
COPY relay/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    test -n "${TARGETARCH}" \
 && GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/pair ./cmd/pair

FROM scratch
COPY LICENSE /licenses/LICENSE
COPY --from=build /out/pair /pair

ARG VERSION=dev
LABEL org.opencontainers.image.title="Slouchcat relay: meeting point" \
      org.opencontainers.image.description="Meeting point service of Slouchcat: a phone sets up a TV by scanning its QR code" \
      org.opencontainers.image.source="https://github.com/slouchcat/slouchcat" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.version="${VERSION}"

USER 65532:65532
ENV RELAY_LISTEN=:8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/pair", "-healthcheck"]
ENTRYPOINT ["/pair"]
