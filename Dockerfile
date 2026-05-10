# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal

ARG TARGETOS=linux
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    if [ -n "$TARGETARCH" ]; then export GOARCH="$TARGETARCH"; fi; \
    CGO_ENABLED=0 GOOS="$TARGETOS" go build -trimpath -ldflags="-s -w" -o /out/mini-notes ./cmd/app

RUN mkdir -p /out/data

FROM scratch

COPY --from=builder --chown=65532:65532 /out/data /data
COPY --from=builder /out/mini-notes /usr/local/bin/mini-notes

WORKDIR /data
ENV NOTES_DB_PATH=/data/notes.db

EXPOSE 8800
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/mini-notes"]
