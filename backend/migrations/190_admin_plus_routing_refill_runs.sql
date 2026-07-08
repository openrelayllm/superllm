CREATE TABLE IF NOT EXISTS admin_plus_routing_refill_runs (
    id BIGSERIAL PRIMARY KEY,
    run_id TEXT NOT NULL DEFAULT '',
    sub2api_instance_id TEXT NOT NULL DEFAULT 'local',
    local_group_id BIGINT NOT NULL DEFAULT 0,
    local_group_name TEXT NOT NULL DEFAULT '',
    platform TEXT NOT NULL DEFAULT '',
    model_scope TEXT NOT NULL DEFAULT '',
    trigger_type TEXT NOT NULL DEFAULT 'manual',
    dry_run BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL DEFAULT 'succeeded',
    reason TEXT NOT NULL DEFAULT '',
    skipped_reason TEXT NOT NULL DEFAULT '',
    before_total_accounts BIGINT NOT NULL DEFAULT 0,
    before_schedulable_accounts BIGINT NOT NULL DEFAULT 0,
    before_active_api_key_count BIGINT NOT NULL DEFAULT 0,
    after_total_accounts BIGINT NOT NULL DEFAULT 0,
    after_schedulable_accounts BIGINT NOT NULL DEFAULT 0,
    after_active_api_key_count BIGINT NOT NULL DEFAULT 0,
    selected_supplier_id BIGINT NOT NULL DEFAULT 0,
    selected_supplier_group_id BIGINT NOT NULL DEFAULT 0,
    selected_supplier_key_id BIGINT NOT NULL DEFAULT 0,
    selected_local_account_id BIGINT NOT NULL DEFAULT 0,
    selected_effective_rate_multiplier DOUBLE PRECISION NOT NULL DEFAULT 0,
    requested_by BIGINT NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    request_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    result_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_routing_refill_runs_status_check CHECK (
        status IN ('previewed', 'succeeded', 'skipped', 'failed')
    )
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_routing_refill_runs_group
    ON admin_plus_routing_refill_runs(local_group_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_routing_refill_runs_status
    ON admin_plus_routing_refill_runs(status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_routing_refill_runs_supplier
    ON admin_plus_routing_refill_runs(selected_supplier_id, created_at DESC, id DESC)
    WHERE selected_supplier_id > 0;
