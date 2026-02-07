# On-Demand Placeholder File Sync — Requirements Specification

**Project goal:** deliver a self-hosted, Docker-hostable server and cross-platform clients that implement true _on-demand file placeholders_ (display entire folder tree, download file content only when accessed) with offline reuse of accessed files.

---

## 1. Executive Summary

Build a modern file sync system that behaves like OneDrive/Dropbox "Files On-Demand" but is self-hosted and Docker-deployable. The server and clients are implemented in **Go**, with a thin **C++ shim** for Windows Cloud Files API integration.

Desktop clients (Windows + Linux) expose OS-level placeholder files using native mechanisms:
- **Windows:** Cloud Files API (CfAPI) via C++ shim called from Go
- **Linux:** FUSE via go-fuse library

Mobile clients provide reduced feature sets (browse, on-demand download, pin for offline).

---

## 2. High-Level Requirements

### 2.1 Must-Have

- Server runs inside Docker, implemented in **Go**
- Clients for **Windows** and **Linux**: full placeholder behavior, offline reuse, cache management, eviction policies
- Windows client uses **Cloud Files API (CfAPI)** for true OS-level placeholders via C++ shim
- Linux client uses **FUSE** (go-fuse library) exposing placeholders that fetch content on open/read
- Metadata sync separated from content transfer
- Range-reads and partial downloads supported
- Authentication: token-based, OIDC compatible
- TLS 1.3 required, configurable storage backend (local FS, S3 compatible)

### 2.2 Should-Have (Post-MVP)

- Multi-user support and per-device tokens
- Per-file versioning and simple conflict resolution
- Prometheus metrics + structured logging

### 2.3 Nice-to-Have

- Transparent integration with OS search (indexing hints)
- Desktop icons/overlay badges for placeholder states
- Admin web UI for user/device management

---

## 3. Architecture

```
+--------------------+           HTTPS              +--------------------+
| Desktop Client     | <-------------------------> | Sync Server (Docker)|
| - Placeholder FS   |                             | - Metadata API      |
| - Cache Manager    |                             | - Auth Service      |
| - Fetcher (range)  |                             | - Content API       |
+--------------------+                             +--------------------+
         |                                                  |
         v                                                  v
    Local Cache                                      Object Storage
    (disk)                                     (Local FS / S3 compatible)
```

**Key protocols:** HTTPS (REST), token auth, HTTP Range for content.

### Component Overview

| Component | Language | Key Libraries |
|-----------|----------|---------------|
| Server | Go | net/http or Gin/Echo, sqlx, aws-sdk-go-v2 |
| Linux Client | Go | hanwen/go-fuse, resty/req |
| Windows Client | Go + C++ | Go core + CfAPI C++ shim via CGO |
| Mobile | Flutter/React Native | Platform HTTP + encrypted cache |

---

## 4. Functional Details

### 4.1 Placeholder Semantics

- Directories and filenames visible without downloading content
- Placeholders contain metadata: size, mtime, readonly flag, hash (optional)
- Placeholder open → client fetches content (blocking until sufficient data) and writes to local cache
- After fetch, OS and apps see the file as normal (offline until eviction)
- If offline and file not cached, opening fails gracefully with descriptive error

### 4.2 Cache & Eviction

- Configurable cache directory and maximum size per device
- LRU eviction with pinned files ("Always keep on this device")
- Manual free-up action via CLI
- Atomic downloads: download to temp file, then rename

### 4.3 Metadata Sync

- Lightweight metadata sync channel (polling or SSE for changes)
- Clients can refresh only relevant subtrees (directory level)
- **Never** bundle content with metadata responses

### 4.4 Reads & Writes

- **Initial MVP:** read-only / pull-only sync
- Support HTTP Range requests for partial reads
- Future: writeback with upload transactions and versioning

---

## 5. Server Specification

### 5.1 Core

- **Language:** Go
- **Framework:** net/http stdlib, or Gin/Echo/Fiber for convenience
- **API:** REST (JSON) for metadata, ranged GET for content

### 5.2 Storage Backends

- Pluggable backend interface: Local FS, S3 compatible (MinIO, AWS S3, Backblaze B2)
- Metadata in PostgreSQL (production) or SQLite (PoC/single-user)

### 5.3 Docker Deployment

- Dockerfile + docker-compose provided
- Application container stateless; mounts for persistent metadata/objects or point at external S3

### 5.4 API Endpoints

```
GET  /api/v1/tree              # Full metadata tree
GET  /api/v1/tree/{path}       # Subtree for path
GET  /api/v1/content/{file_id} # File content (supports Range header)
POST /api/v1/auth/token        # Authentication
```

---

