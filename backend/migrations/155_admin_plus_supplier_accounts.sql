ALTER TABLE admin_plus_suppliers
    ADD COLUMN IF NOT EXISTS browser_login_username_configured BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS browser_login_password_configured BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS browser_login_token_configured BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS masked_browser_login_username TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS admin_plus_supplier_accounts (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    local_sub2api_account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    local_account_name TEXT NOT NULL DEFAULT '',
    local_account_platform TEXT NOT NULL DEFAULT '',
    local_account_type TEXT NOT NULL DEFAULT '',
    supplier_account_identifier TEXT NOT NULL DEFAULT '',
    supplier_account_label TEXT NOT NULL DEFAULT '',
    organization_id TEXT NOT NULL DEFAULT '',
    project_id TEXT NOT NULL DEFAULT '',
    rate_profile TEXT NOT NULL DEFAULT '',
    configured_concurrency INTEGER NOT NULL DEFAULT 0,
    observed_max_concurrency INTEGER NOT NULL DEFAULT 0,
    balance_threshold_cents BIGINT NOT NULL DEFAULT 0,
    balance_cents BIGINT NOT NULL DEFAULT 0,
    balance_currency TEXT NOT NULL DEFAULT 'USD',
    has_usable_balance BOOLEAN NOT NULL DEFAULT FALSE,
    runtime_status TEXT NOT NULL DEFAULT 'monitor_only',
    health_status TEXT NOT NULL DEFAULT 'normal',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_supplier_accounts_unique_local_account UNIQUE (supplier_id, local_sub2api_account_id),
    CONSTRAINT admin_plus_supplier_accounts_runtime_status_check CHECK (runtime_status IN ('monitor_only', 'candidate', 'active', 'disabled')),
    CONSTRAINT admin_plus_supplier_accounts_health_status_check CHECK (health_status IN ('normal', 'unavailable', 'credential_invalid', 'paused')),
    CONSTRAINT admin_plus_supplier_accounts_amount_check CHECK (
        configured_concurrency >= 0
        AND observed_max_concurrency >= 0
        AND balance_threshold_cents >= 0
        AND balance_cents >= 0
    ),
    CONSTRAINT admin_plus_supplier_accounts_switchable_balance_check CHECK (
        runtime_status NOT IN ('candidate', 'active') OR balance_cents > 0
    )
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_accounts_supplier
    ON admin_plus_supplier_accounts(supplier_id, id);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_accounts_local
    ON admin_plus_supplier_accounts(local_sub2api_account_id);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_accounts_runtime
    ON admin_plus_supplier_accounts(runtime_status, health_status);
