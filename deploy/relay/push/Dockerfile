# syntax=docker/dockerfile:1
#
# Image of the push forwarder (concept 27 section 7.3, relay/cmd/push): one static binary on an
# empty base, no shell, not root. Its two secrets -- the Firebase service account and the sealing
# key file -- are never part of the image; they are mounted as files at run time (D-6).
# Build context is the repository root:
#   docker buildx build -f deploy/relay/push/Dockerfile .
# The binary is cross-compiled for TARGETARCH on the build platform, so amd64 and arm64 build
# without emulation.

ARG GO_IMAGE=golang:1.27-trixie

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
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/push ./cmd/push

FROM scratch
# The forwarder talks HTTPS to Google's token endpoint and FCM.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY LICENSE /licenses/LICENSE
COPY --from=build /out/push /push

ARG VERSION=dev
LABEL org.opencontainers.image.title="Slouchcat relay: push forwarder" \
      org.opencontainers.image.description="Push forwarder of Slouchcat: Web Push from Slouchcat servers to the Android app through FCM" \
      org.opencontainers.image.source="https://github.com/slouchcat/slouchcat" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.version="${VERSION}"

USER 65532:65532
# The secrets are read from /run/secrets/sealing-keys.json and /run/secrets/firebase.json unless
# RELAY_PUSH_KEYFILE and RELAY_FCM_CREDENTIALS name other files.
ENV RELAY_LISTEN=:8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/push", "-healthcheck"]
ENTRYPOINT ["/push"]
