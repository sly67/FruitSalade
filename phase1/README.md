# Phase 1: Minimum Viable Product (MVP)

## Aim

Deliver a production-ready, Docker-deployable file synchronization system with real storage backends, authentication, and multi-client support. This is the first version suitable for actual use.

## Features

### Server
- PostgreSQL metadata storage with migrations
- S3/MinIO object storage backend
- JWT authentication with token management
- Device tracking and token revocation
- Server-Sent Events (SSE) for real-time metadata updates
- Health check endpoint with proper status reporting

### FUSE Client (Linux)
- Full LRU cache with configurable size limits
- JWT token authentication
- Periodic metadata refresh (configurable interval)
- SSE subscription for real-time updates
- Health check with automatic reconnection
- Hash verification for downloaded files (optional)
- Pin/unpin for offline access control

### Docker Environment
- Multi-container setup: PostgreSQL, MinIO, Server, Clients
- Seed tool for populating test data
- Two independent client containers (simulating separate machines)
- Proper service dependencies and health checks

## Status: Complete

All Phase 1 MVP objectives achieved:
- [x] PostgreSQL metadata store with schema migrations
- [x] S3/MinIO content storage with range support
- [x] JWT authentication (login, token generation, revocation)
- [x] Default admin user creation on first run
- [x] Docker Compose environment with all services
- [x] Seed tool for test data population
- [x] Two-client test environment (client-a, client-b)
- [x] SSE for real-time metadata updates
- [x] Automatic health check and reconnection

## Architecture

```
┌─────────────┐     ┌─────────────┐
│  Client A   │     │  Client B   │
│ (FUSE Mount)│     │ (FUSE Mount)│
└──────┬──────┘     └──────┬──────┘
       │                   │
       └─────────┬─────────┘
                 │ HTTP/JWT
       ┌─────────▼─────────┐
       │      Server       │
       │   (Go + Gin)      │
       └────┬─────────┬────┘
            │         │
   ┌────────▼───┐ ┌───▼────────┐
   │ PostgreSQL │ │   MinIO    │
   │ (metadata) │ │ (content)  │
   └────────────┘ └────────────┘
```

## Build & Run

```bash
# Build all Phase 1 components
make phase1

# Start full Docker test environment
make phase1-test-env

# View logs
make phase1-test-env-logs

# Shell into clients
make phase1-exec-a
make phase1-exec-b

# Stop and clean up
make phase1-test-env-down
```

## API Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/health` | GET | No | Health check |
| `/api/v1/auth/token` | POST | No | Login, get JWT |
| `/api/v1/tree` | GET | Yes | Full metadata tree |
| `/api/v1/tree/{path}` | GET | Yes | Subtree metadata |
| `/api/v1/content/{id}` | GET | Yes | File content (Range supported) |
| `/api/v1/events` | GET | Yes | SSE metadata updates |

## Configuration

### Server (Environment Variables)
- `DATABASE_URL` - PostgreSQL connection string (required)
- `S3_ENDPOINT` - S3/MinIO endpoint URL
- `S3_BUCKET` - Bucket name
- `S3_ACCESS_KEY` / `S3_SECRET_KEY` - S3 credentials
- `JWT_SECRET` - Secret for signing tokens (required)
- `LISTEN_ADDR` - Server listen address (default `:8080`)

### Client (Flags)
- `-mount` - Mount point for FUSE (required)
- `-server` - Server URL
- `-token` - JWT token (or `FRUITSALADE_TOKEN` env var)
- `-cache` - Cache directory
- `-max-cache` - Maximum cache size in bytes
- `-refresh` - Metadata refresh interval
- `-watch` - Enable SSE subscription
- `-verify-hash` - Verify file hashes after download

## Limitations

- **Read-only**: No file creation, modification, or deletion from clients
- **No versioning**: Files are not version-tracked
- **No conflict detection**: Multiple clients cannot detect conflicts
- **Full tree refresh**: Metadata updates rebuild entire tree (not incremental)

These limitations are addressed in Phase 2.
