# syntax=docker/dockerfile:1

# The launcher image is the c9s binary plus the two pieces of a browser terminal the distroless
# manager image cannot provide itself: ttyd, which serves the pty over HTTP, and tmux, which keeps
# the session alive across browser reconnects. The c9s binary is what enters the device container's
# namespaces and becomes the requested shell, so it lives at the same path as in the manager image.
ARG BUILDPLATFORM

FROM --platform=${BUILDPLATFORM} golang:1.26-bookworm AS builder

WORKDIR /clabernetes

RUN mkdir build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    TARGET_OS="${TARGETOS:-linux}" && \
    TARGET_ARCH="${TARGETARCH:-$(go env GOARCH)}" && \
    CGO_ENABLED=0 \
    GOOS="${TARGET_OS}" \
    GOARCH="${TARGET_ARCH}" && \
    go build \
    -ldflags "-s -w -X github.com/clabernetes/clabernetes/constants.Version=${VERSION}" \
    -trimpath \
    -o \
    build/manager \
    cmd/clabernetes/main.go

FROM debian:bookworm-slim

LABEL org.opencontainers.image.source="https://github.com/maintainer64/cms-labs-clabernetes"

ARG TTYD_VERSION=1.7.7
ARG TARGETARCH

# ttyd ships one static binary per architecture under its own names; ttyd 1.7.7 is the terminal
# server release. tmux keeps the device's shell alive between browser sessions, which is what makes
# a terminal usable across a network hiccup.
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl tmux && \
    rm -rf /var/lib/apt/lists/* && \
    case "${TARGETARCH:-amd64}" in \
      amd64 | x86_64) TTYD_ARCH="x86_64" ;; \
      arm64 | aarch64) TTYD_ARCH="aarch64" ;; \
      *) echo "launcher image has no ttyd build for ${TARGETARCH}" >&2; exit 1 ;; \
    esac && \
    curl -fsSL -o /usr/bin/ttyd \
    "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.${TTYD_ARCH}" && \
    chmod 0755 /usr/bin/ttyd

WORKDIR /clabernetes
COPY --from=builder /clabernetes/build/manager /clabernetes/manager
COPY build/launcher/tmux.conf /etc/tmux.conf

ENTRYPOINT ["/clabernetes/manager", "run"]
