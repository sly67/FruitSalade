# Phase 2: Production Features

## Aim

Extend the MVP into a full-featured production system with write support, observability, and prepare for multi-platform clients.

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

### 2.3 Write Operations - FUSE Client (Pending)
- [ ] FUSE Create operation
- [ ] FUSE Write operation
- [ ] FUSE Mkdir operation
- [ ] FUSE Unlink operation
- [ ] FUSE Rmdir operation

## Planned Features

### 2.4 Versioning & Conflict Detection (Not Started)
- [ ] Per-file version tracking in metadata
- [ ] Conflict detection on concurrent modifications
- [ ] Version history API
- [ ] Rollback to previous versions

### 2.5 Admin UI (Not Started)
- [ ] Admin API endpoints
- [ ] Web UI

### 2.6 Windows Client (Not Started)
- [ ] C++ CfAPI shim
- [ ] CGO integration

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│   Main Server   │     │  Metrics Server │
│   :8080         │     │   :9090         │
│                 │     │   /metrics      │
│  - /health      │     └─────────────────┘
│  - /api/v1/*    │
└────────┬────────┘
         │
    ┌────┴────┐
    │ Logging │  ← Structured JSON (zap)
    │ Metrics │  ← Prometheus counters/histograms
    └────┬────┘
         │
   ┌─────┴─────┐
   │ PostgreSQL│ ← Metadata + Auth
   │   MinIO   │ ← File content
   └───────────┘
```

## API Endpoints

### Read Operations (from Phase 1)
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/health` | GET | No | Health check |
| `/api/v1/auth/token` | POST | No | Login, get JWT |
| `/api/v1/tree` | GET | Yes | Full metadata tree |
| `/api/v1/tree/{path}` | GET | Yes | Subtree metadata |
| `/api/v1/content/{id}` | GET | Yes | File content (Range supported) |

### Write Operations (Phase 2)
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/content/{path}` | POST | Yes | Upload file content |
| `/api/v1/tree/{path}?type=dir` | PUT | Yes | Create directory |
| `/api/v1/tree/{path}` | DELETE | Yes | Delete file/directory |

### Metrics
| Endpoint | Port | Description |
|----------|------|-------------|
| `/metrics` | 9090 | Prometheus metrics |

## Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `fruitsalade_http_requests_total` | Counter | Total HTTP requests by method/path/status |
| `fruitsalade_http_request_duration_seconds` | Histogram | Request duration |
| `fruitsalade_content_bytes_downloaded_total` | Counter | Total bytes downloaded |
| `fruitsalade_content_bytes_uploaded_total` | Counter | Total bytes uploaded |
| `fruitsalade_content_downloads_total` | Counter | Download count by status |
| `fruitsalade_content_uploads_total` | Counter | Upload count by status |
| `fruitsalade_metadata_tree_size` | Gauge | Number of files in tree |
| `fruitsalade_metadata_refresh_duration_seconds` | Histogram | Tree rebuild time |
| `fruitsalade_auth_attempts_total` | Counter | Auth attempts by result |
| `fruitsalade_active_tokens` | Gauge | Active JWT tokens |
| `fruitsalade_db_query_duration_seconds` | Histogram | DB query duration |
| `fruitsalade_db_connections_open` | Gauge | Open DB connections |
| `fruitsalade_s3_operation_duration_seconds` | Histogram | S3 operation duration |
| `fruitsalade_s3_operations_total` | Counter | S3 operations by type/status |

## Configuration

### Environment Variables
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

## Build & Run

```bash
# Build Phase 2 server
make phase2-server

# Run locally (requires PostgreSQL + MinIO)
DATABASE_URL="postgres://..." JWT_SECRET="secret" ./bin/phase2-server

# Build for Docker (uses Phase 1 Dockerfile as base)
# TODO: Create Phase 2 Docker environment
```

## Testing Write Operations

```bash
# Get auth token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r .token)

# Upload a file
curl -X POST http://localhost:8080/api/v1/content/test/hello.txt \
  -H "Authorization: Bearer $TOKEN" \
  -d "Hello, World!"

# Create a directory
curl -X PUT "http://localhost:8080/api/v1/tree/test/subdir?type=dir" \
  -H "Authorization: Bearer $TOKEN"

# Delete a file
curl -X DELETE http://localhost:8080/api/v1/tree/test/hello.txt \
  -H "Authorization: Bearer $TOKEN"

# Check metrics
curl http://localhost:9090/metrics | grep fruitsalade
```
