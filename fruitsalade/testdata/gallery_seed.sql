-- Gallery seed data: tags, albums, and album images.
-- Run after file seeding so that files(path) rows exist for FK references.
-- image_metadata rows are created by the gallery processor on upload;
-- this script seeds tags and custom albums on top.

-- ─── Admin User ──────────────────────────────────────────────────────────────
-- Create default admin user (admin/admin) so album FKs resolve.
-- password is bcrypt hash of "admin". ON CONFLICT avoids duplicate if server already created it.
INSERT INTO users (username, password, is_admin)
VALUES ('admin', '$2a$10$C6WFhHkXYlxY6.2gZkeLv.dZ3fG7kfRO4QyPXpekQBvDXi0u6rCbe', TRUE)
ON CONFLICT (username) DO NOTHING;

-- ─── Image Tags ──────────────────────────────────────────────────────────────
-- source='seed' distinguishes from manual or plugin tags.

-- Paris Vacation
INSERT INTO image_tags (file_path, tag, confidence, source) VALUES
  ('/photos/paris_01.jpg', 'travel',      1.0, 'seed'),
  ('/photos/paris_01.jpg', 'architecture', 1.0, 'seed'),
  ('/photos/paris_01.jpg', 'outdoor',     1.0, 'seed'),
  ('/photos/paris_01.jpg', 'cityscape',   0.9, 'seed'),
  ('/photos/paris_02.jpg', 'travel',      1.0, 'seed'),
  ('/photos/paris_02.jpg', 'architecture', 1.0, 'seed'),
  ('/photos/paris_02.jpg', 'outdoor',     1.0, 'seed'),
  ('/photos/paris_03.jpg', 'travel',      1.0, 'seed'),
  ('/photos/paris_03.jpg', 'street',      0.9, 'seed'),
  ('/photos/paris_03.jpg', 'outdoor',     1.0, 'seed'),
  ('/photos/paris_04.jpg', 'travel',      1.0, 'seed'),
  ('/photos/paris_04.jpg', 'cityscape',   1.0, 'seed'),
  ('/photos/paris_04.jpg', 'sunset',      0.8, 'seed'),
  ('/photos/paris_04.jpg', 'outdoor',     1.0, 'seed'),
  ('/photos/paris_05.jpg', 'travel',      1.0, 'seed'),
  ('/photos/paris_05.jpg', 'night',       1.0, 'seed'),
  ('/photos/paris_05.jpg', 'cityscape',   0.9, 'seed'),
  ('/photos/paris_05.jpg', 'outdoor',     1.0, 'seed')
ON CONFLICT (file_path, tag, source) DO NOTHING;

-- Tokyo Trip
INSERT INTO image_tags (file_path, tag, confidence, source) VALUES
  ('/photos/tokyo_01.jpg', 'travel',      1.0, 'seed'),
  ('/photos/tokyo_01.jpg', 'cityscape',   1.0, 'seed'),
  ('/photos/tokyo_01.jpg', 'outdoor',     1.0, 'seed'),
  ('/photos/tokyo_02.jpg', 'travel',      1.0, 'seed'),
  ('/photos/tokyo_02.jpg', 'street',      0.9, 'seed'),
  ('/photos/tokyo_02.jpg', 'food',        0.7, 'seed'),
  ('/photos/tokyo_02.jpg', 'outdoor',     1.0, 'seed'),
  ('/photos/tokyo_03.jpg', 'travel',      1.0, 'seed'),
  ('/photos/tokyo_03.jpg', 'portrait',    0.9, 'seed'),
  ('/photos/tokyo_03.jpg', 'street',      0.8, 'seed'),
  ('/photos/tokyo_04.jpg', 'travel',      1.0, 'seed'),
  ('/photos/tokyo_04.jpg', 'night',       1.0, 'seed'),
  ('/photos/tokyo_04.jpg', 'indoor',      0.8, 'seed')
ON CONFLICT (file_path, tag, source) DO NOTHING;

-- Nature Hike
INSERT INTO image_tags (file_path, tag, confidence, source) VALUES
  ('/photos/nature_01.jpg', 'landscape',  1.0, 'seed'),
  ('/photos/nature_01.jpg', 'nature',     1.0, 'seed'),
  ('/photos/nature_01.jpg', 'outdoor',    1.0, 'seed'),
  ('/photos/nature_02.jpg', 'landscape',  1.0, 'seed'),
  ('/photos/nature_02.jpg', 'nature',     1.0, 'seed'),
  ('/photos/nature_02.jpg', 'outdoor',    1.0, 'seed'),
  ('/photos/nature_02.jpg', 'flowers',    0.7, 'seed'),
  ('/photos/nature_03.jpg', 'nature',     1.0, 'seed'),
  ('/photos/nature_03.jpg', 'wildlife',   1.0, 'seed'),
  ('/photos/nature_03.jpg', 'outdoor',    1.0, 'seed'),
  ('/photos/nature_04.jpg', 'landscape',  1.0, 'seed'),
  ('/photos/nature_04.jpg', 'nature',     1.0, 'seed'),
  ('/photos/nature_04.jpg', 'sunset',     0.9, 'seed'),
  ('/photos/nature_04.jpg', 'outdoor',    1.0, 'seed')
