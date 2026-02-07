.PHONY: all phase0 phase1 phase2 clean test help

# Default target
all: phase0

#==============================================================================
# PHASE 0 - PoC (Research & Proof of Concept)
#==============================================================================
.PHONY: phase0 phase0-server phase0-fuse phase0-cache-cli phase0-test phase0-run-server phase0-run-fuse

phase0: phase0-server phase0-fuse phase0-cache-cli

phase0-server:
	@echo "Building Phase 0 Server..."
	cd phase0 && go build -o ../bin/phase0-server ./cmd/server

phase0-fuse:
	@echo "Building Phase 0 FUSE Client..."
	cd phase0 && go build -o ../bin/phase0-fuse ./cmd/fuse-client

phase0-cache-cli:
	@echo "Building Phase 0 Cache CLI..."
	cd phase0 && go build -o ../bin/phase0-cache-cli ./cmd/cache-cli

phase0-test:
	@echo "Testing Phase 0..."
	cd phase0 && go test ./...

phase0-run-server:
	@echo "Running Phase 0 Server on :8080..."
	cd phase0 && go run ./cmd/server

phase0-run-fuse:
	@echo "Running Phase 0 FUSE Client..."
	@echo "Usage: sudo ./bin/phase0-fuse -mount /mnt/fruitsalade -server http://localhost:8080"
	cd phase0 && go run ./cmd/fuse-client

#==============================================================================
# PHASE 1 - MVP (Minimum Viable Product)
#==============================================================================
.PHONY: phase1 phase1-server phase1-fuse phase1-seed phase1-test phase1-docker
.PHONY: phase1-test-env phase1-test-env-down phase1-test-env-logs phase1-exec-a phase1-exec-b

phase1: phase1-server phase1-fuse

phase1-server:
	@echo "Building Phase 1 Server..."
	cd phase1 && go build -o ../bin/phase1-server ./cmd/server

phase1-fuse:
	@echo "Building Phase 1 FUSE Client..."
	cd phase1 && go build -o ../bin/phase1-fuse ./cmd/fuse-client

phase1-seed:
	@echo "Building Phase 1 Seed Tool..."
	cd phase1 && go build -o ../bin/phase1-seed ./cmd/seed-tool

phase1-test:
	@echo "Testing Phase 1..."
	cd phase1 && go test ./...

phase1-docker:
	@echo "Building Phase 1 Docker images..."
	docker build -t fruitsalade:phase1-server --target server -f phase1/docker/Dockerfile .
	docker build -t fruitsalade:phase1-seed --target seed -f phase1/docker/Dockerfile .
	docker build -t fruitsalade:phase1-fuse --target fuse-client -f phase1/docker/Dockerfile .

phase1-up:
	@echo "Starting Phase 1 services..."
	docker compose -f phase1/docker/docker-compose.yml up -d

phase1-down:
	docker compose -f phase1/docker/docker-compose.yml down

phase1-test-env:
	@echo "Starting Phase 1 test environment (postgres + minio + server + 2 clients)..."
	docker compose -f phase1/docker/docker-compose.yml up -d --build

phase1-test-env-down:
	@echo "Stopping Phase 1 test environment and removing volumes..."
	docker compose -f phase1/docker/docker-compose.yml down -v

phase1-test-env-logs:
	docker compose -f phase1/docker/docker-compose.yml logs -f

phase1-exec-a:
	docker compose -f phase1/docker/docker-compose.yml exec client-a sh

phase1-exec-b:
	docker compose -f phase1/docker/docker-compose.yml exec client-b sh

#==============================================================================
# PHASE 2 - Production (Full Features)
#==============================================================================
.PHONY: phase2 phase2-server phase2-fuse phase2-windows phase2-test

phase2: phase2-server phase2-fuse

phase2-server:
	@echo "Building Phase 2 Server..."
	cd phase2 && go build -o ../bin/phase2-server ./cmd/server

phase2-fuse:
	@echo "Building Phase 2 FUSE Client..."
	cd phase2 && go build -o ../bin/phase2-fuse ./cmd/fuse-client

phase2-windows:
	@echo "Building Phase 2 Windows Client..."
	@echo "Requires: Windows build environment with CGO"
	cd phase2 && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -o ../bin/phase2-windows.exe ./cmd/windows-client

phase2-test:
	@echo "Testing Phase 2..."
	cd phase2 && go test ./...

#==============================================================================
# SHARED
#==============================================================================
.PHONY: shared-test

shared-test:
	@echo "Testing Shared package..."
	cd shared && go test ./...

#==============================================================================
# UTILITIES
#==============================================================================
.PHONY: clean test lint fmt deps

clean:
	rm -rf bin/
	rm -rf phase0/bin phase1/bin phase2/bin

test: shared-test phase0-test phase1-test phase2-test

lint:
	@echo "Linting all phases..."
	cd shared && go vet ./...
	cd phase0 && go vet ./...
	cd phase1 && go vet ./...
	cd phase2 && go vet ./...

fmt:
	@echo "Formatting all code..."
	gofmt -s -w shared/ phase0/ phase1/ phase2/

deps:
	@echo "Downloading dependencies..."
	cd shared && go mod download
	cd phase0 && go mod download
	cd phase1 && go mod download
	cd phase2 && go mod download

#==============================================================================
# HELP
#==============================================================================
help:
	@echo "FruitSalade Build System"
	@echo ""
	@echo "Phase 0 (PoC):"
	@echo "  make phase0            Build server + FUSE client + cache-cli"
	@echo "  make phase0-server     Build server only"
	@echo "  make phase0-fuse       Build FUSE client only"
	@echo "  make phase0-cache-cli  Build cache CLI tool"
	@echo "  make phase0-test       Run Phase 0 tests"
	@echo "  make phase0-run-server Run server locally"
	@echo ""
	@echo "Phase 1 (MVP):"
	@echo "  make phase1              Build server + FUSE client"
	@echo "  make phase1-seed         Build seed tool"
	@echo "  make phase1-docker       Build all Docker images"
	@echo "  make phase1-up           Start Docker services"
	@echo "  make phase1-down         Stop Docker services"
	@echo "  make phase1-test         Run Phase 1 tests"
	@echo "  make phase1-test-env     Start full test env (build + up)"
	@echo "  make phase1-test-env-down Stop test env + remove volumes"
	@echo "  make phase1-test-env-logs Follow logs from test env"
	@echo "  make phase1-exec-a       Shell into client-a"
	@echo "  make phase1-exec-b       Shell into client-b"
	@echo ""
	@echo "Phase 2 (Production):"
	@echo "  make phase2            Build server + FUSE client"
	@echo "  make phase2-windows    Build Windows client (requires CGO)"
	@echo "  make phase2-test       Run Phase 2 tests"
	@echo ""
	@echo "Utilities:"
	@echo "  make test              Run all tests"
	@echo "  make lint              Lint all code"
	@echo "  make fmt               Format all code"
	@echo "  make clean             Remove build artifacts"
	@echo "  make deps              Download dependencies"
