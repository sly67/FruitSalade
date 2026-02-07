#!/bin/bash
# Phase 0 End-to-End Test Script
#
# This script tests the full Phase 0 flow:
# 1. Starts the server
# 2. Mounts the FUSE client
# 3. Runs test commands
# 4. Cleans up

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
MOUNT_POINT="/tmp/fruitsalade-test"
SERVER_PID=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"

    # Unmount if mounted
    if mountpoint -q "$MOUNT_POINT" 2>/dev/null; then
        echo "Unmounting $MOUNT_POINT"
        fusermount -u "$MOUNT_POINT" 2>/dev/null || sudo umount "$MOUNT_POINT" 2>/dev/null || true
    fi

    # Kill server
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        echo "Stopping server (PID $SERVER_PID)"
        kill "$SERVER_PID" 2>/dev/null || true
    fi

    # Remove mount point
    rmdir "$MOUNT_POINT" 2>/dev/null || true

    echo -e "${GREEN}Cleanup complete${NC}"
}

trap cleanup EXIT

echo "╔═══════════════════════════════════════╗"
echo "║   FruitSalade Phase 0 E2E Test        ║"
echo "╚═══════════════════════════════════════╝"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    echo "Install Go from https://go.dev/dl/"
    exit 1
fi

# Build
echo -e "${YELLOW}Building...${NC}"
cd "$PROJECT_DIR"
make phase0

# Start server
echo -e "\n${YELLOW}Starting server...${NC}"
./bin/phase0-server -data ./phase0/testdata &
SERVER_PID=$!
sleep 2

# Check server is running
if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo -e "${RED}Server failed to start${NC}"
    exit 1
fi
echo -e "${GREEN}Server running (PID $SERVER_PID)${NC}"

# Test server endpoints
echo -e "\n${YELLOW}Testing server...${NC}"
echo "GET /health"
curl -s http://localhost:8080/health | head -c 100
echo ""

echo "GET /api/v1/tree"
curl -s http://localhost:8080/api/v1/tree | head -c 200
echo "..."

# Mount FUSE
echo -e "\n${YELLOW}Mounting FUSE client...${NC}"
mkdir -p "$MOUNT_POINT"
./bin/phase0-fuse -mount "$MOUNT_POINT" -server http://localhost:8080 &
FUSE_PID=$!
sleep 2

if ! mountpoint -q "$MOUNT_POINT" 2>/dev/null; then
    echo -e "${RED}FUSE mount failed${NC}"
    echo "Make sure you have FUSE installed: sudo apt install fuse3"
    exit 1
fi
echo -e "${GREEN}Mounted at $MOUNT_POINT${NC}"

# Test FUSE
echo -e "\n${YELLOW}Testing FUSE mount...${NC}"

echo "ls $MOUNT_POINT:"
ls -la "$MOUNT_POINT"

echo -e "\ncat hello.txt (first read - should fetch from server):"
cat "$MOUNT_POINT/hello.txt"

echo -e "\ncat hello.txt (second read - should be cached):"
cat "$MOUNT_POINT/hello.txt"

echo -e "\nls subdir:"
ls -la "$MOUNT_POINT/subdir" 2>/dev/null || echo "(no subdir)"

echo -e "\n${GREEN}═══════════════════════════════════════${NC}"
echo -e "${GREEN}All tests passed!${NC}"
echo -e "${GREEN}═══════════════════════════════════════${NC}"
echo ""
echo "The FUSE mount is still active at $MOUNT_POINT"
echo "Press Enter to unmount and exit..."
read
