ALTER TABLE admin_plus_supplier_groups
    ADD COLUMN IF NOT EXISTS official_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS model_family TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS model_spec TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS standard_key_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS naming_updated_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_groups_standard_key_name
    ON admin_plus_supplier_groups(supplier_id, standard_key_name);
