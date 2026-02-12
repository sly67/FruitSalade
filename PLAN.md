# FruitSalade Implementation Plan

## Project Structure

Each phase is independently buildable:
- `shared/` — Common types used by all phases
- `phase0/` — PoC (Proof of Concept) ✅ **COMPLETE**
- `phase1/` — MVP (Minimum Viable Product)
- `phase2/` — Production (Full Features)

Build any phase with: `make phase0`, `make phase1`, `make phase2`

---

## Phase 0: Proof of Concept ✅ COMPLETE

**Goal:** Validate the architecture works end-to-end.

### 0.1 Shared Types [S] ✅
- [x] `shared/pkg/models/filenode.go` — FileNode, CacheEntry structs
- [x] `shared/pkg/protocol/api.go` — API types

### 0.2 Server — HTTP Framework [S] ✅
*Location: `phase0/internal/api/`*
- [x] Basic HTTP server structure
- [x] Wire up handlers to main.go
- [x] Add JSON encoding helpers
- [x] Add request logging middleware

### 0.3 Server — Local Storage [M] ✅
*Location: `phase0/internal/storage/`*
- [x] Storage interface defined
- [x] LocalStorage implementation (reads from disk)
- [x] Build metadata tree from directory
- [x] Test with sample testdata

### 0.4 Server — Metadata API [S] ✅
- [x] `GET /api/v1/tree` — return full tree as JSON
- [x] `GET /api/v1/tree/{path}` — return subtree
- [x] `GET /health` — health check
- [x] Integration tests

### 0.5 Server — Content API with Range [M] ✅
- [x] `GET /api/v1/content/{id}` — full file
- [x] Parse `Range: bytes=start-end` header
- [x] Return `206 Partial Content` with `Content-Range`
- [x] Return `416 Range Not Satisfiable` for invalid
- [x] Integration tests for range requests

### 0.6 FUSE Client — Basic Mount [M] ✅
*Location: `phase0/internal/fuse/`*
- [x] Add `github.com/hanwen/go-fuse/v2` dependency
- [x] Create root inode
- [x] Implement `Getattr()` — from cached metadata
- [x] Implement `Readdir()` — from cached metadata
- [x] Test mount/unmount works

### 0.7 FUSE Client — Metadata Integration [M] ✅
- [x] HTTP client to fetch `/api/v1/tree`
- [x] Parse JSON into FileNode tree
- [x] Build inode map from metadata
- [x] Implement `Lookup()` — resolve paths
- [x] Implement `Getattr()` — from cached metadata (NO DOWNLOAD)
- [x] Implement `Readdir()` — from cached metadata (NO DOWNLOAD)
- [x] Test: `ls -la` shows correct sizes (no downloads)

### 0.8 FUSE Client — On-Demand Fetch [L] ✅
*Location: `phase0/internal/cache/`*
- [x] Cache struct with LRU eviction
- [x] HTTP client for `/api/v1/content/{id}`
- [x] Implement `Open()` — check cache, prepare handle
- [x] Implement `Read()`:
  - Cached: read from local file
  - Not cached: fetch → temp file → rename → read
- [x] Implement `Release()` — cleanup
- [x] Test: `cat` fetches, second `cat` reads cache

### 0.9 FUSE Client — Range Reads [M] ✅
- [x] Small files (<1MB): fetch fully and cache
- [x] Large files (>=1MB): fetch only requested ranges
- [x] Test with `head`, `tail` on large files

### 0.10 End-to-End Validation [S] ✅
- [x] Create test files in `phase0/testdata/`
- [x] Run server: `make phase0-run-server`
- [x] Mount client: `sudo ./bin/phase0-fuse -mount /mnt/test`
- [x] Verify: `ls` → no downloads, `cat` → downloads, cache works
- [x] Range reads work for large files
- [x] Cache stats reported on exit

---

## Phase 1: MVP ✅ COMPLETE

**Goal:** Production-ready server and Linux client.

### 1.1 PostgreSQL Metadata [M] ✅
*Location: `phase1/internal/metadata/postgres/`*
- [x] Add `sqlx` and `lib/pq` dependencies
- [x] Design schema (files, directories, versions)
- [x] Implement PostgresStore
- [x] Add migrations in `phase1/migrations/`

### 1.2 S3 Storage Backend [M] ✅
*Location: `phase1/internal/storage/s3/`*
- [x] Add `aws-sdk-go-v2` dependency
- [x] Implement S3Storage
- [x] Support MinIO for local dev
- [x] Range reads from S3

