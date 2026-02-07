# FruitSalade

A self-hosted, Docker-deployable file synchronization system with **on-demand file placeholders** -- similar to OneDrive/Dropbox "Files On-Demand" but fully self-hosted.

Files appear instantly in the filesystem via FUSE, but content is fetched from the server only when accessed. A local LRU cache keeps recently opened files for offline use.

## Features

- **Placeholder files** -- `ls` shows all files instantly (metadata only), `cat` triggers download
- **LRU cache** -- recently accessed files are cached locally with configurable size limits
- **Offline mode** -- cached files remain accessible when the server is unreachable
- **Server-Sent Events** -- real-time metadata updates pushed to connected clients
- **Write support** -- create, modify, rename, and delete files from the FUSE mount
- **File versioning** -- version history with rollback to any previous version
- **Conflict detection** -- optimistic locking via `X-Expected-Version` / `If-Match` headers
- **JWT authentication** -- per-device tokens with revocation support
- **Observability** -- Prometheus metrics + structured JSON logging (zap)
- **S3 backend** -- content stored in S3/MinIO, metadata in PostgreSQL
- **Docker-ready** -- full test environment with compose (server, 2 FUSE clients, PostgreSQL, MinIO)

## Architecture

```
                    FUSE Clients
                  ┌──────────────┐
                  │  client-a    │──┐
                  │  /mnt/fruit  │  │  HTTP + JWT
                  └──────────────┘  │
                  ┌──────────────┐  │  ┌─────────────────────────┐
                  │  client-b    │──┴─>│      API Server         │
                  │  /mnt/fruit  │     │      :8080               │
                  └──────────────┘     │                         │
                                       │  GET  /api/v1/tree      │
                                       │  GET  /api/v1/content/* │
                                       │  POST /api/v1/content/* │
                                       │  GET  /api/v1/versions/*│
                                       └────────┬───────┬────────┘
                                                 │       │
                                       ┌─────────┘       └─────────┐
                                       v                           v
                                 ┌───────────┐             ┌─────────────┐
                                 │ PostgreSQL│             │  S3 / MinIO │
                                 │ metadata  │             │  content    │
                                 │ auth      │             │  versions   │
                                 └───────────┘             └─────────────┘
```

## Quick Start

### Docker Test Environment (recommended)

```bash
make phase1-test-env
```

This starts PostgreSQL, MinIO, seeds test data, launches the server, and mounts two independent FUSE clients.

```bash
# Verify both clients see the same files
docker compose -f phase1/docker/docker-compose.yml exec client-a ls /mnt/fruitsalade
docker compose -f phase1/docker/docker-compose.yml exec client-b cat /mnt/fruitsalade/hello.txt

# Shell into a client
make phase1-exec-a

# Tear down
make phase1-test-env-down
```

### Local Development

```bash
# Build everything
make phase2

# Run server (requires PostgreSQL + MinIO)
DATABASE_URL="postgres://user:pass@localhost/fruitsalade?sslmode=disable" \
JWT_SECRET="dev-secret" \
./bin/phase2-server

# In another terminal, mount the FUSE client
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r .token)

./bin/phase2-fuse -mount /tmp/fruitsalade -server http://localhost:8080 -token "$TOKEN"
```

## Project Structure

```
fruitsalade/
├── shared/                 # Shared code across all phases
│   └── pkg/
│       ├── cache/          # LRU file cache with pinning
│       ├── client/         # HTTP client (retry, offline, auth, upload)
│       ├── fuse/           # FUSE filesystem (read + write ops)
│       ├── logger/         # Simple logger
│       ├── models/         # Data types (FileNode, CacheEntry)
│       ├── protocol/       # API request/response types
│       └── retry/          # Retry with backoff
│
├── phase0/                 # Proof of Concept (local filesystem backend)
├── phase1/                 # MVP (PostgreSQL + S3 + JWT + Docker)
├── phase2/                 # Production (metrics, logging, versioning)
│
├── go.work                 # Go workspace
└── Makefile                # Build targets
```

## API Reference

