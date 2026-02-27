DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'storage_locations' AND column_name = 'read_only'
    ) THEN
        ALTER TABLE storage_locations ADD COLUMN read_only BOOLEAN NOT NULL DEFAULT FALSE;
    END IF;
END $$;