### 1.3 JWT Authentication [M] ✅
*Location: `phase1/internal/auth/`*
- [x] Add `golang-jwt/jwt/v5`
- [x] `POST /api/v1/auth/token` — login
- [x] Auth middleware for protected routes
- [x] Token refresh
- [x] Device registration

### 1.4 Docker Deployment [S] ✅
*Location: `phase1/docker/`*
- [x] Dockerfile (multi-stage build)
- [x] docker-compose.yml (postgres, minio, server)
- [x] Health checks
- [x] Environment configuration

### 1.5 Client — Pin/Unpin [S] ✅
- [x] Persistent pin storage (JSON file)
- [x] Exclude pinned from eviction
- [x] CLI: `fruitsalade pin <path>`
- [x] CLI: `fruitsalade unpin <path>`
- [x] CLI: `fruitsalade pinned` — list pinned

### 1.6 Client — Systemd Service [S] ✅
- [x] Create `fruitsalade.service` unit file
- [x] Handle SIGTERM gracefully (unmount)
- [x] Logging to journald
- [x] Installation docs

### 1.7 Documentation [S] ✅
- [x] README.md with installation
- [x] API documentation
- [x] Architecture diagrams
- [x] Getting started guide

---

## Phase 2: Production ✅ COMPLETE

**Goal:** Full-featured system with Windows support.

### 2.1 Windows Client [XL] ✅
*Location: `phase2/windows/` and `phase2/cmd/windows-client/`*
- [x] C++ CfAPI shim (sync root, placeholders, hydration callbacks)
- [x] CGO integration with Go core
- [x] cgofuse cross-platform FUSE backend
- [x] Windows Service support

### 2.2 Observability [M] ✅
- [x] Prometheus metrics (`/metrics`)
- [x] Structured logging (zap JSON)
- [x] Grafana dashboard
- [x] Request logging with correlation IDs

### 2.3 Admin UI [L] ✅
- [x] Admin API endpoints
- [x] Embedded web UI (vanilla HTML/CSS/JS, `go:embed`)
- [x] User management (CRUD, groups, password changes)
- [x] Storage statistics and dashboard

### 2.4 Multi-User Support [L] ✅
- [x] User groups with nested hierarchy and RBAC roles
- [x] ACL-based sharing/permissions with path inheritance
- [x] Per-user quotas (storage, bandwidth, RPM, upload size)
- [x] File visibility (public/group/private)

### 2.5 Webapp [L] ✅
- [x] Full-featured file browser at `/app/`
- [x] Dark mode, sortable columns, kebab/context menus
- [x] Multi-select batch actions, inline rename, detail panel
- [x] Toast notifications, shared utilities, file-type icons

### 2.6 Mobile Apps [XL]
- [ ] API adjustments for mobile
- [ ] Android app
- [ ] iOS app
- [ ] Encrypted local cache

---

## Complexity Key

| Rating | Meaning |
|--------|---------|
| S | Small — straightforward, few unknowns |
| M | Medium — some integration complexity |
| L | Large — significant work or unfamiliar APIs |
| XL | Extra large — research-heavy, high uncertainty |

---

## Quick Start

```bash
# 1. Build Phase 0
cd /home/Karthangar/Projets/FruitSalade
export PATH=$PATH:/usr/local/go/bin
cd phase0 && go build -o ../bin/phase0-server ./cmd/server
cd phase0 && go build -o ../bin/phase0-fuse ./cmd/fuse-client

# 2. Run server
./bin/phase0-server -data ./phase0/testdata

# 3. In another terminal, mount client
sudo ./bin/phase0-fuse -mount /mnt/fruitsalade -server http://localhost:8080

# 4. Test
ls /mnt/fruitsalade              # Shows files (no download)
cat /mnt/fruitsalade/hello.txt   # Fetches and displays
cat /mnt/fruitsalade/hello.txt   # Reads from cache
head -c 100 /mnt/fruitsalade/huge.bin  # Range read (large file)

# 5. Unmount
sudo umount /mnt/fruitsalade
```

## Phase 0 Test Results

| Test | Result |
|------|--------|
| `ls` (no download) | ✅ Only metadata fetched |
| `cat` small file | ✅ Fetched and cached |
| `cat` (cache hit) | ✅ No server request |
| Range read (large file) | ✅ Only requested bytes fetched |
| Subdirectories | ✅ Nested files work |
| LRU cache | ✅ Files cached with size limit |