### Authentication

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/auth/token` | POST | Login with `{username, password, device_name}`, returns JWT |

Default credentials: `admin` / `admin`

### Metadata

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/api/v1/tree` | GET | Full metadata tree (supports gzip) |
| `/api/v1/tree/{path}` | GET | Subtree at path |

### Content

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/content/{path}` | GET | Download file (supports `Range` header) |
| `/api/v1/content/{path}` | POST | Upload file content |

Content responses include `ETag` (SHA256 hash) and `X-Version` headers.

### Directories

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/tree/{path}?type=dir` | PUT | Create directory |
| `/api/v1/tree/{path}` | DELETE | Delete file or directory (recursive) |

### Versioning

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/versions/{path}` | GET | List version history |
| `/api/v1/versions/{path}?v=N` | GET | Download version N content |
| `/api/v1/versions/{path}` | POST | Rollback to version `{"version": N}` |

### Conflict Detection

Upload requests can include concurrency control headers:

- **`X-Expected-Version: N`** -- reject with 409 if current version != N
- **`If-Match: "hash"`** -- reject with 409 if current content hash doesn't match

Without these headers, the default behavior is last-write-wins.

## FUSE Operations

The FUSE client supports full read-write access:

| Operation | Description |
|-----------|-------------|
| `ls` | Lists files from cached metadata (no content download) |
| `cat`, `cp` | Downloads content on first access, serves from cache after |
| `echo > file` | Creates file locally, uploads to server on close |
| `mkdir` | Creates directory on server immediately |
| `rm` | Deletes file from server and evicts from cache |
| `rmdir` | Removes empty directory from server |
| `mv` | Renames via re-upload + delete (no server-side rename API) |

**Key design rule**: `ls`, `stat`, `find`, and `du` never trigger content downloads.

## Build Targets

```bash
make phase0              # Build Phase 0 (PoC)
make phase1              # Build Phase 1 (MVP)
make phase2              # Build Phase 2 (Production)

make phase1-test-env     # Full Docker test environment
make phase1-test-env-down # Stop and remove volumes
make phase1-exec-a       # Shell into client-a container
make phase1-exec-b       # Shell into client-b container

make test                # Run all tests
make fmt                 # Format code
make lint                # Lint code
make clean               # Remove build artifacts
```

## Configuration

### Server Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `METRICS_ADDR` | `:9090` | Prometheus metrics address |
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `JWT_SECRET` | (required) | JWT signing secret |
| `S3_ENDPOINT` | `http://localhost:9000` | S3/MinIO endpoint |
| `S3_BUCKET` | `fruitsalade` | S3 bucket name |
| `S3_ACCESS_KEY` | `minioadmin` | S3 access key |
| `S3_SECRET_KEY` | `minioadmin` | S3 secret key |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `json` | Log format (json, console) |
| `MAX_UPLOAD_SIZE` | `104857600` | Max upload size in bytes (100MB) |

### FUSE Client Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-mount` | (required) | Mount point path |
| `-server` | `http://localhost:8080` | Server URL |
| `-cache` | `/tmp/fruitsalade-cache` | Cache directory |
| `-max-cache` | `1073741824` | Max cache size in bytes (1GB) |
| `-token` | (required) | JWT token (or `FRUITSALADE_TOKEN` env) |
| `-refresh` | `30s` | Metadata refresh interval |
| `-watch` | `false` | Enable SSE for real-time updates |
| `-health-check` | `30s` | Health check interval |
| `-verify-hash` | `false` | Verify SHA256 on download |

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Server | Go, net/http, PostgreSQL, S3/MinIO |
| Linux Client | Go, go-fuse v2 (FUSE3) |
| Metrics | Prometheus client_golang |
| Logging | Uber zap |
| Auth | JWT (golang-jwt/jwt/v5), bcrypt |
| Container | Docker, Docker Compose |

## Phase Overview

| Phase | Focus | Status |
|-------|-------|--------|
| **Phase 0** | Proof of Concept -- local filesystem backend, basic FUSE | Complete |
| **Phase 1** | MVP -- PostgreSQL, S3, JWT auth, Docker test env | Complete |
| **Phase 2** | Production -- metrics, logging, write ops, versioning, conflict detection | Complete |
| **Phase 3** | Multi-platform -- Windows CfAPI client, Admin UI | Planned |
