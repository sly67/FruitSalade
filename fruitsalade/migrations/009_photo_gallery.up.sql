-- 009: Photo gallery — EXIF metadata, tags, and auto-tagging plugins

CREATE TABLE IF NOT EXISTS image_metadata (
    id SERIAL PRIMARY KEY,
    file_path TEXT NOT NULL UNIQUE REFERENCES files(path) ON DELETE CASCADE,
    width INTEGER,
    height INTEGER,
    camera_make TEXT,
    camera_model TEXT,
    lens_model TEXT,
    focal_length REAL,
    aperture REAL,
    shutter_speed TEXT,
    iso INTEGER,
    flash BOOLEAN,
    date_taken TIMESTAMPTZ,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    altitude REAL,
    location_country TEXT,
    location_city TEXT,
    location_name TEXT,
    orientation INTEGER NOT NULL DEFAULT 1,
    has_thumbnail BOOLEAN NOT NULL DEFAULT FALSE,
    thumb_s3_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS image_tags (
    id SERIAL PRIMARY KEY,
    file_path TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    confidence REAL NOT NULL DEFAULT 1.0,
    source TEXT NOT NULL DEFAULT 'manual',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(file_path, tag, source)
);

CREATE TABLE IF NOT EXISTS tagging_plugins (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    webhook_url TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    config JSONB NOT NULL DEFAULT '{}',
    last_health TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for image_metadata
CREATE INDEX IF NOT EXISTS idx_image_metadata_date_taken ON image_metadata (date_taken DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_image_metadata_camera ON image_metadata (camera_make, camera_model);
CREATE INDEX IF NOT EXISTS idx_image_metadata_location ON image_metadata (location_country, location_city);
CREATE INDEX IF NOT EXISTS idx_image_metadata_status ON image_metadata (status);

-- Indexes for image_tags
CREATE INDEX IF NOT EXISTS idx_image_tags_tag ON image_tags (tag);
CREATE INDEX IF NOT EXISTS idx_image_tags_file_path ON image_tags (file_path);
