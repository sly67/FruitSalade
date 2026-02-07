# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

FruitSalade is a self-hosted, Docker-deployable file synchronization system with **on-demand file placeholders** — similar to OneDrive/Dropbox "Files On-Demand" but fully self-hosted. Full specification in `specs.md`.

## Project Structure (Phase-Based)

```
fruitsalade/
├── shared/                 # Shared code across all phases
│   └── pkg/
│       ├── models/         # Data types (FileNode, CacheEntry)
│       └── protocol/       # API request/response types
│
├── phase0/                 # PoC - Proof of Concept
│   ├── cmd/
│   │   ├── server/         # Minimal HTTP server
│   │   └── fuse-client/    # Minimal FUSE client
│   ├── internal/
│   │   ├── api/            # HTTP handlers
│   │   ├── storage/        # Local filesystem backend
│   │   ├── fuse/           # FUSE implementation
│   │   └── cache/          # Simple file cache
│   └── testdata/           # Test files
│
├── phase1/                 # MVP - Minimum Viable Product
│   ├── cmd/
│   │   ├── server/         # Production server
│   │   └── fuse-client/    # Production client
│   ├── internal/
│   │   ├── storage/s3/     # S3 backend
│   │   ├── metadata/postgres/
│   │   └── auth/           # JWT authentication
│   ├── migrations/         # Database migrations
│   └── docker/             # Dockerfile, docker-compose
│
├── phase2/                 # Production - Full Features
│   ├── cmd/
│   │   ├── server/
│   │   ├── fuse-client/
│   │   └── windows-client/ # Windows CfAPI client
│   ├── internal/
│   └── windows/            # C++ CfAPI shim
│
├── go.work                 # Go workspace (links all phases)
├── Makefile                # Build targets per phase
└── PLAN.md                 # Detailed task breakdown
```

## Build Commands

```bash
# Phase 0 (PoC)
make phase0              # Build server + FUSE client
make phase0-server       # Build server only
make phase0-fuse         # Build FUSE client only
make phase0-test         # Run tests
make phase0-run-server   # Run server locally

# Phase 1 (MVP)
make phase1              # Build all
make phase1-docker       # Build Docker image
make phase1-up           # Start Docker services (postgres, minio, server)
make phase1-down         # Stop Docker services

# Phase 2 (Production)
make phase2              # Build all
make phase2-windows      # Build Windows client (requires CGO)

# Utilities
make test                # Test all phases
make lint                # Lint all code
make fmt                 # Format all code
make clean               # Remove build artifacts
make help                # Show all targets
```

## Technology Stack

| Component | Language | Key Libraries |
|-----------|----------|---------------|
| Server | Go | net/http or Gin, sqlx, aws-sdk-go-v2 |
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
// Storage backend (server) - internal/storage/storage.go
type Storage interface {
    GetMetadata(ctx context.Context, path string) (*models.FileNode, error)
    GetContent(ctx context.Context, id string, offset, length int64) (io.ReadCloser, int64, error)
    ListDir(ctx context.Context, path string) ([]*models.FileNode, error)
}

// Cache (client) - internal/cache/cache.go
type Cache interface {
    Get(fileID string) (path string, ok bool)
    Put(fileID string, r io.Reader, size int64) (path string, error)
    Evict(fileID string) error
    Pin(fileID string) error
    Unpin(fileID string) error
}
```

## Development Workflow

1. **Start with Phase 0** — get PoC working first
2. **Test locally** — server serves files, FUSE client mounts and fetches
3. **Validate E2E** — `ls` shows files, `cat` fetches, cache works
4. **Move to Phase 1** — add PostgreSQL, S3, auth
5. **Phase 2** — Windows client, metrics, admin UI

## Known Pitfalls

- Do NOT use WebDAV mounts (poor performance, no placeholders)
- Do NOT conflate metadata sync with content transfer
- Do NOT fetch content in `Getattr` — breaks `ls`, `find`, `du`
- Do NOT assume offline access works — fail gracefully

## Reference Code

Study for design patterns:
- **rclone VFS** (`github.com/rclone/rclone/vfs`) — Go VFS with lazy reads, LRU cache
- **go-fuse examples** (`github.com/hanwen/go-fuse/example`) — FUSE patterns
- **Windows CfAPI SDK samples** — sync root, hydration callbacks
