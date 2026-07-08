ALTER TABLE admin_plus_suppliers
    ADD COLUMN IF NOT EXISTS key_limit_policy TEXT NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS key_limit_value INTEGER NOT NULL DEFAULT 0;

ALTER TABLE admin_plus_suppliers
    DROP CONSTRAINT IF EXISTS admin_plus_suppliers_key_limit_policy_check;

ALTER TABLE admin_plus_suppliers
    ADD CONSTRAINT admin_plus_suppliers_key_limit_policy_check
    CHECK (key_limit_policy IN ('unknown', 'unlimited', 'limited', 'unsupported'));

ALTER TABLE admin_plus_suppliers
    DROP CONSTRAINT IF EXISTS admin_plus_suppliers_key_limit_value_check;

ALTER TABLE admin_plus_suppliers
    ADD CONSTRAINT admin_plus_suppliers_key_limit_value_check
    CHECK (key_limit_value >= 0);
