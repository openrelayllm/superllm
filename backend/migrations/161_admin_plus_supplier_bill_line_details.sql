ALTER TABLE admin_plus_supplier_usage_cost_lines
    ADD COLUMN IF NOT EXISTS api_key_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS endpoint TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS request_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS billing_mode TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reasoning_effort TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_tokens BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS first_token_ms BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS duration_ms BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS user_agent TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'admin_plus_supplier_usage_cost_lines_detail_check'
    ) THEN
        ALTER TABLE admin_plus_supplier_usage_cost_lines
            ADD CONSTRAINT admin_plus_supplier_usage_cost_lines_detail_check
            CHECK (
                cache_read_tokens >= 0
                AND total_tokens >= 0
                AND first_token_ms >= 0
                AND duration_ms >= 0
            );
    END IF;
END $$;

UPDATE admin_plus_supplier_usage_cost_lines
SET total_tokens = input_tokens + output_tokens + cache_read_tokens
WHERE total_tokens = 0
  AND (input_tokens > 0 OR output_tokens > 0 OR cache_read_tokens > 0);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_usage_cost_lines_model_started
    ON admin_plus_supplier_usage_cost_lines(model, started_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_usage_cost_lines_api_key_started
    ON admin_plus_supplier_usage_cost_lines(api_key_name, started_at DESC, id DESC)
    WHERE api_key_name <> '';
