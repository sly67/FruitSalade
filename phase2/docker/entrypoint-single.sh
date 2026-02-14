#!/bin/sh
set -e

# Single-container entrypoint: PostgreSQL + FruitSalade server
#
# Environment variables:
#   POSTGRES_USER     (default: fruitsalade)
#   POSTGRES_PASSWORD (default: fruitsalade)
#   POSTGRES_DB       (default: fruitsalade)
#   SEED_DATA         (default: false) - run seed tool on first launch
#   JWT_SECRET        (required)
#   STORAGE_BACKEND   (default: local)
#   LOCAL_STORAGE_PATH (default: /data/storage)

PGDATA="/data/postgres"
PGUSER="${POSTGRES_USER:-fruitsalade}"
PGPASSWORD="${POSTGRES_PASSWORD:-fruitsalade}"
PGDB="${POSTGRES_DB:-fruitsalade}"
PGRUN="/run/postgresql"

# ---- 1. Initialize PostgreSQL if data directory is empty ----
if [ ! -f "$PGDATA/PG_VERSION" ]; then
    echo "[init] Initializing PostgreSQL data directory..."
    su-exec postgres initdb -D "$PGDATA" --auth=trust --encoding=UTF8 --locale=C

    # Configure pg_hba.conf for local + TCP connections
    cat > "$PGDATA/pg_hba.conf" <<EOF
local   all   all                 trust
host    all   all   127.0.0.1/32  md5
host    all   all   ::1/128       md5
EOF

    # Listen on localhost only (single container)
    sed -i "s/#listen_addresses = 'localhost'/listen_addresses = '127.0.0.1'/" "$PGDATA/postgresql.conf"
    sed -i "s|#unix_socket_directories = '/run/postgresql'|unix_socket_directories = '$PGRUN'|" "$PGDATA/postgresql.conf"
fi

# ---- 2. Start PostgreSQL in background ----
echo "[init] Starting PostgreSQL..."
su-exec postgres pg_ctl -D "$PGDATA" -l /data/postgres/logfile -o "-k $PGRUN" start

# ---- 3. Wait for PostgreSQL to be ready ----
echo "[init] Waiting for PostgreSQL..."
for i in $(seq 1 30); do
    if su-exec postgres pg_isready -h 127.0.0.1 -q 2>/dev/null; then
        echo "[init] PostgreSQL is ready."
        break
    fi
    if [ "$i" = "30" ]; then
        echo "[init] ERROR: PostgreSQL failed to start within 30 seconds."
        cat /data/postgres/logfile
        exit 1
    fi
    sleep 1
done

# ---- 4. Create database and user if not exists ----
su-exec postgres psql -h 127.0.0.1 -tc "SELECT 1 FROM pg_roles WHERE rolname = '$PGUSER'" | grep -q 1 || \
    su-exec postgres psql -h 127.0.0.1 -c "CREATE USER $PGUSER WITH PASSWORD '$PGPASSWORD';"

su-exec postgres psql -h 127.0.0.1 -tc "SELECT 1 FROM pg_database WHERE datname = '$PGDB'" | grep -q 1 || \
    su-exec postgres psql -h 127.0.0.1 -c "CREATE DATABASE $PGDB OWNER $PGUSER;"

# Grant privileges
su-exec postgres psql -h 127.0.0.1 -c "GRANT ALL PRIVILEGES ON DATABASE $PGDB TO $PGUSER;"

# ---- 5. Build DATABASE_URL if not set ----
export DATABASE_URL="${DATABASE_URL:-postgres://$PGUSER:$PGPASSWORD@127.0.0.1:5432/$PGDB?sslmode=disable}"

# ---- 6. Optionally run seed tool ----
if [ "${SEED_DATA}" = "true" ] && [ -d /app/testdata ]; then
    echo "[init] Running seed tool..."
    /app/seed-tool -data /app/testdata -migrations /app/migrations || true
    echo "[init] Seed tool finished."
fi

# ---- 7. Trap signals for graceful shutdown ----
cleanup() {
    echo "[shutdown] Stopping server..."
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
    echo "[shutdown] Stopping PostgreSQL..."
    su-exec postgres pg_ctl -D "$PGDATA" stop -m fast
    echo "[shutdown] Done."
    exit 0
}
trap cleanup TERM INT

# ---- 8. Start FruitSalade server ----
echo "[init] Starting FruitSalade server..."
/app/server &
SERVER_PID=$!

# Wait for server process (forward signals)
wait "$SERVER_PID"
