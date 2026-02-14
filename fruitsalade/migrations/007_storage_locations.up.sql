-- Storage locations: configurable per-group storage backends
CREATE TABLE IF NOT EXISTS storage_locations (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    group_id        INTEGER REFERENCES groups(id) ON DELETE SET NULL,
    backend_type    TEXT NOT NULL CHECK (backend_type IN ('s3', 'local', 'smb')),
    config          JSONB NOT NULL DEFAULT '{}',
    priority        INTEGER NOT NULL DEFAULT 0,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Only one default allowed
CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_locations_default
    ON storage_locations (is_default) WHERE is_default = TRUE;

-- Link files to their storage location (NULL = use default)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'files' AND column_name = 'storage_location_id'
    ) THEN
        ALTER TABLE files ADD COLUMN storage_location_id INTEGER
            REFERENCES storage_locations(id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'file_versions' AND column_name = 'storage_location_id'
    ) THEN
        ALTER TABLE file_versions ADD COLUMN storage_location_id INTEGER
            REFERENCES storage_locations(id) ON DELETE SET NULL;
    END IF;
END $$;
