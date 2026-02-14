#!/bin/sh
set -e

SERVER_URL="${SERVER_URL:-http://server:8080}"
MOUNT_POINT="${MOUNT_POINT:-/mnt/fruitsalade}"
CACHE_DIR="${CACHE_DIR:-/var/cache/fruitsalade}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin}"

echo "FruitSalade Phase 2 client starting (read/write)..."
echo "  Server: $SERVER_URL"
echo "  Mount:  $MOUNT_POINT"
echo "  Cache:  $CACHE_DIR"

mkdir -p "$MOUNT_POINT" "$CACHE_DIR"

# Wait for server to be healthy
echo "Waiting for server..."
until curl -sf "$SERVER_URL/health" > /dev/null 2>&1; do
    sleep 2
done
echo "Server is healthy."

# Authenticate and get JWT token
echo "Authenticating as $USERNAME..."
RESPONSE=$(curl -sf -X POST "$SERVER_URL/api/v1/auth/token" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\",\"device_name\":\"$(hostname)\"}")

TOKEN=$(echo "$RESPONSE" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')

if [ -z "$TOKEN" ]; then
    echo "ERROR: Failed to get auth token"
    echo "Response: $RESPONSE"
    exit 1
fi
echo "Authenticated successfully."

# Run Phase 2 FUSE client (foreground, read/write)
exec /app/fuse-client \
    -mount "$MOUNT_POINT" \
    -server "$SERVER_URL" \
    -cache "$CACHE_DIR" \
    -token "$TOKEN" \
    -watch \
    -v 2
