-- Phase 2: Rate limiting and user quotas

CREATE TABLE IF NOT EXISTS user_quotas (
    user_id                 INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    max_storage_bytes       BIGINT NOT NULL DEFAULT 0,       -- 0 = unlimited
    max_bandwidth_per_day   BIGINT NOT NULL DEFAULT 0,       -- 0 = unlimited
    max_requests_per_minute INTEGER NOT NULL DEFAULT 0,      -- 0 = unlimited
    max_upload_size_bytes   BIGINT NOT NULL DEFAULT 0,       -- 0 = use global default
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bandwidth_usage (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date        DATE NOT NULL DEFAULT CURRENT_DATE,
    bytes_in    BIGINT NOT NULL DEFAULT 0,
    bytes_out   BIGINT NOT NULL DEFAULT 0,
    UNIQUE(user_id, date)
);

CREATE INDEX IF NOT EXISTS idx_bandwidth_usage_user_date ON bandwidth_usage (user_id, date);
