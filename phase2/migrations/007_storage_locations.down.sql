ALTER TABLE file_versions DROP COLUMN IF EXISTS storage_location_id;
ALTER TABLE files DROP COLUMN IF EXISTS storage_location_id;
DROP INDEX IF EXISTS idx_storage_locations_default;
DROP TABLE IF EXISTS storage_locations;
