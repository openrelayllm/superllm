CREATE TABLE IF NOT EXISTS admin_plus_local_account_state_snapshots (
    local_sub2api_account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    accepted_account_name TEXT NOT NULL DEFAULT '',
    accepted_account_platform TEXT NOT NULL DEFAULT '',
    accepted_account_type TEXT NOT NULL DEFAULT '',
    accepted_schedulable BOOLEAN NOT NULL DEFAULT FALSE,
    accepted_group_ids BIGINT[] NOT NULL DEFAULT ARRAY[]::BIGINT[],
    observed_account_name TEXT NOT NULL DEFAULT '',
    observed_account_platform TEXT NOT NULL DEFAULT '',
    observed_account_type TEXT NOT NULL DEFAULT '',
    observed_schedulable BOOLEAN NOT NULL DEFAULT FALSE,
    observed_group_ids BIGINT[] NOT NULL DEFAULT ARRAY[]::BIGINT[],
    drift_status TEXT NOT NULL DEFAULT 'synced',
    first_drift_detected_at TIMESTAMPTZ NULL,
    last_checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_local_account_state_snapshots_status_check CHECK (drift_status IN ('synced', 'pending'))
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_local_account_state_snapshots_drift
    ON admin_plus_local_account_state_snapshots(drift_status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_local_account_state_snapshots_groups
    ON admin_plus_local_account_state_snapshots USING GIN (observed_group_ids);

CREATE TABLE IF NOT EXISTS admin_plus_local_account_drift_events (
    id BIGSERIAL PRIMARY KEY,
    local_sub2api_account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    drift_type TEXT NOT NULL DEFAULT 'local_state',
    old_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    new_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'detected',
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ NULL,
    CONSTRAINT admin_plus_local_account_drift_events_status_check CHECK (status IN ('detected', 'accepted', 'restored', 'ignored'))
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_local_account_drift_events_account
    ON admin_plus_local_account_drift_events(local_sub2api_account_id, detected_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_local_account_drift_events_status
    ON admin_plus_local_account_drift_events(status, detected_at DESC, id DESC);
