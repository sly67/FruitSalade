-- 006_rbac: Nested groups, per-group roles, file visibility, group ownership

-- 1. Nest groups: parent_id = NULL means top-level (org)
ALTER TABLE groups ADD COLUMN IF NOT EXISTS parent_id INTEGER REFERENCES groups(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_groups_parent ON groups (parent_id);

-- 2. Per-group roles on membership (existing rows get 'viewer')
ALTER TABLE group_members ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'viewer';

-- 3. Visibility on files (existing rows get 'public' = unchanged behavior)
ALTER TABLE files ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'public';

-- 4. Group ownership on files (for "group" visibility scope)
ALTER TABLE files ADD COLUMN IF NOT EXISTS group_id INTEGER REFERENCES groups(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_files_group_id ON files (group_id);
CREATE INDEX IF NOT EXISTS idx_files_visibility ON files (visibility);

-- 5. Cycle prevention trigger for group nesting
CREATE OR REPLACE FUNCTION check_group_cycle()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.parent_id IS NOT NULL THEN
        IF EXISTS (
            WITH RECURSIVE descendants AS (
                SELECT id FROM groups WHERE id = NEW.id
                UNION ALL
                SELECT g.id FROM groups g JOIN descendants d ON g.parent_id = d.id
            )
            SELECT 1 FROM descendants WHERE id = NEW.parent_id
        ) THEN
            RAISE EXCEPTION 'circular group nesting detected';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_check_group_cycle ON groups;
CREATE TRIGGER trg_check_group_cycle
    BEFORE INSERT OR UPDATE OF parent_id ON groups
    FOR EACH ROW EXECUTE FUNCTION check_group_cycle();
