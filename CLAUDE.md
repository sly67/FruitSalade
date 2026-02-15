# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

FruitSalade is a self-hosted, Docker-deployable file synchronization system with **on-demand file placeholders** — similar to OneDrive/Dropbox "Files On-Demand" but fully self-hosted. Full specification in `specs.md`.

## Project Structure

```
FruitSalade/
├── shared/                 # Shared code (reusable across clients)
│   └── pkg/
│       ├── cache/          # LRU file cache with pinning
│       ├── client/         # HTTP client (retry, offline, auth, upload)
│       ├── fuse/           # FUSE filesystem (read + write ops)
│       ├── logger/         # Simple logger
│       ├── models/         # Data types (FileNode, CacheEntry)
│       ├── protocol/       # API request/response types
│       ├── retry/          # Retry with backoff
│       └── tree/           # Tree utilities (FindByPath, CacheID, CountNodes)
│
├── fruitsalade/            # Main application
│   ├── cmd/
│   │   ├── server/         # HTTP server
│   │   ├── fuse-client/    # Linux FUSE client
│   │   ├── seed-tool/      # Database/storage seeder
│   │   └── windows-client/ # Windows CfAPI client
│   ├── internal/
│   │   ├── api/            # HTTP handlers, middleware, admin API
│   │   ├── auth/           # JWT + OIDC authentication
│   │   ├── config/         # Environment-based configuration
│   │   ├── events/         # SSE broadcaster
│   │   ├── logging/        # Structured logging (zap)
│   │   ├── metadata/postgres/ # PostgreSQL metadata store
│   │   ├── metrics/        # Prometheus metrics
│   │   ├── quota/          # Rate limiting and user quotas
│   │   ├── sharing/        # Permissions, share links, groups
│   │   ├── storage/        # Multi-backend storage router
│   │   │   ├── s3/         # S3/MinIO backend
│   │   │   ├── local/      # Local filesystem backend
│   │   │   └── smb/        # SMB/CIFS backend
│   │   ├── webdav/         # WebDAV handler
│   │   └── winclient/      # Windows client core
│   ├── migrations/         # PostgreSQL migrations (001-007)
│   ├── docker/             # Dockerfiles and compose files
│   ├── deploy/             # Systemd units, Grafana dashboard
│   ├── ui/                 # Admin UI (go:embed)
│   ├── webapp/             # Webapp file browser (go:embed)
│   ├── windows/            # C++ CfAPI shim
│   └── testdata/           # Seed data for Docker environments
│
├── go.work                 # Go workspace (links shared + fruitsalade)
└── Makefile                # Build targets
```

## Build Commands

```bash
# Build
make server          # Build server binary
make fuse            # Build FUSE client binary
make seed            # Build seed tool
make winclient       # Build Windows client (native, cgofuse)
make windows         # Cross-compile Windows client (requires CGO)

# Test
make test            # Run all tests
make test-shared     # Run shared package tests
make test-app        # Run app tests

# Docker
make docker          # Build server + client Docker images
make docker-up       # Start full env (server + minio + 2 clients)
make docker-down     # Stop env + remove volumes
make docker-logs     # Follow logs
make docker-run      # Run server standalone (local storage, no S3)
make exec-server     # Shell into server
make exec-a          # Shell into client-a
make exec-b          # Shell into client-b

# Utilities
make lint            # Lint all code
make fmt             # Format all code
make clean           # Remove build artifacts
make deps            # Download dependencies
```

## Technology Stack

| Component | Language | Key Libraries |
|-----------|----------|---------------|
| Server | Go | net/http, sqlx, aws-sdk-go-v2 |
| Linux Client | Go | hanwen/go-fuse v2 |
| Windows Client | Go + C++ | Go core + CfAPI shim via CGO |

## Critical Implementation Rules

### Filesystem Operations (FUSE)
- **NEVER** trigger content downloads on `Getattr()`, `Readdir()`, `Lookup()`
- **ONLY** fetch content on `Open()` or `Read()` calls
- Cache metadata aggressively; cache content only after explicit access
- All downloads must be atomic: write to temp file, then `os.Rename()`

### Server API
- Metadata endpoints return JSON (file tree, sizes, mtimes, hashes)
- Content endpoints support HTTP `Range` header for partial reads
- These are separate API paths — never bundle content in metadata responses

```
GET /api/v1/tree           → metadata only
GET /api/v1/tree/{path}    → subtree metadata
GET /api/v1/content/{id}   → content with Range support
GET /health                → health check
```

### Cache Manager
- Implement LRU eviction with configurable max size
- Support pinning ("Always keep on this device")
- Track file access times for eviction ordering
- Atomic writes: temp file → rename

## Key Interfaces

```go
// Storage backend - fruitsalade/internal/storage/backend.go
type Backend interface {
    PutObject(ctx context.Context, key string, r io.Reader, size int64) error
    GetObject(ctx context.Context, key string) (io.ReadCloser, error)
    GetObjectRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error)
    DeleteObject(ctx context.Context, key string) error
    StatObject(ctx context.Context, key string) (int64, error)
}

// Cache (client) - shared/pkg/cache/cache.go
type Cache interface {
    Get(fileID string) (path string, ok bool)
    Put(fileID string, r io.Reader, size int64) (path string, error)
    Evict(fileID string) error
    Pin(fileID string) error
    Unpin(fileID string) error
}
```

## Known Pitfalls

- Do NOT use WebDAV mounts (poor performance, no placeholders)
- Do NOT conflate metadata sync with content transfer
- Do NOT fetch content in `Getattr` — breaks `ls`, `find`, `du`
- Do NOT assume offline access works — fail gracefully
- Sub-packages (s3, local, smb) CANNOT import parent `storage` pkg (import cycle)
- FUSE Create: Set dirty=false initially; shell redirections trigger early Flush before Write
- UploadFile closes readers: Go HTTP client closes io.ReadCloser bodies. Use io.NewSectionReader
- Metadata tree sync: Write ops must update both n.metadata AND FruitFS.metadata tree

## Reference Code

Study for design patterns:
- **rclone VFS** (`github.com/rclone/rclone/vfs`) — Go VFS with lazy reads, LRU cache
- **go-fuse examples** (`github.com/hanwen/go-fuse/example`) — FUSE patterns
- **Windows CfAPI SDK samples** — sync root, hydration callbacks
