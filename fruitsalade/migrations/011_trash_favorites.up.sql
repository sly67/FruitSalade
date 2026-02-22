-- Migration 011: Trash (soft-delete) and Favorites

-- Soft-delete columns on files table
ALTER TABLE files ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE files ADD COLUMN IF NOT EXISTS deleted_by INTEGER REFERENCES users(id);
ALTER TABLE files ADD COLUMN IF NOT EXISTS original_path TEXT;
CREATE INDEX IF NOT EXISTS idx_files_deleted_at ON files (deleted_at) WHERE deleted_at IS NOT NULL;

-- User favorites
CREATE TABLE IF NOT EXISTS user_favorites (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, file_path)
);
CREATE INDEX IF NOT EXISTS idx_user_favorites_user ON user_favorites (user_id);
