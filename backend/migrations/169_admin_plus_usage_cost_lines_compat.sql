CREATE TABLE IF NOT EXISTS admin_plus_supplier_usage_cost_lines (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT 'manual',
    external_usage_cost_id TEXT NOT NULL DEFAULT '',
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
    api_key_name TEXT NOT NULL DEFAULT '',
    endpoint TEXT NOT NULL DEFAULT '',
    request_type TEXT NOT NULL DEFAULT '',
    billing_mode TEXT NOT NULL DEFAULT '',
    reasoning_effort TEXT NOT NULL DEFAULT '',
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    first_token_ms BIGINT NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    user_agent TEXT NOT NULL DEFAULT '',
    CONSTRAINT admin_plus_supplier_usage_cost_lines_amount_check CHECK (
        cost_cents >= 0
        AND input_tokens >= 0
        AND output_tokens >= 0
    ),
    CONSTRAINT admin_plus_supplier_usage_cost_lines_detail_check CHECK (
        cache_read_tokens >= 0
        AND total_tokens >= 0
        AND first_token_ms >= 0
        AND duration_ms >= 0
    )
);

INSERT INTO admin_plus_supplier_usage_cost_lines (
    supplier_id,
    source,
    external_usage_cost_id,
    external_request_id,
    model,
    currency,
    cost_cents,
    input_tokens,
    output_tokens,
    started_at,
    ended_at,
    raw_payload,
    created_at,
    api_key_name,
    endpoint,
    request_type,
    billing_mode,
    reasoning_effort,
    cache_read_tokens,
    total_tokens,
    first_token_ms,
    duration_ms,
    user_agent
)
SELECT
    supplier_id,
    source,
    external_bill_id,
    external_request_id,
    model,
    currency,
    cost_cents,
    input_tokens,
    output_tokens,
    started_at,
    ended_at,
    raw_payload,
    created_at,
    api_key_name,
    endpoint,
    request_type,
    billing_mode,
    reasoning_effort,
    cache_read_tokens,
    total_tokens,
    first_token_ms,
    duration_ms,
    user_agent
FROM admin_plus_supplier_bill_lines
WHERE NOT EXISTS (
    SELECT 1
    FROM admin_plus_supplier_usage_cost_lines target
    WHERE target.supplier_id = admin_plus_supplier_bill_lines.supplier_id
      AND target.external_usage_cost_id = admin_plus_supplier_bill_lines.external_bill_id
      AND target.external_request_id = admin_plus_supplier_bill_lines.external_request_id
      AND target.started_at = admin_plus_supplier_bill_lines.started_at
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_usage_cost_lines_supplier_started
    ON admin_plus_supplier_usage_cost_lines(supplier_id, started_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_usage_cost_lines_external_request
    ON admin_plus_supplier_usage_cost_lines(external_request_id)
    WHERE external_request_id <> '';

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_usage_cost_lines_model_started
    ON admin_plus_supplier_usage_cost_lines(model, started_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_usage_cost_lines_api_key_started
    ON admin_plus_supplier_usage_cost_lines(api_key_name, started_at DESC, id DESC)
    WHERE api_key_name <> '';
