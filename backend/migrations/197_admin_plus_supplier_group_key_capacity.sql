ALTER TABLE admin_plus_supplier_groups
    ADD COLUMN IF NOT EXISTS key_limit_policy TEXT NOT NULL DEFAULT 'inherit',
    ADD COLUMN IF NOT EXISTS key_limit_value INTEGER NOT NULL DEFAULT 0;

ALTER TABLE admin_plus_supplier_groups
    DROP CONSTRAINT IF EXISTS admin_plus_supplier_groups_key_limit_policy_check;

ALTER TABLE admin_plus_supplier_groups
    ADD CONSTRAINT admin_plus_supplier_groups_key_limit_policy_check
    CHECK (key_limit_policy IN ('inherit', 'unknown', 'unlimited', 'limited', 'unsupported'));

ALTER TABLE admin_plus_supplier_groups
    DROP CONSTRAINT IF EXISTS admin_plus_supplier_groups_key_limit_value_check;

ALTER TABLE admin_plus_supplier_groups
    ADD CONSTRAINT admin_plus_supplier_groups_key_limit_value_check
    CHECK (key_limit_value >= 0);
