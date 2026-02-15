# FruitSalade - Application

This is the main application directory containing the server, clients, and all internal packages.

## Implemented Features

### Observability
- [x] Prometheus metrics endpoint (`/metrics` on port 9090)
- [x] Structured JSON logging (zap)
- [x] Request logging with correlation IDs
- [x] HTTP, content, database, S3, auth, sharing, quota metrics

### Write Operations - Server
- [x] File upload (`POST /api/v1/content/{path}`)
- [x] Directory creation (`PUT /api/v1/tree/{path}?type=dir`)
- [x] File/directory deletion (`DELETE /api/v1/tree/{path}`)
- [x] Recursive directory deletion
- [x] Automatic parent directory creation
- [x] Configurable max upload size

### Write Operations - FUSE Client
- [x] Create, Write, Flush, Mkdir, Unlink, Rmdir, Rename, Setattr

### File Versioning
- [x] Automatic version history on upload
- [x] Version backup to storage backend
- [x] List/download/rollback versions
- [x] Database migration for `file_versions` table

### Conflict Detection
- [x] Optimistic concurrency via `X-Expected-Version` header
- [x] ETag-based via `If-Match` header
- [x] 409 Conflict response with version details

### SSE Real-Time Sync
- [x] `events.Broadcaster` with subscribe/unsubscribe/publish
- [x] SSE endpoint (`GET /api/v1/events`)
- [x] FUSE client `--watch` flag for live metadata refresh

### File Sharing
- [x] ACL-based permissions with path inheritance
- [x] Share links with optional password, expiry, and download limits

### Rate Limiting & Quotas
- [x] Per-user quotas: storage, bandwidth/day, requests/min, upload size
- [x] In-memory token bucket rate limiter

### Web App
- [x] Vanilla HTML/CSS/JS embedded via `go:embed` (no build step)
- [x] Served at `/app/` with hash-based SPA routing
- [x] Dashboard, users, files, share links, groups, storage management

### Webapp
- [x] Full-featured file browser at `/app/`
- [x] Dark mode, sortable columns, kebab/context menus
- [x] Multi-select batch actions, inline rename, detail panel

### User Groups
- [x] Nested group hierarchy with RBAC roles (admin/editor/viewer)
- [x] File visibility (public/group/private) with group ownership
- [x] Auto-provisioning of group directories
- [x] Cycle-prevention DB trigger

### Multi-Backend Storage
- [x] S3/MinIO, local filesystem, SMB backends
- [x] Per-group storage locations via storage router
- [x] Admin API and UI for managing storage locations

### Windows Client
- [x] CfAPI + cgofuse dual backend
- [x] C++ CfAPI shim for cloud files API
- [x] Windows Service support

### CI & Deployment
- [x] GitHub Actions (lint, test, build, Docker)
- [x] TLS 1.3 support
- [x] OIDC authentication
- [x] Token management (revoke/refresh/sessions)
- [x] Systemd service files
- [x] Grafana dashboard

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | Main server address |
| `METRICS_ADDR` | `:9090` | Metrics server address |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `json` | Log format (json, console) |
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `JWT_SECRET` | (required) | JWT signing secret |
| `STORAGE_BACKEND` | `local` | Storage backend: `local` or `s3` |
| `LOCAL_STORAGE_PATH` | `/data/storage` | Path for local filesystem storage |
| `S3_ENDPOINT` | `http://localhost:9000` | S3/MinIO endpoint |
| `S3_BUCKET` | `fruitsalade` | S3 bucket name |
| `S3_ACCESS_KEY` | `minioadmin` | S3 access key |
| `S3_SECRET_KEY` | `minioadmin` | S3 secret key |
| `MAX_UPLOAD_SIZE` | `104857600` | Max upload size (100MB) |
| `TLS_CERT_FILE` | (empty) | TLS certificate file (enables HTTPS with TLS 1.3) |
| `TLS_KEY_FILE` | (empty) | TLS private key file |
| `OIDC_ISSUER_URL` | (empty) | OIDC provider URL (enables federated auth) |
| `OIDC_CLIENT_ID` | (empty) | OIDC client ID |
| `OIDC_CLIENT_SECRET` | (empty) | OIDC client secret |
| `OIDC_ADMIN_CLAIM` | `is_admin` | OIDC token claim key for admin |
| `OIDC_ADMIN_VALUE` | `true` | OIDC claim value that indicates admin |

## Testing

```bash
# Build
make server

# Get auth token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r .token)

# Upload a file
curl -X POST http://localhost:8080/api/v1/content/test/hello.txt \
  -H "Authorization: Bearer $TOKEN" \
  -d "Hello, World!"

# List versions
curl http://localhost:8080/api/v1/versions/test/hello.txt \
  -H "Authorization: Bearer $TOKEN"

# Upload with conflict detection
curl -X POST http://localhost:8080/api/v1/content/test/hello.txt \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Expected-Version: 1" \
  -d "Updated content"

# Check metrics
curl http://localhost:9090/metrics | grep fruitsalade

# Web app: http://localhost:8080/app/
# Webapp:   http://localhost:8080/app/
# Login with admin/admin
```

## Deployment

### Systemd

```bash
# Copy service files
sudo cp fruitsalade/deploy/fruitsalade-server.service /etc/systemd/system/
sudo cp fruitsalade/deploy/fruitsalade-fuse@.service /etc/systemd/system/

# Edit environment file
sudo nano /etc/fruitsalade/server.env

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable --now fruitsalade-server
sudo systemctl enable --now fruitsalade-fuse@work
```

### TLS

```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
TLS_CERT_FILE=cert.pem TLS_KEY_FILE=key.pem ./bin/server
```

### OIDC (Keycloak example)

```bash
OIDC_ISSUER_URL=https://keycloak.example.com/realms/fruitsalade \
OIDC_CLIENT_ID=fruitsalade \
OIDC_CLIENT_SECRET=secret \
./bin/server
```

### Grafana Dashboard

Import `fruitsalade/deploy/grafana-dashboard.json` into Grafana, selecting your Prometheus datasource.

## Docker

```bash
# Full environment (server + minio + 2 FUSE clients)
make docker-up

# Server standalone (local storage, no S3)
make docker-run

# Stop
make docker-down
```
