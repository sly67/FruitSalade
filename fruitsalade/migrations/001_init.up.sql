-- Phase 1: Initial schema for FruitSalade metadata
--
-- Stores file/directory metadata. Content is stored in S3.
-- The server queries this to build the metadata tree for clients.

CREATE TABLE IF NOT EXISTS files (
    id          TEXT PRIMARY KEY,                -- unique file ID (SHA256 of path)
    name        TEXT NOT NULL,                   -- filename
    path        TEXT NOT NULL UNIQUE,            -- full path (e.g., /docs/readme.md)
    parent_path TEXT NOT NULL DEFAULT '/',       -- parent directory path
    size        BIGINT NOT NULL DEFAULT 0,       -- file size in bytes
    mod_time    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_dir      BOOLEAN NOT NULL DEFAULT FALSE,
    hash        TEXT NOT NULL DEFAULT '',        -- SHA256 content hash
    s3_key      TEXT NOT NULL DEFAULT '',        -- S3 object key
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for directory listing (list children of a directory)
CREATE INDEX IF NOT EXISTS idx_files_parent_path ON files (parent_path);

-- Index for path lookups
CREATE INDEX IF NOT EXISTS idx_files_path ON files (path);

-- Users table for JWT auth
CREATE TABLE IF NOT EXISTS users (
    id          SERIAL PRIMARY KEY,
    username    TEXT NOT NULL UNIQUE,
    password    TEXT NOT NULL,                   -- bcrypt hash
    is_admin    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Device tokens for per-device auth
CREATE TABLE IF NOT EXISTS device_tokens (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_name TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,            -- SHA256 of the JWT
    last_used   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked     BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_device_tokens_user_id ON device_tokens (user_id);
