CREATE TABLE IF NOT EXISTS admin_plus_action_executions (
    id BIGSERIAL PRIMARY KEY,
    recommendation_id BIGINT NOT NULL REFERENCES admin_plus_action_recommendations(id) ON DELETE CASCADE,
    action_type TEXT NOT NULL,
    supplier_id BIGINT NOT NULL DEFAULT 0,
    target_supplier_id BIGINT NULL REFERENCES admin_plus_suppliers(id) ON DELETE SET NULL,
    status TEXT NOT NULL,
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message TEXT NOT NULL DEFAULT '',
    operator_user_id BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_action_executions_status_check CHECK (status IN ('running', 'succeeded', 'failed', 'unsupported')),
    CONSTRAINT admin_plus_action_executions_type_check CHECK (action_type IN ('switch_supplier', 'pause_supplier', 'degrade_supplier', 'increase_weight', 'recharge_supplier', 'investigate_profit', 'review_credential'))
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_action_executions_recommendation
    ON admin_plus_action_executions(recommendation_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_action_executions_supplier_status
    ON admin_plus_action_executions(supplier_id, status, created_at DESC, id DESC);
