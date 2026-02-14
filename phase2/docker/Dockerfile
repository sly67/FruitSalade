# Phase 2 Multi-Target Dockerfile
# Builds: server, seed-tool, fuse-client
# Phase 2 adds: versioning migration, metrics endpoint, write-capable FUSE client

# =============================================================================
# Builder stage
# =============================================================================
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# Copy workspace and source (shared + phase1 seed tool + phase2 server/client)
COPY go.work ./
COPY shared/ ./shared/
COPY phase1/ ./phase1/
COPY phase2/ ./phase2/

# Build Phase 2 server, FUSE client, and seed tool
WORKDIR /build/phase2
ENV GOWORK=off
RUN go build -o /bin/server ./cmd/server
RUN go build -o /bin/fuse-client ./cmd/fuse-client
RUN go build -o /bin/seed-tool ./cmd/seed-tool

# =============================================================================
# Server target
# =============================================================================
FROM alpine:3.19 AS server

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/server /app/server
# Merge Phase 1 base migrations + Phase 2 migrations into one directory
COPY phase1/migrations/ /app/migrations/
COPY phase2/migrations/ /app/migrations/

EXPOSE 8080 9090

HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=5 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]

# =============================================================================
# Seed target (Phase 2 backend-agnostic seed tool)
# =============================================================================
FROM alpine:3.19 AS seed

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/seed-tool /app/seed-tool
COPY phase1/migrations/ /app/migrations/
COPY phase2/migrations/ /app/migrations/
COPY phase1/testdata/ /testdata/

ENTRYPOINT ["/app/seed-tool"]

# =============================================================================
# FUSE client target (Phase 2 - read/write)
# =============================================================================
FROM alpine:3.19 AS fuse-client

RUN apk --no-cache add ca-certificates tzdata fuse3 curl

WORKDIR /app

COPY --from=builder /bin/fuse-client /app/fuse-client
COPY phase2/docker/client-entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

RUN mkdir -p /mnt/fruitsalade /var/cache/fruitsalade

ENTRYPOINT ["/app/entrypoint.sh"]
