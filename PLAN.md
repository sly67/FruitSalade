# FruitSalade Implementation Plan

## Project Structure

```
FruitSalade/
├── shared/       # Shared types and client code
├── fruitsalade/  # Main application (server, clients, Docker)
├── go.work       # Go workspace
└── Makefile      # Build targets
```

Build with: `make server`, `make fuse`, `make seed`

---

## Completed Features

### Core
- [x] PostgreSQL metadata store with migrations (001-007)
- [x] Multi-backend storage (S3/Local/SMB) with per-group routing
- [x] JWT authentication with device tokens
- [x] OIDC federated authentication (Keycloak, Auth0, etc.)
- [x] Token management (revoke, refresh, sessions)

### File Operations
- [x] File upload/download with Range support
- [x] Directory creation and recursive deletion
- [x] File versioning with rollback
- [x] Conflict detection (X-Expected-Version, If-Match)
- [x] SSE real-time sync

### Sharing & Permissions
- [x] ACL-based permissions with path inheritance
- [x] Share links (password, expiry, download limits)
- [x] User groups with nested hierarchy and RBAC roles
- [x] File visibility (public/group/private)
- [x] Auto-provisioning of group/home directories

### Clients
- [x] Linux FUSE client (read + write, LRU cache, pinning, SSE)
- [x] Windows client (CfAPI + cgofuse dual backend, Windows Service)
- [x] Pin/unpin CLI subcommands

### Observability & Admin
- [x] Prometheus metrics + Grafana dashboard
- [x] Structured JSON logging (zap)
- [x] Admin web UI (embedded, no build step)
- [x] Webapp file browser (dark mode, batch actions, inline rename)
- [x] Rate limiting and user quotas
- [x] CI pipeline (GitHub Actions)

### Deployment
- [x] Multi-container Docker (PostgreSQL + MinIO + server + FUSE clients)
- [x] Single-container Docker (PostgreSQL + server, local storage)
- [x] Systemd service files
- [x] TLS 1.3 support

---

## Future Work

### Mobile Apps
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
