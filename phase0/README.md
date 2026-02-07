# Phase 0: Proof of Concept

## Aim

Validate the core architecture and prove that on-demand file synchronization with FUSE placeholders is technically feasible. This phase is research-focused with minimal production concerns.

## Features

### Server
- Minimal HTTP server serving files from local filesystem
- Tree endpoint (`GET /api/v1/tree`) returning full metadata hierarchy
- Content endpoint (`GET /api/v1/content/{path}`) with HTTP Range support
- Health check endpoint (`GET /health`)

### FUSE Client
- Mount virtual filesystem using go-fuse v2
- Lazy content fetching - metadata-only on `ls`, content on `cat`/`open`
- Simple LRU file cache with configurable size
- Basic offline support for cached files

### Cache
- File-based LRU cache with atomic writes (temp file + rename)
- Pin/unpin support for "always keep" files
- CLI tool for cache inspection and management

## Status: Complete

All Phase 0 objectives achieved:
- [x] Server serves local files over HTTP
- [x] FUSE client mounts and displays file tree
- [x] Content only fetched on explicit read
- [x] Cache stores downloaded files
- [x] Range requests work for partial reads

## Build & Run

```bash
# Build all Phase 0 components
make phase0

# Run server (serves ./testdata by default)
make phase0-run-server

# Mount FUSE client (requires separate terminal)
sudo ./bin/phase0-fuse -mount /mnt/fruitsalade -server http://localhost:8080
```

## Key Learnings

1. FUSE `Getattr`/`Lookup`/`Readdir` must NOT trigger content downloads
2. Content fetch only on `Open`/`Read` - this is critical for `ls`/`find`/`du`
3. Atomic cache writes prevent corruption on interrupted downloads
4. go-fuse v2 provides clean interface for implementing custom filesystems
