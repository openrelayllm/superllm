ALTER TABLE admin_plus_action_executions
    ADD COLUMN IF NOT EXISTS idempotency_key_hash TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS idempotency_replayed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS before_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS after_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_admin_plus_action_executions_idempotency
    ON admin_plus_action_executions(idempotency_key_hash, created_at DESC, id DESC)
    WHERE idempotency_key_hash <> '';
