-- Performance indexes for common query patterns

-- Quota: SUM(size) WHERE owner_id = $1
CREATE INDEX IF NOT EXISTS idx_files_owner_id ON files (owner_id) WHERE owner_id IS NOT NULL;

-- Visibility/group filtering in tree endpoints
CREATE INDEX IF NOT EXISTS idx_files_group_id ON files (group_id) WHERE group_id IS NOT NULL;

-- Permission inheritance queries: WHERE user_id = $1 AND path = ANY($2)
CREATE INDEX IF NOT EXISTS idx_file_permissions_user_path ON file_permissions (user_id, path);

-- Group membership lookups by user
CREATE INDEX IF NOT EXISTS idx_group_members_user_id ON group_members (user_id);
