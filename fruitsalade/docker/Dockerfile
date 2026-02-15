# FruitSalade Docker
#
# Two targets:
#   server  - All-in-one: PostgreSQL 16 + API server + seed tool
#   client  - Standalone FUSE client (for test/remote machines)
#
# Build:
#   docker build -t fruitsalade:server --target server -f fruitsalade/docker/Dockerfile .
#   docker build -t fruitsalade:client --target client -f fruitsalade/docker/Dockerfile .

# =============================================================================
# Builder stage
# =============================================================================
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# Copy workspace and source
COPY go.work ./
COPY shared/ ./shared/
COPY fruitsalade/ ./fruitsalade/

# Build binaries
WORKDIR /build/fruitsalade
ENV GOWORK=off
RUN go build -o /bin/server ./cmd/server
RUN go build -o /bin/seed-tool ./cmd/seed-tool
RUN go build -o /bin/fuse-client ./cmd/fuse-client  # client target only

# =============================================================================
# Server target: PostgreSQL + API server + seed tool
# =============================================================================
FROM alpine:3.19 AS server

RUN apk add --no-cache \
    postgresql16 \
    postgresql16-contrib \
    su-exec \
    ca-certificates \
    tzdata \
    curl

WORKDIR /app

# Copy server binaries
COPY --from=builder /bin/server /app/server
COPY --from=builder /bin/seed-tool /app/seed-tool

# Migrations and seed data
COPY fruitsalade/migrations/ /app/migrations/
COPY fruitsalade/testdata/ /app/testdata/

# Prepare directories
RUN mkdir -p /data/postgres /data/storage /run/postgresql && \
    chown -R postgres:postgres /data/postgres /run/postgresql

# Entrypoint script
COPY fruitsalade/docker/entrypoint-single.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

EXPOSE 8080 9090
VOLUME ["/data/postgres", "/data/storage"]

HEALTHCHECK --interval=5s --timeout=3s --start-period=15s --retries=5 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]

# =============================================================================
# Client target: standalone FUSE client
# =============================================================================
FROM alpine:3.19 AS client

RUN apk --no-cache add ca-certificates tzdata fuse3 curl

WORKDIR /app

COPY --from=builder /bin/fuse-client /app/fuse-client
COPY fruitsalade/docker/client-entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

RUN mkdir -p /mnt/fruitsalade /var/cache/fruitsalade

ENTRYPOINT ["/app/entrypoint.sh"]
