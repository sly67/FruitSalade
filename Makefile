.PHONY: all clean test help

# Default target
all: server fuse

#==============================================================================
# BUILD
#==============================================================================
.PHONY: server fuse seed winclient windows

server:
	@echo "Building Server..."
	cd phase2 && go build -o ../bin/server ./cmd/server

fuse:
	@echo "Building FUSE Client..."
	cd phase2 && go build -o ../bin/fuse-client ./cmd/fuse-client

seed:
	@echo "Building Seed Tool..."
	cd phase2 && go build -o ../bin/seed-tool ./cmd/seed-tool

winclient:
	@echo "Building Windows Client (cgofuse, native)..."
	cd phase2 && go build -o ../bin/winclient ./cmd/windows-client

windows:
	@echo "Building Windows Client (cross-compile for Windows)..."
	@echo "Requires: Windows build environment with CGO"
	cd phase2 && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -o ../bin/windows-client.exe ./cmd/windows-client

#==============================================================================
# TEST
#==============================================================================
.PHONY: test test-shared test-phase2

test: test-shared test-phase2

test-shared:
	@echo "Testing Shared package..."
	cd shared && go test ./...

test-phase2:
	@echo "Testing Phase 2..."
	cd phase2 && go test ./...

#==============================================================================
# DOCKER - Multi-container (S3 backend)
#==============================================================================
.PHONY: docker test-env test-env-down test-env-logs exec-a exec-b

docker:
	@echo "Building Docker images..."
	docker build -t fruitsalade:server --target server -f phase2/docker/Dockerfile .
	docker build -t fruitsalade:seed --target seed -f phase2/docker/Dockerfile .
	docker build -t fruitsalade:fuse --target fuse-client -f phase2/docker/Dockerfile .

test-env:
	@echo "Starting test environment (postgres + minio + server + 2 clients)..."
	docker compose -f phase2/docker/docker-compose.yml up -d --build

test-env-down:
	@echo "Stopping test environment and removing volumes..."
	docker compose -f phase2/docker/docker-compose.yml down -v

test-env-logs:
	docker compose -f phase2/docker/docker-compose.yml logs -f

exec-a:
	docker compose -f phase2/docker/docker-compose.yml exec client-a sh

exec-b:
	docker compose -f phase2/docker/docker-compose.yml exec client-b sh

#==============================================================================
# DOCKER - Single container (local storage)
#==============================================================================
.PHONY: single single-up single-down single-run

single:
	@echo "Building single-container Docker image..."
	docker build -t fruitsalade:single -f phase2/docker/Dockerfile.single .

single-up:
	@echo "Starting single-container deployment..."
	docker compose -f phase2/docker/docker-compose.single.yml up -d --build

single-down:
	@echo "Stopping single-container deployment..."
	docker compose -f phase2/docker/docker-compose.single.yml down

single-run:
	@echo "Running single container (docker run)..."
	docker run --rm -p 8080:8080 -p 9090:9090 \
		-e JWT_SECRET=change-me-in-production \
		-e SEED_DATA=true \
		-v fruitsalade_pg:/data/postgres \
		-v fruitsalade_storage:/data/storage \
		fruitsalade:single

#==============================================================================
# UTILITIES
#==============================================================================
.PHONY: lint fmt deps

clean:
	rm -rf bin/
	rm -rf phase2/bin

lint:
	@echo "Linting..."
	cd shared && go vet ./...
	cd phase2 && go vet ./...

fmt:
	@echo "Formatting..."
	gofmt -s -w shared/ phase2/

deps:
	@echo "Downloading dependencies..."
	cd shared && go mod download
	cd phase2 && go mod download

#==============================================================================
# HELP
#==============================================================================
help:
	@echo "FruitSalade Build System"
	@echo ""
	@echo "Build:"
	@echo "  make server          Build server"
	@echo "  make fuse            Build FUSE client"
	@echo "  make seed            Build seed tool"
	@echo "  make winclient       Build Windows client (native, cgofuse)"
	@echo "  make windows         Cross-compile Windows client (requires CGO)"
	@echo ""
	@echo "Test:"
	@echo "  make test            Run all tests"
	@echo "  make test-shared     Run shared package tests"
	@echo "  make test-phase2     Run Phase 2 tests"
	@echo ""
	@echo "Docker (multi-container, S3 backend):"
	@echo "  make docker          Build all Docker images"
	@echo "  make test-env        Start full test env (build + up)"
	@echo "  make test-env-down   Stop test env + remove volumes"
	@echo "  make test-env-logs   Follow logs from test env"
	@echo "  make exec-a          Shell into client-a"
	@echo "  make exec-b          Shell into client-b"
	@echo ""
	@echo "Docker (single container, local storage):"
	@echo "  make single          Build single-container Docker image"
	@echo "  make single-up       Start single-container (compose)"
	@echo "  make single-down     Stop single-container"
	@echo "  make single-run      Run single container (docker run)"
	@echo ""
	@echo "Utilities:"
	@echo "  make test            Run all tests"
	@echo "  make lint            Lint all code"
	@echo "  make fmt             Format all code"
	@echo "  make clean           Remove build artifacts"
	@echo "  make deps            Download dependencies"
