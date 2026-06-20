CREATE TABLE IF NOT EXISTS admin_plus_balance_snapshots (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT 'manual',
    runtime_status TEXT NOT NULL DEFAULT 'monitor_only',
    balance_cents BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    switch_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_balance_snapshots_amount_check CHECK (balance_cents >= 0),
    CONSTRAINT admin_plus_balance_snapshots_runtime_status_check CHECK (runtime_status IN ('monitor_only', 'candidate', 'active', 'disabled'))
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_balance_snapshots_supplier_captured
    ON admin_plus_balance_snapshots(supplier_id, captured_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_balance_snapshots_supplier_currency
    ON admin_plus_balance_snapshots(supplier_id, currency, captured_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_balance_events (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    snapshot_id BIGINT NOT NULL REFERENCES admin_plus_balance_snapshots(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    runtime_status TEXT NOT NULL DEFAULT 'monitor_only',
    old_balance_cents BIGINT NULL,
    new_balance_cents BIGINT NOT NULL DEFAULT 0,
    low_balance_threshold_cents BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    switch_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ NULL,
    CONSTRAINT admin_plus_balance_events_type_check CHECK (type IN ('low_balance', 'depleted', 'recovered')),
    CONSTRAINT admin_plus_balance_events_status_check CHECK (status IN ('open', 'acknowledged', 'ignored')),
    CONSTRAINT admin_plus_balance_events_amount_check CHECK (
        (old_balance_cents IS NULL OR old_balance_cents >= 0)
        AND new_balance_cents >= 0
        AND low_balance_threshold_cents >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_balance_events_supplier_status
    ON admin_plus_balance_events(supplier_id, status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_balance_events_snapshot
    ON admin_plus_balance_events(snapshot_id);

CREATE TABLE IF NOT EXISTS admin_plus_promotion_events (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT 'manual',
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    currency TEXT NOT NULL DEFAULT 'USD',
    min_recharge_cents BIGINT NOT NULL DEFAULT 0,
    bonus_percent DOUBLE PRECISION NULL,
    discount_percent DOUBLE PRECISION NULL,
    runtime_status TEXT NOT NULL DEFAULT 'monitor_only',
    balance_cents BIGINT NOT NULL DEFAULT 0,
    switch_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    recommendation TEXT NOT NULL DEFAULT 'informational',
    status TEXT NOT NULL DEFAULT 'open',
    starts_at TIMESTAMPTZ NULL,
    ends_at TIMESTAMPTZ NULL,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ NULL,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT admin_plus_promotion_events_type_check CHECK (type IN ('recharge_bonus', 'rate_discount', 'package_deal', 'limited_offer', 'other')),
    CONSTRAINT admin_plus_promotion_events_status_check CHECK (status IN ('open', 'acknowledged', 'ignored')),
    CONSTRAINT admin_plus_promotion_events_recommendation_check CHECK (recommendation IN ('recharge_to_unlock', 'switch_candidate', 'monitor_only', 'informational')),
    CONSTRAINT admin_plus_promotion_events_amount_check CHECK (min_recharge_cents >= 0 AND balance_cents >= 0),
    CONSTRAINT admin_plus_promotion_events_percent_check CHECK (
        (bonus_percent IS NULL OR (bonus_percent >= 0 AND bonus_percent <= 100))
        AND (discount_percent IS NULL OR (discount_percent >= 0 AND discount_percent <= 100))
    )
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_promotion_events_supplier_status
    ON admin_plus_promotion_events(supplier_id, status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_promotion_events_recommendation
    ON admin_plus_promotion_events(recommendation, status, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_health_samples (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT 'manual',
    model TEXT NOT NULL,
    first_token_latency_ms BIGINT NOT NULL DEFAULT 0,
    total_latency_ms BIGINT NOT NULL DEFAULT 0,
    status_code INTEGER NOT NULL DEFAULT 0,
    error_class TEXT NOT NULL DEFAULT '',
    observed_concurrency INTEGER NOT NULL DEFAULT 0,
    available_concurrency INTEGER NULL,
    concurrency_limit INTEGER NULL,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_health_samples_latency_check CHECK (first_token_latency_ms >= 0 AND total_latency_ms >= 0),
    CONSTRAINT admin_plus_health_samples_concurrency_check CHECK (
        observed_concurrency >= 0
        AND (available_concurrency IS NULL OR available_concurrency >= 0)
        AND (concurrency_limit IS NULL OR concurrency_limit >= 0)
    )
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_health_samples_supplier_captured
    ON admin_plus_health_samples(supplier_id, captured_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_health_samples_supplier_model
    ON admin_plus_health_samples(supplier_id, model, captured_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_health_events (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    sample_id BIGINT NOT NULL REFERENCES admin_plus_health_samples(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    model TEXT NOT NULL,
    observed_value BIGINT NOT NULL DEFAULT 0,
    threshold_value BIGINT NOT NULL DEFAULT 0,
    status_code INTEGER NOT NULL DEFAULT 0,
    error_class TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ NULL,
    CONSTRAINT admin_plus_health_events_type_check CHECK (type IN ('slow_first_token', 'slow_total', 'request_error', 'concurrency_full')),
    CONSTRAINT admin_plus_health_events_status_check CHECK (status IN ('open', 'acknowledged', 'ignored'))
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_health_events_supplier_status
    ON admin_plus_health_events(supplier_id, status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_health_events_sample
    ON admin_plus_health_events(sample_id);

CREATE TABLE IF NOT EXISTS admin_plus_supplier_bill_lines (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT 'manual',
    external_bill_id TEXT NOT NULL DEFAULT '',
    external_request_id TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'USD',
    cost_cents BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ NULL,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_supplier_bill_lines_amount_check CHECK (cost_cents >= 0 AND input_tokens >= 0 AND output_tokens >= 0)
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_bill_lines_supplier_started
    ON admin_plus_supplier_bill_lines(supplier_id, started_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_bill_lines_external_request
    ON admin_plus_supplier_bill_lines(external_request_id)
    WHERE external_request_id <> '';

CREATE TABLE IF NOT EXISTS admin_plus_extension_tasks (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    priority INTEGER NOT NULL DEFAULT 0,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    device_id TEXT NOT NULL DEFAULT '',
    lease_token TEXT NOT NULL DEFAULT '',
    lease_expires_at TIMESTAMPTZ NULL,
    last_heartbeat_at TIMESTAMPTZ NULL,
    available_after TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    result JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ NULL,
    CONSTRAINT admin_plus_extension_tasks_type_check CHECK (type IN ('fetch_rates', 'fetch_balance', 'fetch_promotions', 'export_bills', 'fetch_health')),
    CONSTRAINT admin_plus_extension_tasks_status_check CHECK (status IN ('pending', 'claimed', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT admin_plus_extension_tasks_attempts_check CHECK (attempts >= 0 AND max_attempts > 0)
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_extension_tasks_claim
    ON admin_plus_extension_tasks(status, available_after, priority DESC, created_at ASC, id ASC)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_admin_plus_extension_tasks_supplier_status
    ON admin_plus_extension_tasks(supplier_id, status, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_action_recommendations (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL DEFAULT 0,
    target_supplier_id BIGINT NULL REFERENCES admin_plus_suppliers(id) ON DELETE SET NULL,
    type TEXT NOT NULL,
    severity TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    reason_code TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    expected_impact TEXT NOT NULL DEFAULT '',
    requires_approval BOOLEAN NOT NULL DEFAULT TRUE,
    signals TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_action_recommendations_type_check CHECK (type IN ('switch_supplier', 'pause_supplier', 'degrade_supplier', 'increase_weight', 'recharge_supplier', 'investigate_profit', 'review_credential')),
    CONSTRAINT admin_plus_action_recommendations_severity_check CHECK (severity IN ('info', 'warning', 'critical')),
    CONSTRAINT admin_plus_action_recommendations_status_check CHECK (status IN ('open', 'acknowledged', 'approved', 'executed', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_action_recommendations_supplier_status
    ON admin_plus_action_recommendations(supplier_id, status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_action_recommendations_status_severity
    ON admin_plus_action_recommendations(status, severity, created_at DESC, id DESC);
