CREATE TABLE IF NOT EXISTS admin_plus_supplier_keys (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    supplier_group_id BIGINT NOT NULL REFERENCES admin_plus_supplier_groups(id) ON DELETE RESTRICT,
    external_group_id TEXT NOT NULL DEFAULT '',
    external_key_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    key_fingerprint TEXT NOT NULL,
    key_last4 TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'provisioning',
    provider_family TEXT NOT NULL DEFAULT 'mixed',
    local_sub2api_account_id BIGINT NOT NULL DEFAULT 0,
    local_account_name TEXT NOT NULL DEFAULT '',
    local_account_platform TEXT NOT NULL DEFAULT '',
    local_account_type TEXT NOT NULL DEFAULT '',
    provision_request JSONB NOT NULL DEFAULT '{}'::jsonb,
    provision_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_supplier_keys_status_check CHECK (status IN ('provisioning', 'bound', 'manual_secret_required', 'failed', 'disabled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_supplier_keys_fingerprint
    ON admin_plus_supplier_keys(supplier_id, key_fingerprint);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_supplier_keys_one_active_group
    ON admin_plus_supplier_keys(supplier_id, supplier_group_id)
    WHERE status IN ('provisioning', 'bound', 'manual_secret_required');

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_keys_supplier_status
    ON admin_plus_supplier_keys(supplier_id, status, updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_keys_local_account
    ON admin_plus_supplier_keys(local_sub2api_account_id)
    WHERE local_sub2api_account_id > 0;

ALTER TABLE admin_plus_supplier_accounts
    ADD COLUMN IF NOT EXISTS supplier_key_id BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_accounts_supplier_key
    ON admin_plus_supplier_accounts(supplier_key_id)
    WHERE supplier_key_id > 0;

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_supplier_accounts_one_supplier_key
    ON admin_plus_supplier_accounts(supplier_id, supplier_key_id)
    WHERE supplier_key_id > 0;
