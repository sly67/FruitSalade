# Phase 2: Production Features

## Aim

Extend the MVP into a full-featured production system with write support, observability, file versioning, and conflict detection.

## Implemented Features

### 2.1 Observability (Complete)
- [x] Prometheus metrics endpoint (`/metrics` on port 9090)
- [x] Structured JSON logging (zap)
- [x] Request logging with correlation IDs
- [x] HTTP request metrics (count, duration, status)
- [x] Content transfer metrics (bytes downloaded/uploaded)
- [x] Database query metrics
- [x] S3 operation metrics
- [x] Authentication metrics

### 2.2 Write Operations - Server Side (Complete)
- [x] File upload endpoint (`POST /api/v1/content/{path}`)
- [x] Directory creation (`PUT /api/v1/tree/{path}?type=dir`)
- [x] File/directory deletion (`DELETE /api/v1/tree/{path}`)
- [x] Recursive directory deletion
- [x] Automatic parent directory creation
- [x] Max upload size limit (configurable)

### 2.3 Write Operations - FUSE Client (Complete)
- [x] Create -- new file creation with temp file buffer
- [x] Write -- buffered writes to temp file
- [x] Flush -- upload to server on file close
- [x] Mkdir -- create directory on server
- [x] Unlink -- delete file from server + evict cache
- [x] Rmdir -- remove empty directory from server
- [x] Rename -- re-upload under new path + delete old
- [x] Setattr -- truncate and mtime changes
- [x] Open for write -- existing files opened with O_WRONLY/O_RDWR/O_TRUNC

### 2.4 File Versioning (Complete)
- [x] Automatic version history on file upload
- [x] Version backup to S3 (`_versions/{key}/{version}`)
- [x] List version history (`GET /api/v1/versions/{path}`)
- [x] Download specific version content (`GET /api/v1/versions/{path}?v=N`)
- [x] Rollback to previous version (`POST /api/v1/versions/{path}`)
- [x] Database migration for `file_versions` table and `version` column

### 2.5 Conflict Detection (Complete)
- [x] Optimistic concurrency via `X-Expected-Version` header
- [x] ETag-based conflict detection via `If-Match` header
- [x] 409 Conflict response with version details
- [x] `ETag` and `X-Version` headers on content downloads
- [x] Default last-write-wins (backward compatible)

### 2.6 SSE Real-Time Sync (Complete)
- [x] `events.Broadcaster` with subscribe/unsubscribe/publish
- [x] SSE endpoint (`GET /api/v1/events`)
- [x] Events published from upload, create, delete, and rollback handlers
- [x] FUSE client `--watch` flag for live metadata refresh

### 2.7 File Sharing (Complete)
- [x] ACL-based permissions (`file_permissions` table)
- [x] Path inheritance (permission on `/docs` applies to `/docs/sub/file.txt`)
- [x] Permission levels: read, write, owner (hierarchical)
- [x] Share links with optional password (bcrypt), expiry, and download limits
- [x] Public download endpoint (`GET /api/v1/share/{token}`)
- [x] Database migrations (`003_sharing`)

### 2.8 Rate Limiting & Quotas (Complete)
- [x] Per-user quotas: storage, bandwidth/day, requests/min, upload size
- [x] In-memory token bucket rate limiter
- [x] Daily bandwidth tracking (`bandwidth_usage` table)
- [x] Middleware chain: metrics -> logging -> mux -> auth -> rateLimiter -> handler
- [x] Database migrations (`004_quotas`)

### 2.9 Admin UI (Complete)
- [x] Vanilla HTML/CSS/JS embedded via `go:embed` (no build step)
- [x] Served at `/admin/` with hash-based SPA routing
- [x] Login with admin-only access check
- [x] Dashboard: user count, sessions, files, storage, share links
- [x] Users: list, create, delete, change password
- [x] Files: browse metadata tree with breadcrumb navigation
- [x] Share Links: list all with revoke action
- [x] Admin API endpoints (`/api/v1/admin/{users,sharelinks,stats}`)

### 2.10 CI Pipeline (Complete)
- [x] GitHub Actions workflow (lint, test, build, Docker)

### 2.11 Docker Environment (Complete)
- [x] Docker Compose with server, PostgreSQL, MinIO, 2 FUSE clients
- [x] Dockerfile with multi-stage build
- [x] Automated database seeding and migration

## Planned Features

### Windows Client (Not Started)
- [ ] C++ CfAPI shim
- [ ] CGO integration

## API Endpoints

### Read Operations
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/health` | GET | No | Health check |
| `/api/v1/auth/token` | POST | No | Login, get JWT |
| `/api/v1/tree` | GET | Yes | Full metadata tree |
| `/api/v1/tree/{path}` | GET | Yes | Subtree metadata |
| `/api/v1/content/{path}` | GET | Yes | File content (Range, ETag, X-Version) |

### Write Operations
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/content/{path}` | POST | Yes | Upload file (supports X-Expected-Version, If-Match) |
| `/api/v1/tree/{path}?type=dir` | PUT | Yes | Create directory |
| `/api/v1/tree/{path}` | DELETE | Yes | Delete file/directory |

### Version Operations
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/versions/{path}` | GET | Yes | List version history |
| `/api/v1/versions/{path}?v=N` | GET | Yes | Download version N content |
| `/api/v1/versions/{path}` | POST | Yes | Rollback `{"version": N}` |

### SSE Events
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/events` | GET | Yes | SSE stream (create, modify, delete, version events) |

