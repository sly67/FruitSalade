# Phase 2: Production Features

## Aim

Extend the MVP into a full-featured production system with write support, observability, multi-platform clients, and administrative capabilities.

## Planned Features

### 2.1 Observability (Priority: High, Effort: Small)
- [ ] Prometheus metrics endpoint (`/metrics`)
- [ ] Structured JSON logging (zap or zerolog)
- [ ] Request tracing with correlation IDs
- [ ] Grafana dashboard templates
- [ ] Cache hit/miss metrics
- [ ] Download/upload throughput metrics

### 2.2 Write Operations (Priority: High, Effort: Medium)
- [ ] File upload endpoint (`POST /api/v1/content`)
- [ ] File creation (`PUT /api/v1/tree/{path}`)
- [ ] File deletion (`DELETE /api/v1/tree/{path}`)
- [ ] Directory creation (`POST /api/v1/tree/{path}?type=dir`)
- [ ] FUSE write operations (Create, Write, Mkdir, Unlink, Rmdir)
- [ ] Atomic uploads with temp files
- [ ] Upload progress tracking

### 2.3 Versioning & Conflict Detection (Priority: High, Effort: Medium)
- [ ] Per-file version tracking in metadata
- [ ] Conflict detection on concurrent modifications
- [ ] Simple conflict resolution (last-write-wins or manual)
- [ ] Version history API
- [ ] Rollback to previous versions

### 2.4 Admin UI (Priority: Medium, Effort: Large)
- [ ] Admin API endpoints (user CRUD, device management)
- [ ] Web UI (Vue.js or React)
- [ ] User management interface
- [ ] Device/token management
- [ ] Storage statistics dashboard
- [ ] Activity logs

### 2.5 Windows Client (Priority: High, Effort: Extra Large)
- [ ] C++ CfAPI shim for Windows Cloud Files API
- [ ] CGO integration with Go client core
- [ ] Sync root registration
- [ ] Placeholder file creation
- [ ] Hydration callbacks (on-demand fetch)
- [ ] Explorer shell integration
- [ ] Windows installer (MSI or MSIX)
- [ ] Requires: Windows 10 1809+, Visual Studio, Windows SDK

### 2.6 Multi-User Support (Priority: Medium, Effort: Large)
- [ ] User data isolation
- [ ] Per-user storage quotas
- [ ] Sharing and permissions
- [ ] User groups/teams

### 2.7 Mobile Apps (Priority: Low, Effort: Extra Large)
- [ ] Android app (Kotlin)
- [ ] iOS app (Swift)
- [ ] Browse and manual download
- [ ] Offline pinning
- [ ] Encrypted local cache

## Status: Not Started

Phase 2 development begins after Phase 1 is complete and tested.

## Architecture Changes

### Database Schema Additions
```sql
-- File versions
ALTER TABLE files ADD COLUMN version INTEGER DEFAULT 1;
ALTER TABLE files ADD COLUMN previous_version_id TEXT;

-- Version history
CREATE TABLE file_versions (
    id TEXT PRIMARY KEY,
    file_id TEXT REFERENCES files(id),
    version INTEGER,
    hash TEXT,
    s3_key TEXT,
    size BIGINT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- User quotas
ALTER TABLE users ADD COLUMN quota_bytes BIGINT DEFAULT 10737418240; -- 10GB
ALTER TABLE users ADD COLUMN used_bytes BIGINT DEFAULT 0;
```

### New API Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/metrics` | GET | Prometheus metrics |
| `/api/v1/content` | POST | Upload new file |
| `/api/v1/tree/{path}` | PUT | Create/update file metadata |
| `/api/v1/tree/{path}` | DELETE | Delete file/directory |
| `/api/v1/versions/{id}` | GET | Get version history |
| `/api/v1/admin/users` | GET/POST | User management |
| `/api/v1/admin/devices` | GET/DELETE | Device management |

## Build & Run

```bash
# Build Phase 2 components (once implemented)
make phase2

# Build Windows client (requires Windows + CGO)
make phase2-windows

# Run tests
make phase2-test
```

## Dependencies

Phase 2 requires Phase 1 to be complete:
- PostgreSQL metadata store ✓
- S3 content storage ✓
- JWT authentication ✓
- FUSE client foundation ✓
- Docker environment ✓

## Development Order

Recommended implementation sequence:
1. **Observability** - Metrics and logging (foundation for debugging)
2. **Write Operations** - File upload/create/delete
3. **Versioning** - Track changes, detect conflicts
4. **Admin UI** - Management interface
5. **Windows Client** - Platform expansion
6. **Multi-User** - Enterprise features
7. **Mobile Apps** - Mobile access
