CREATE TABLE IF NOT EXISTS admin_plus_supplier_funding_transactions (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    provider_type TEXT NOT NULL DEFAULT 'sub2api',
    external_id TEXT NOT NULL,
    out_trade_no TEXT NOT NULL DEFAULT '',
    payment_trade_no TEXT NOT NULL DEFAULT '',
    payment_type TEXT NOT NULL DEFAULT '',
    order_type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'UNKNOWN',
    currency TEXT NOT NULL DEFAULT 'USD',
    amount_cents BIGINT NOT NULL DEFAULT 0,
    cash_amount_cents BIGINT NOT NULL DEFAULT 0,
    refund_amount_cents BIGINT NOT NULL DEFAULT 0,
    fee_rate DOUBLE PRECISION NULL,
    created_at_external TIMESTAMPTZ NULL,
    paid_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_supplier_funding_amount_check CHECK (
        amount_cents >= 0
        AND cash_amount_cents >= 0
        AND refund_amount_cents >= 0
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_supplier_funding_external
    ON admin_plus_supplier_funding_transactions(supplier_id, provider_type, external_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_supplier_funding_out_trade
    ON admin_plus_supplier_funding_transactions(supplier_id, provider_type, out_trade_no)
    WHERE out_trade_no <> '';

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_funding_supplier_created
    ON admin_plus_supplier_funding_transactions(supplier_id, created_at_external DESC NULLS LAST, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_supplier_entitlement_transactions (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    provider_type TEXT NOT NULL DEFAULT 'sub2api',
    external_id TEXT NOT NULL,
    code_fingerprint TEXT NOT NULL DEFAULT '',
    code_last4 TEXT NOT NULL DEFAULT '',
    source_family TEXT NOT NULL DEFAULT 'manual_redeem',
    type TEXT NOT NULL DEFAULT 'balance',
    status TEXT NOT NULL DEFAULT 'used',
    currency TEXT NOT NULL DEFAULT 'USD',
    value_cents BIGINT NOT NULL DEFAULT 0,
    raw_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    group_id BIGINT NOT NULL DEFAULT 0,
    validity_days INTEGER NOT NULL DEFAULT 0,
    used_at TIMESTAMPTZ NULL,
    created_at_external TIMESTAMPTZ NULL,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_supplier_entitlement_amount_check CHECK (
        value_cents >= 0
        AND validity_days >= 0
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_supplier_entitlement_external
    ON admin_plus_supplier_entitlement_transactions(supplier_id, provider_type, external_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_supplier_entitlement_code
    ON admin_plus_supplier_entitlement_transactions(supplier_id, provider_type, code_fingerprint)
    WHERE code_fingerprint <> '';

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_entitlement_supplier_used
    ON admin_plus_supplier_entitlement_transactions(supplier_id, used_at DESC NULLS LAST, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_supplier_cost_ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    provider_type TEXT NOT NULL DEFAULT 'sub2api',
    entry_type TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_id BIGINT NOT NULL,
    source_external_id TEXT NOT NULL DEFAULT '',
    currency TEXT NOT NULL DEFAULT 'USD',
    amount_cents BIGINT NOT NULL DEFAULT 0,
    cash_amount_cents BIGINT NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_supplier_cost_ledger_entry_type_check CHECK (
        entry_type IN ('funding_credit', 'entitlement_credit', 'usage_debit', 'refund_debit', 'manual_adjustment', 'reversal')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_supplier_cost_ledger_source
    ON admin_plus_supplier_cost_ledger_entries(supplier_id, provider_type, entry_type, source_type, source_id);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_cost_ledger_supplier_occurred
    ON admin_plus_supplier_cost_ledger_entries(supplier_id, occurred_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_supplier_cost_snapshots (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    currency TEXT NOT NULL DEFAULT 'USD',
    completed_funding_amount_cents BIGINT NOT NULL DEFAULT 0,
    completed_funding_cash_cents BIGINT NOT NULL DEFAULT 0,
    entitlement_amount_cents BIGINT NOT NULL DEFAULT 0,
    usage_cost_cents BIGINT NOT NULL DEFAULT 0,
    refund_amount_cents BIGINT NOT NULL DEFAULT 0,
    adjustment_amount_cents BIGINT NOT NULL DEFAULT 0,
    expected_balance_cents BIGINT NOT NULL DEFAULT 0,
    actual_balance_cents BIGINT NULL,
    balance_delta_cents BIGINT NULL,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_cost_snapshots_supplier_captured
    ON admin_plus_supplier_cost_snapshots(supplier_id, captured_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_cost_snapshots_supplier_currency
    ON admin_plus_supplier_cost_snapshots(supplier_id, currency, captured_at DESC, id DESC);
