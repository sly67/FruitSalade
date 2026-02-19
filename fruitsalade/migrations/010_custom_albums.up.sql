CREATE TABLE IF NOT EXISTS user_albums (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    cover_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, name)
);

CREATE TABLE IF NOT EXISTS album_images (
    album_id INTEGER NOT NULL REFERENCES user_albums(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (album_id, file_path)
);

CREATE INDEX IF NOT EXISTS idx_album_images_file_path ON album_images (file_path);
CREATE INDEX IF NOT EXISTS idx_user_albums_user_id ON user_albums (user_id);
