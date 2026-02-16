.PHONY: all clean test help

GO ?= $(shell which go 2>/dev/null || echo /usr/local/go/bin/go)

# Default target
all: server fuse

#==============================================================================
# BUILD
#==============================================================================
.PHONY: server fuse seed winclient windows

server:
	@echo "Building Server..."
	cd fruitsalade && $(GO) build -ldflags="-s -w" -trimpath -o ../bin/server ./cmd/server

fuse:
	@echo "Building FUSE Client..."
	cd fruitsalade && $(GO) build -ldflags="-s -w" -trimpath -o ../bin/fuse-client ./cmd/fuse-client

seed:
	@echo "Building Seed Tool..."
	cd fruitsalade && $(GO) build -ldflags="-s -w" -trimpath -o ../bin/seed-tool ./cmd/seed-tool

winclient:
	@echo "Building Windows Client (cgofuse, native)..."
	cd fruitsalade && $(GO) build -ldflags="-s -w" -trimpath -o ../bin/winclient ./cmd/windows-client

windows:
	@echo "Building Windows Client (cross-compile for Windows)..."
	@echo "Requires: Windows build environment with CGO"
	cd fruitsalade && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 $(GO) build -ldflags="-s -w" -trimpath -o ../bin/windows-client.exe ./cmd/windows-client

#==============================================================================
# TEST
#==============================================================================
.PHONY: test test-shared test-app

test: test-shared test-app

test-shared:
	@echo "Testing Shared package..."
	cd shared && $(GO) test ./...

test-app:
	@echo "Testing FruitSalade..."
	cd fruitsalade && $(GO) test ./...

#==============================================================================
# DOCKER
#==============================================================================
.PHONY: docker docker-up docker-down docker-logs docker-run docker-deploy exec-a exec-b exec-server

docker:
	@echo "Building Docker images..."
	docker build -t fruitsalade:server --target server -f fruitsalade/docker/Dockerfile .
	docker build -t fruitsalade:client --target client -f fruitsalade/docker/Dockerfile .

docker-up:
	@echo "Starting environment (server + minio + 2 clients)..."
	docker compose -f fruitsalade/docker/docker-compose.yml up -d --build

docker-down:
	@echo "Stopping environment and removing volumes..."
	docker compose -f fruitsalade/docker/docker-compose.yml down -v

docker-logs:
	docker compose -f fruitsalade/docker/docker-compose.yml logs -f

docker-run:
	@echo "Running server standalone (local storage, no S3)..."
	docker run --rm -p 48000:8080 -p 48001:9090 \
		-e JWT_SECRET=change-me-in-production \
		-e SEED_DATA=true \
		-v fruitsalade_pg:/data/postgres \
		-v fruitsalade_storage:/data/storage \
		fruitsalade:server

COMPOSE=docker compose -f fruitsalade/docker/docker-compose.yml
SERVER_CONTAINER=$$($(COMPOSE) ps -q server)

docker-deploy:
	@echo "Building server binary (static, for container)..."
	cd fruitsalade && CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -trimpath -o ../bin/server ./cmd/server
	@echo "Copying binary into running container..."
	docker cp bin/server $(SERVER_CONTAINER):/app/server
	@echo "Copying migrations..."
	docker cp fruitsalade/migrations/. $(SERVER_CONTAINER):/app/migrations/
	@echo "Restarting server..."
	$(COMPOSE) restart server
	@echo "Deploy complete — no image rebuild."

exec-server:
	docker compose -f fruitsalade/docker/docker-compose.yml exec server sh

exec-a:
	docker compose -f fruitsalade/docker/docker-compose.yml exec client-a sh

exec-b:
	docker compose -f fruitsalade/docker/docker-compose.yml exec client-b sh

#==============================================================================
# UTILITIES
#==============================================================================
.PHONY: lint fmt deps

clean:
	rm -rf bin/
	rm -rf fruitsalade/bin

lint:
	@echo "Linting..."
	cd shared && $(GO) vet ./...
	cd fruitsalade && $(GO) vet ./...

fmt:
	@echo "Formatting..."
	gofmt -s -w shared/ fruitsalade/

deps:
	@echo "Downloading dependencies..."
	cd shared && $(GO) mod download
	cd fruitsalade && $(GO) mod download

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
	@echo "  make test-app        Run app tests"
	@echo ""
	@echo "Docker:"
	@echo "  make docker          Build server + client Docker images"
	@echo "  make docker-up       Start full env (server + minio + 2 clients)"
	@echo "  make docker-down     Stop env + remove volumes"
	@echo "  make docker-logs     Follow logs"
	@echo "  make docker-run      Run server standalone (local storage, no S3)"
	@echo "  make docker-deploy   Build + copy binary into running container (no rebuild)"
	@echo "  make exec-server     Shell into server"
	@echo "  make exec-a          Shell into client-a"
	@echo "  make exec-b          Shell into client-b"
	@echo ""
	@echo "Utilities:"
	@echo "  make lint            Lint all code"
	@echo "  make fmt             Format all code"
	@echo "  make clean           Remove build artifacts"
	@echo "  make deps            Download dependencies"
