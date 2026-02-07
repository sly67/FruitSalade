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

## Phase 1: MVP

**Goal:** Production-ready server and Linux client.

### 1.1 PostgreSQL Metadata [M]
*Location: `phase1/internal/metadata/postgres/`*
- [ ] Add `sqlx` and `lib/pq` dependencies
- [ ] Design schema (files, directories, versions)
- [ ] Implement PostgresStore
- [ ] Add migrations in `phase1/migrations/`

### 1.2 S3 Storage Backend [M]
*Location: `phase1/internal/storage/s3/`*
- [ ] Add `aws-sdk-go-v2` dependency
- [ ] Implement S3Storage
- [ ] Support MinIO for local dev
- [ ] Range reads from S3

### 1.3 JWT Authentication [M]
*Location: `phase1/internal/auth/`*
- [ ] Add `golang-jwt/jwt/v5`
- [ ] `POST /api/v1/auth/token` — login
- [ ] Auth middleware for protected routes
- [ ] Token refresh
- [ ] Device registration

### 1.4 Docker Deployment [S]
*Location: `phase1/docker/`*
- [x] Dockerfile (multi-stage build)
- [x] docker-compose.yml (postgres, minio, server)
- [ ] Health checks
- [ ] Environment configuration

### 1.5 Client — Pin/Unpin [S]
- [ ] Persistent pin storage (JSON file or SQLite)
- [ ] Exclude pinned from eviction
- [ ] CLI: `fruitsalade pin <path>`
- [ ] CLI: `fruitsalade unpin <path>`
- [ ] CLI: `fruitsalade pinned` — list pinned

### 1.6 Client — Systemd Service [S]
- [ ] Create `fruitsalade.service` unit file
- [ ] Handle SIGTERM gracefully (unmount)
- [ ] Logging to journald
- [ ] Installation docs

### 1.7 Documentation [S]
- [ ] README.md with installation
- [ ] API documentation
- [ ] Architecture diagrams
- [ ] Getting started guide

---

## Phase 2: Production

**Goal:** Full-featured system with Windows support.

### 2.1 Windows Client [XL]
*Location: `phase2/windows/` and `phase2/cmd/windows-client/`*
- [ ] C++ CfAPI shim:
  - Sync root registration
  - Placeholder creation
  - Hydration callbacks
- [ ] CGO integration with Go core
- [ ] Windows installer
- [ ] Test with Explorer, common apps

### 2.2 Observability [M]
- [ ] Prometheus metrics (`/metrics`)
- [ ] Structured logging (JSON)
- [ ] Grafana dashboard
- [ ] Request tracing

### 2.3 Admin UI [L]
- [ ] Admin API endpoints
- [ ] Web UI (Vue/React)
- [ ] User management
- [ ] Device management
- [ ] Storage statistics

### 2.4 Multi-User Support [L]
- [ ] User isolation
- [ ] Sharing/permissions
- [ ] Per-user quotas

### 2.5 Mobile Apps [XL]
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