### Permissions
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/permissions/{path}` | PUT | Yes | Set permission `{user_id, permission}` |
| `/api/v1/permissions/{path}` | GET | Yes | List permissions |
| `/api/v1/permissions/{path}?user_id=N` | DELETE | Yes | Remove permission |

### Share Links
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/share/{path}` | POST | Yes | Create link `{password?, expires_in_sec?, max_downloads?}` |
| `/api/v1/share/{id}` | DELETE | Yes | Revoke link |
| `/api/v1/share/{token}` | GET | No | Download via link (public) |

### Quotas
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/usage` | GET | Yes | Current user's usage |
| `/api/v1/admin/quotas/{userID}` | GET | Admin | Get user quota |
| `/api/v1/admin/quotas/{userID}` | PUT | Admin | Set user quota |

### Admin
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/admin/users` | GET | Admin | List users |
| `/api/v1/admin/users` | POST | Admin | Create user `{username, password, is_admin}` |
| `/api/v1/admin/users/{id}` | DELETE | Admin | Delete user |
| `/api/v1/admin/users/{id}/password` | PUT | Admin | Change password `{password}` |
| `/api/v1/admin/sharelinks` | GET | Admin | List share links (`?active=true`) |
| `/api/v1/admin/stats` | GET | Admin | Dashboard stats |
| `/admin/` | - | - | Admin web UI (auth in-app) |

### Metrics
| Endpoint | Port | Description |
|----------|------|-------------|
| `/metrics` | 9090 | Prometheus metrics |

## Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `fruitsalade_http_requests_total` | Counter | HTTP requests by method/path/status |
| `fruitsalade_http_request_duration_seconds` | Histogram | Request duration |
| `fruitsalade_content_bytes_downloaded_total` | Counter | Bytes downloaded |
| `fruitsalade_content_bytes_uploaded_total` | Counter | Bytes uploaded |
| `fruitsalade_content_downloads_total` | Counter | Download count by status |
| `fruitsalade_content_uploads_total` | Counter | Upload count by status |
| `fruitsalade_metadata_tree_size` | Gauge | Files in metadata tree |
| `fruitsalade_metadata_refresh_duration_seconds` | Histogram | Tree rebuild time |
| `fruitsalade_auth_attempts_total` | Counter | Auth attempts by result |
| `fruitsalade_active_tokens` | Gauge | Active JWT tokens |
| `fruitsalade_db_query_duration_seconds` | Histogram | DB query duration |
| `fruitsalade_db_connections_open` | Gauge | Open DB connections |
| `fruitsalade_s3_operation_duration_seconds` | Histogram | S3 operation duration |
| `fruitsalade_s3_operations_total` | Counter | S3 operations by type/status |
| `fruitsalade_permission_checks_total` | Counter | Permission checks by result |
| `fruitsalade_share_downloads_total` | Counter | Share link downloads |
| `fruitsalade_share_links_active` | Gauge | Active share links |
| `fruitsalade_quota_exceeded_total` | Counter | Quota exceeded events by type |
| `fruitsalade_rate_limited_total` | Counter | Rate-limited requests |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | Main server address |
| `METRICS_ADDR` | `:9090` | Metrics server address |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `json` | Log format (json, console) |
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `S3_ENDPOINT` | `http://localhost:9000` | S3/MinIO endpoint |
| `S3_BUCKET` | `fruitsalade` | S3 bucket name |
| `S3_ACCESS_KEY` | `minioadmin` | S3 access key |
| `S3_SECRET_KEY` | `minioadmin` | S3 secret key |
| `JWT_SECRET` | (required) | JWT signing secret |
| `MAX_UPLOAD_SIZE` | `104857600` | Max upload size (100MB) |

## Testing

```bash
# Build
make phase2

# Get auth token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r .token)

# Upload a file
curl -X POST http://localhost:8080/api/v1/content/test/hello.txt \
  -H "Authorization: Bearer $TOKEN" \
  -d "Hello, World!"

# Upload again (creates version 2)
curl -X POST http://localhost:8080/api/v1/content/test/hello.txt \
  -H "Authorization: Bearer $TOKEN" \
  -d "Updated content"

# List versions
curl http://localhost:8080/api/v1/versions/test/hello.txt \
  -H "Authorization: Bearer $TOKEN"

# Download version 1
curl http://localhost:8080/api/v1/versions/test/hello.txt?v=1 \
  -H "Authorization: Bearer $TOKEN"

# Rollback to version 1
curl -X POST http://localhost:8080/api/v1/versions/test/hello.txt \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"version": 1}'

# Upload with conflict detection
curl -X POST http://localhost:8080/api/v1/content/test/hello.txt \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Expected-Version: 2" \
  -d "This fails if version changed"

# Check metrics
curl http://localhost:9090/metrics | grep fruitsalade

# ─── Sharing ───────────────────────────────────────────────────────────

# Create a share link
curl -X POST http://localhost:8080/api/v1/share/test/hello.txt \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"max_downloads": 5}'

# ─── Admin ─────────────────────────────────────────────────────────────

# List users
curl http://localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer $TOKEN"

# Create user
curl -X POST http://localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"bob","password":"secret","is_admin":false}'

# Dashboard stats
curl http://localhost:8080/api/v1/admin/stats \
  -H "Authorization: Bearer $TOKEN"

# ─── Admin UI ──────────────────────────────────────────────────────────
# Open http://localhost:8080/admin/ in a browser
# Login with admin/admin
```

## Docker Test Environment

```bash
# Start Phase 2 environment (server + postgres + minio + 2 FUSE clients)
make phase2-test-env

# Stop
make phase2-test-env-down
```
