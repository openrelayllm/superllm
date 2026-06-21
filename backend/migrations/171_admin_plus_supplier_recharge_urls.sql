ALTER TABLE admin_plus_suppliers
    ADD COLUMN IF NOT EXISTS third_party_recharge_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS local_recharge_url TEXT NOT NULL DEFAULT '';
