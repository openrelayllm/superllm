CREATE TABLE IF NOT EXISTS admin_plus_notification_deliveries (
    id BIGSERIAL PRIMARY KEY,
    channel TEXT NOT NULL,
    event_type TEXT NOT NULL,
    event_id BIGINT NOT NULL,
    supplier_id BIGINT NOT NULL DEFAULT 0,
    dedupe_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'sending',
    attempts INTEGER NOT NULL DEFAULT 1,
    last_error TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    sent_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_notification_deliveries_channel_check CHECK (channel IN ('feishu')),
    CONSTRAINT admin_plus_notification_deliveries_status_check CHECK (status IN ('sending', 'succeeded', 'failed')),
    CONSTRAINT admin_plus_notification_deliveries_attempts_check CHECK (attempts >= 1),
    CONSTRAINT admin_plus_notification_deliveries_event_check CHECK (event_id > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS admin_plus_notification_deliveries_dedupe_key_unique
    ON admin_plus_notification_deliveries(dedupe_key);

CREATE INDEX IF NOT EXISTS idx_admin_plus_notification_deliveries_supplier_status
    ON admin_plus_notification_deliveries(supplier_id, status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_notification_deliveries_event
    ON admin_plus_notification_deliveries(event_type, event_id);