## 6. Client Specification

### 6.1 Linux Client

**Integration:** FUSE via [hanwen/go-fuse](https://github.com/hanwen/go-fuse)

**Responsibilities:**
- Mount virtual filesystem exposing placeholders
- Fetch content only on `open()`/`read()`, never on `stat()`/`readdir()`
- Write fetched content to local cache atomically (temp file → rename)
- LRU cache eviction, manual pin/unpin
- CLI and optional systemd service

**Key packages:**
- `github.com/hanwen/go-fuse/v2/fs` — FUSE implementation
- `github.com/go-resty/resty/v2` or `net/http` — HTTP client

### 6.2 Windows Client

**Integration:** Cloud Files API (CfAPI) via C++ shim

**Architecture:**
```
+------------------+     CGO      +------------------+
| Go Client Core   | <---------> | C++ CfAPI Shim   |
| - Cache Manager  |             | - Sync Root Reg  |
| - HTTP Fetcher   |             | - Hydration CB   |
| - Metadata Cache |             | - Placeholder Op |
+------------------+             +------------------+
```

**Responsibilities:**
- Register sync root for virtual drive
- Create placeholder files, respond to OS hydration callbacks
- Go core handles all network, caching, business logic
- C++ shim is thin layer (~500-1000 LOC) for CfAPI calls only

**Why C++ shim:**
- No Go bindings exist for CfAPI
- CfAPI is callback-based, easier to handle in C++
- Shim communicates with Go via CGO or local IPC

### 6.3 Mobile (Android/iOS)

- Browse, manual download, offline pin
- No OS-level placeholder integration in v1
- Use platform HTTP + local encrypted cache

---

## 7. Security

- TLS 1.3 required; support Let's Encrypt in recommended deployment
- OIDC for auth (Keycloak, Authentik). Per-device JWT with revocation
- Optional E2E encryption at rest (later phase)

---

## 8. Technology Stack Summary

### Server
| Component | Choice |
|-----------|--------|
| Language | Go 1.22+ |
| HTTP Framework | net/http or Gin |
| Database | PostgreSQL / SQLite |
| Object Storage | Local FS, S3, MinIO |
| Auth | JWT, OIDC |

### Linux Client
| Component | Choice |
|-----------|--------|
| Language | Go 1.22+ |
| FUSE | hanwen/go-fuse v2 |
| HTTP | net/http or resty |
| Cache | Local disk, LRU |

### Windows Client
| Component | Choice |
|-----------|--------|
| Core | Go 1.22+ |
| CfAPI Shim | C++ (Win SDK) |
| Integration | CGO or named pipes |

---

## 9. Development Phases

### Phase 0 — Research & PoC
- PoC: Go server serving metadata + ranged GETs from local storage
- PoC: Linux FUSE client mounting metadata tree, fetching on open
- Spike: Windows CfAPI C++ shim feasibility (register sync root, create placeholder, hydrate)

### Phase 1 — MVP
- Production Docker server, PostgreSQL, S3 backend
- Linux client with LRU cache, pin/unpin, eviction, range reads
- Windows prototype with basic CfAPI integration

### Phase 2 — Harden & Features
- Full Windows client with complete CfAPI support
- Mobile apps (limited features)
- Metrics, logging, admin UI, multi-user

---

## 10. Reference Implementations

Study for design patterns (not code reuse):

- **rclone VFS** — Go-based VFS with lazy reads, range fetching, LRU cache. Excellent reference for Linux client and cache layer.
- **SeaDrive (Seafile)** — Closest self-hosted UX for virtual drive + eviction + pinning behavior.
- **Windows CfAPI SDK samples** — Mandatory patterns for sync root registration, hydration callbacks, placeholder attributes.

---

## 11. Known Pitfalls

- Do NOT use WebDAV mounts for large trees (poor performance, no placeholders)
- Do NOT trigger downloads on `stat`, `readdir`, or checksum calls
- Do NOT treat metadata and content as the same sync channel
- Do NOT assume offline access works — fail gracefully

---

## 12. Explicit Non-Goals

- Custom kernel modules
- Real-time collaborative editing
- Office-suite integration
- Media transcoding
- Full mobile filesystem integration in v1
- Bidirectional sync with conflict resolution in initial MVP

---

## 13. Glossary

| Term | Definition |
|------|------------|
| Placeholder | Filesystem entry with metadata but no content |
| On-demand sync | Content transferred only when accessed |
| Range read | Partial file retrieval via HTTP Range header |
| CfAPI | Windows Cloud Files API for Files On-Demand |
| FUSE | Filesystem in Userspace (Linux) |
| Hydration | Process of fetching content for a placeholder |
