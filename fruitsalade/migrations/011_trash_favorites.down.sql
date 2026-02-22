-- Rollback migration 011: Trash and Favorites

DROP TABLE IF EXISTS user_favorites;
DROP INDEX IF EXISTS idx_files_deleted_at;
ALTER TABLE files DROP COLUMN IF EXISTS original_path;
ALTER TABLE files DROP COLUMN IF EXISTS deleted_by;
ALTER TABLE files DROP COLUMN IF EXISTS deleted_at;
