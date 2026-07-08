ALTER TABLE admin_plus_action_executions
    ADD COLUMN IF NOT EXISTS scheduler_run_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS scheduler_step_id BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_admin_plus_action_executions_scheduler_source
    ON admin_plus_action_executions(scheduler_run_id, scheduler_step_id, created_at DESC, id DESC)
    WHERE scheduler_run_id <> '' OR scheduler_step_id > 0;