ON CONFLICT (file_path, tag, source) DO NOTHING;

-- Family Portraits
INSERT INTO image_tags (file_path, tag, confidence, source) VALUES
  ('/photos/family_01.jpg', 'portrait',   1.0, 'seed'),
  ('/photos/family_01.jpg', 'outdoor',    1.0, 'seed'),
  ('/photos/family_02.jpg', 'portrait',   1.0, 'seed'),
  ('/photos/family_02.jpg', 'indoor',     0.9, 'seed'),
  ('/photos/family_02.jpg', 'night',      0.7, 'seed'),
  ('/photos/family_03.jpg', 'portrait',   1.0, 'seed'),
  ('/photos/family_03.jpg', 'outdoor',    1.0, 'seed'),
  ('/photos/family_03.jpg', 'landscape',  0.6, 'seed')
ON CONFLICT (file_path, tag, source) DO NOTHING;

-- Macro/Studio
INSERT INTO image_tags (file_path, tag, confidence, source) VALUES
  ('/photos/macro_01.jpg', 'macro',       1.0, 'seed'),
  ('/photos/macro_01.jpg', 'flowers',     0.9, 'seed'),
  ('/photos/macro_01.jpg', 'indoor',      1.0, 'seed'),
  ('/photos/macro_02.jpg', 'macro',       1.0, 'seed'),
  ('/photos/macro_02.jpg', 'nature',      0.8, 'seed'),
  ('/photos/macro_02.jpg', 'indoor',      1.0, 'seed'),
  ('/photos/macro_03.jpg', 'macro',       1.0, 'seed'),
  ('/photos/macro_03.jpg', 'indoor',      1.0, 'seed'),
  ('/photos/macro_04.jpg', 'macro',       1.0, 'seed'),
  ('/photos/macro_04.jpg', 'flowers',     0.8, 'seed'),
  ('/photos/macro_04.jpg', 'indoor',      1.0, 'seed')
ON CONFLICT (file_path, tag, source) DO NOTHING;

-- ─── Custom Albums ───────────────────────────────────────────────────────────
-- user_id=1 is the default admin user created by EnsureDefaultAdmin.

INSERT INTO user_albums (user_id, name, description, cover_path) VALUES
  (1, 'Paris 2024',       'Our trip to Paris, June 2024',       '/photos/paris_01.jpg'),
  (1, 'Tokyo 2024',       'Tokyo adventures, September 2024',   '/photos/tokyo_01.jpg'),
  (1, 'Best Landscapes',  'Favorite landscape shots',           '/photos/nature_01.jpg'),
  (1, 'Family',           'Family moments',                     '/photos/family_01.jpg');

-- ─── Album Images ────────────────────────────────────────────────────────────
-- Paris 2024 album (id will be assigned sequentially)
INSERT INTO album_images (album_id, file_path)
SELECT a.id, p.path FROM user_albums a, (VALUES
  ('/photos/paris_01.jpg'), ('/photos/paris_02.jpg'), ('/photos/paris_03.jpg'),
  ('/photos/paris_04.jpg'), ('/photos/paris_05.jpg')
) AS p(path) WHERE a.name = 'Paris 2024' AND a.user_id = 1
ON CONFLICT DO NOTHING;

-- Tokyo 2024 album
INSERT INTO album_images (album_id, file_path)
SELECT a.id, p.path FROM user_albums a, (VALUES
  ('/photos/tokyo_01.jpg'), ('/photos/tokyo_02.jpg'), ('/photos/tokyo_03.jpg'),
  ('/photos/tokyo_04.jpg')
) AS p(path) WHERE a.name = 'Tokyo 2024' AND a.user_id = 1
ON CONFLICT DO NOTHING;

-- Best Landscapes album (cross-set: nature + some Paris/Tokyo)
INSERT INTO album_images (album_id, file_path)
SELECT a.id, p.path FROM user_albums a, (VALUES
  ('/photos/nature_01.jpg'), ('/photos/nature_02.jpg'), ('/photos/nature_04.jpg'),
  ('/photos/paris_04.jpg'), ('/photos/tokyo_01.jpg')
) AS p(path) WHERE a.name = 'Best Landscapes' AND a.user_id = 1
ON CONFLICT DO NOTHING;

-- Family album
INSERT INTO album_images (album_id, file_path)
SELECT a.id, p.path FROM user_albums a, (VALUES
  ('/photos/family_01.jpg'), ('/photos/family_02.jpg'), ('/photos/family_03.jpg')
) AS p(path) WHERE a.name = 'Family' AND a.user_id = 1
ON CONFLICT DO NOTHING;
