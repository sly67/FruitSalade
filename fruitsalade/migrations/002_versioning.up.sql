-- Phase 2: File versioning support
-- Tracks version history for files, enabling rollback to previous versions.

-- Add version column to files table
ALTER TABLE files ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1;

-- Version history table
CREATE TABLE IF NOT EXISTS file_versions (
    id          SERIAL PRIMARY KEY,
    file_id     TEXT NOT NULL,              -- references files.id at time of version
    path        TEXT NOT NULL,              -- file path at time of version
    version     INT NOT NULL,               -- version number
    size        BIGINT NOT NULL DEFAULT 0,
    hash        TEXT NOT NULL DEFAULT '',
    s3_key      TEXT NOT NULL DEFAULT '',   -- S3 key for this version's content
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(path, version)
);

CREATE INDEX IF NOT EXISTS idx_file_versions_path ON file_versions (path);
