-- Phase 2: File sharing - permissions and share links

CREATE TABLE IF NOT EXISTS file_permissions (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    path        TEXT NOT NULL,
    permission  TEXT NOT NULL DEFAULT 'read',  -- owner, read, write
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, path)
);

CREATE INDEX IF NOT EXISTS idx_file_permissions_path ON file_permissions (path);
CREATE INDEX IF NOT EXISTS idx_file_permissions_user ON file_permissions (user_id);

ALTER TABLE files ADD COLUMN IF NOT EXISTS owner_id INTEGER REFERENCES users(id);

CREATE TABLE IF NOT EXISTS share_links (
    id              TEXT PRIMARY KEY,           -- random token
    path            TEXT NOT NULL,
    created_by      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at      TIMESTAMPTZ,               -- NULL = no expiry
    password_hash   TEXT,                      -- NULL = no password (bcrypt)
    max_downloads   INTEGER NOT NULL DEFAULT 0, -- 0 = unlimited
    download_count  INTEGER NOT NULL DEFAULT 0,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_share_links_path ON share_links (path);
CREATE INDEX IF NOT EXISTS idx_share_links_created_by ON share_links (created_by);
