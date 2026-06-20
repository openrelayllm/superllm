CREATE TABLE IF NOT EXISTS admin_plus_supplier_groups (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    external_group_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    provider_family TEXT NOT NULL DEFAULT 'mixed',
    rate_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1,
    user_rate_multiplier DOUBLE PRECISION NULL,
    effective_rate_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1,
    rpm_limit BIGINT NULL,
    daily_limit_usd DOUBLE PRECISION NULL,
    weekly_limit_usd DOUBLE PRECISION NULL,
    monthly_limit_usd DOUBLE PRECISION NULL,
    allow_image_generation BOOLEAN NOT NULL DEFAULT FALSE,
    is_private BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL DEFAULT 'active',
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_supplier_groups_status_check CHECK (status IN ('active', 'missing', 'disabled')),
    CONSTRAINT admin_plus_supplier_groups_multiplier_check CHECK (
        rate_multiplier >= 0
        AND (user_rate_multiplier IS NULL OR user_rate_multiplier >= 0)
        AND effective_rate_multiplier >= 0
    ),
    CONSTRAINT admin_plus_supplier_groups_limit_check CHECK (
        (rpm_limit IS NULL OR rpm_limit >= 0)
        AND (daily_limit_usd IS NULL OR daily_limit_usd >= 0)
        AND (weekly_limit_usd IS NULL OR weekly_limit_usd >= 0)
        AND (monthly_limit_usd IS NULL OR monthly_limit_usd >= 0)
    ),
    CONSTRAINT admin_plus_supplier_groups_unique_external UNIQUE (supplier_id, external_group_id)
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_groups_supplier_status
    ON admin_plus_supplier_groups(supplier_id, status, updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_groups_supplier_seen
    ON admin_plus_supplier_groups(supplier_id, last_seen_at DESC, id DESC);
