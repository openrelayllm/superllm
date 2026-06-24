ALTER TABLE admin_plus_notification_deliveries
    DROP CONSTRAINT IF EXISTS admin_plus_notification_deliveries_status_check;

ALTER TABLE admin_plus_notification_deliveries
    ADD CONSTRAINT admin_plus_notification_deliveries_status_check
    CHECK (status IN ('sending', 'succeeded', 'failed', 'suppressed'));

CREATE TABLE IF NOT EXISTS admin_plus_notification_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
