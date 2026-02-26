-- Chunked upload tracking for resumable large file uploads
CREATE TABLE IF NOT EXISTS chunked_uploads (
    id           TEXT PRIMARY KEY,
    user_id      INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    path         TEXT NOT NULL,
    file_name    TEXT NOT NULL,
    file_size    BIGINT NOT NULL,
    chunk_size   INT NOT NULL DEFAULT 5242880,
    total_chunks INT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS upload_chunks (
    upload_id   TEXT NOT NULL REFERENCES chunked_uploads(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    size        INT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (upload_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_chunked_uploads_user ON chunked_uploads(user_id);
CREATE INDEX IF NOT EXISTS idx_chunked_uploads_expires ON chunked_uploads(expires_at) WHERE status = 'active';
