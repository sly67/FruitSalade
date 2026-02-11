-- 006_rbac down: Reverse nested groups, roles, visibility, group ownership

DROP TRIGGER IF EXISTS trg_check_group_cycle ON groups;
DROP FUNCTION IF EXISTS check_group_cycle();

DROP INDEX IF EXISTS idx_files_visibility;
DROP INDEX IF EXISTS idx_files_group_id;
ALTER TABLE files DROP COLUMN IF EXISTS group_id;
ALTER TABLE files DROP COLUMN IF EXISTS visibility;

ALTER TABLE group_members DROP COLUMN IF EXISTS role;

DROP INDEX IF EXISTS idx_groups_parent;
ALTER TABLE groups DROP COLUMN IF EXISTS parent_id;
