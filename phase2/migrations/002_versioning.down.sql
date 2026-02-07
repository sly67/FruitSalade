-- Rollback versioning support
DROP TABLE IF EXISTS file_versions;
ALTER TABLE files DROP COLUMN IF EXISTS version;
