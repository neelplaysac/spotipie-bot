# syntax=docker/dockerfile:1

# ---------- build stage ----------
# Pinned to the *build* platform, so the Go toolchain runs natively on the CI
# runner. TARGETOS/TARGETARCH are supplied by BuildKit / `docker buildx
# --platform`; GOOS/GOARCH then cross-compile the binary for the target arch
# (linux/arm64) without QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Dependencies first: this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Then the source. Rebuilds only when code changes.
COPY . .

# Static, stripped, cross-compiled binary.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/spotipie-go ./cmd/spotipie

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build \
        -trimpath -ldflags="-s -w" \
        -o /out/spotipie-authserver ./cmd/authserver

# ---------- runtime stage ----------
# Distroless ships ca-certificates (HTTPS to Telegram / Spotify / Last.fm) and
# runs as nonroot. There is no RUN step here, so a linux/arm64 image builds
# without emulation. If you need a shell for debugging, swap this for
# `alpine:3.22` + `RUN apk add --no-cache ca-certificates tzdata` and add QEMU
# in CI (buildx would then emulate arm64 for that RUN).
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app

COPY --from=builder /out/spotipie-go /app/spotipie-go
COPY --from=builder /out/spotipie-authserver /app/spotipie-authserver
# The app loads fonts from the relative "fonts/" directory at runtime.
COPY fonts/ /app/fonts/

# distroless:nonroot
USER 65532:65532

# Only the authserver command listens on HTTP. The bot still uses long polling.
EXPOSE 6969
STOPSIGNAL SIGTERM

ENTRYPOINT ["/app/spotipie-go"]
