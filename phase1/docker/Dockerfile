# Phase 1 Multi-Target Dockerfile
# Builds: server, seed-tool, fuse-client

# =============================================================================
# Builder stage
# =============================================================================
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# Copy workspace and source
COPY go.work ./
COPY shared/ ./shared/
COPY phase1/ ./phase1/

# Build all three binaries with GOWORK=off (use module's own go.mod)
WORKDIR /build/phase1
ENV GOWORK=off
RUN go build -o /bin/server ./cmd/server
RUN go build -o /bin/seed-tool ./cmd/seed-tool
RUN go build -o /bin/fuse-client ./cmd/fuse-client

# =============================================================================
# Server target
# =============================================================================
FROM alpine:3.19 AS server

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/server /app/server
COPY phase1/migrations/ /app/migrations/

EXPOSE 8080

HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=5 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]

# =============================================================================
# Seed target
# =============================================================================
FROM alpine:3.19 AS seed

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/seed-tool /app/seed-tool
COPY phase1/migrations/ /app/migrations/
COPY phase1/testdata/ /testdata/

ENTRYPOINT ["/app/seed-tool"]

# =============================================================================
# FUSE client target
# =============================================================================
FROM alpine:3.19 AS fuse-client

RUN apk --no-cache add ca-certificates tzdata fuse3 curl

WORKDIR /app

COPY --from=builder /bin/fuse-client /app/fuse-client
COPY phase1/docker/client-entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Create default mount point and cache dir
RUN mkdir -p /mnt/fruitsalade /var/cache/fruitsalade

ENTRYPOINT ["/app/entrypoint.sh"]
