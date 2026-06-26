CREATE TABLE IF NOT EXISTS admin_plus_proxy_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    subscription_type TEXT NOT NULL,
    url_ciphertext TEXT NOT NULL DEFAULT '',
    url_hash TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    refresh_interval_seconds INTEGER NOT NULL DEFAULT 3600,
    last_refresh_status TEXT NOT NULL DEFAULT 'never',
    last_refresh_error TEXT NOT NULL DEFAULT '',
    active_config_version TEXT NOT NULL DEFAULT '',
    node_count INTEGER NOT NULL DEFAULT 0,
    created_by BIGINT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_refreshed_at TIMESTAMPTZ NULL,
    CONSTRAINT admin_plus_proxy_subscriptions_type_check CHECK (subscription_type IN ('clash', 'shadowrocket', 'v2ray_ss')),
    CONSTRAINT admin_plus_proxy_subscriptions_refresh_status_check CHECK (last_refresh_status IN ('never', 'succeeded', 'failed', 'invalid')),
    CONSTRAINT admin_plus_proxy_subscriptions_refresh_interval_check CHECK (refresh_interval_seconds >= 60)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_proxy_subscriptions_url_hash
    ON admin_plus_proxy_subscriptions(url_hash)
    WHERE url_hash <> '';

CREATE INDEX IF NOT EXISTS idx_admin_plus_proxy_subscriptions_enabled
    ON admin_plus_proxy_subscriptions(enabled, updated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_proxy_config_versions (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL REFERENCES admin_plus_proxy_subscriptions(id) ON DELETE CASCADE,
    config_version TEXT NOT NULL DEFAULT '',
    mihomo_yaml TEXT NOT NULL DEFAULT '',
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_proxy_config_versions_unique
    ON admin_plus_proxy_config_versions(subscription_id, config_version);

CREATE TABLE IF NOT EXISTS admin_plus_proxy_nodes (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL REFERENCES admin_plus_proxy_subscriptions(id) ON DELETE CASCADE,
    config_version TEXT NOT NULL DEFAULT '',
    node_key TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    protocol TEXT NOT NULL DEFAULT '',
    region TEXT NOT NULL DEFAULT '',
    server_hash TEXT NOT NULL DEFAULT '',
    health_status TEXT NOT NULL DEFAULT 'unknown',
    last_latency_ms INTEGER NULL,
    last_egress_ip TEXT NOT NULL DEFAULT '',
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ NULL,
    disabled_reason TEXT NOT NULL DEFAULT '',
    raw_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_proxy_nodes_health_check CHECK (health_status IN ('unknown', 'healthy', 'degraded', 'suspect', 'unhealthy', 'disabled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_proxy_nodes_unique_snapshot
    ON admin_plus_proxy_nodes(subscription_id, config_version, node_key);

CREATE INDEX IF NOT EXISTS idx_admin_plus_proxy_nodes_subscription_health
    ON admin_plus_proxy_nodes(subscription_id, health_status, updated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_proxy_policies (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    subscription_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    preferred_regions JSONB NOT NULL DEFAULT '[]'::jsonb,
    max_concurrency INTEGER NOT NULL DEFAULT 1,
    max_switches_per_task INTEGER NOT NULL DEFAULT 2,
    connect_timeout_ms INTEGER NOT NULL DEFAULT 10000,
    request_timeout_ms INTEGER NOT NULL DEFAULT 30000,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_proxy_policies_concurrency_check CHECK (max_concurrency > 0),
    CONSTRAINT admin_plus_proxy_policies_switch_check CHECK (max_switches_per_task >= 0),
    CONSTRAINT admin_plus_proxy_policies_timeout_check CHECK (connect_timeout_ms > 0 AND request_timeout_ms > 0)
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_proxy_policies_enabled
    ON admin_plus_proxy_policies(enabled, updated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_proxy_target_policies (
    id BIGSERIAL PRIMARY KEY,
    policy_id BIGINT NOT NULL REFERENCES admin_plus_proxy_policies(id) ON DELETE CASCADE,
    target_host TEXT NOT NULL DEFAULT '',
    purpose TEXT NOT NULL DEFAULT '',
    allowed_methods JSONB NOT NULL DEFAULT '["GET","POST"]'::jsonb,
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 60,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    authorization_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_proxy_target_policies_purpose_check CHECK (purpose IN ('site_discovery', 'registration', 'supplier_probe', 'manual_test')),
    CONSTRAINT admin_plus_proxy_target_policies_rate_check CHECK (rate_limit_per_minute > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_proxy_target_policies_unique
    ON admin_plus_proxy_target_policies(policy_id, target_host, purpose);

CREATE INDEX IF NOT EXISTS idx_admin_plus_proxy_target_policies_lookup
    ON admin_plus_proxy_target_policies(policy_id, purpose, target_host)
    WHERE enabled;

CREATE TABLE IF NOT EXISTS admin_plus_proxy_runtime_slots (
    id BIGSERIAL PRIMARY KEY,
    slot_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'idle',
    mixed_port INTEGER NOT NULL DEFAULT 0,
    controller_port INTEGER NOT NULL DEFAULT 0,
    controller_secret_ciphertext TEXT NOT NULL DEFAULT '',
    process_id INTEGER NULL,
    config_path TEXT NOT NULL DEFAULT '',
    assigned_task_type TEXT NOT NULL DEFAULT '',
    assigned_task_id TEXT NOT NULL DEFAULT '',
    selected_node_id BIGINT NULL REFERENCES admin_plus_proxy_nodes(id) ON DELETE SET NULL,
    last_started_at TIMESTAMPTZ NULL,
    last_heartbeat_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_proxy_runtime_slots_status_check CHECK (status IN ('idle', 'assigned', 'draining', 'unhealthy', 'stopped')),
    CONSTRAINT admin_plus_proxy_runtime_slots_port_check CHECK (mixed_port >= 0 AND controller_port >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_proxy_runtime_slots_key
    ON admin_plus_proxy_runtime_slots(slot_key)
    WHERE slot_key <> '';

CREATE INDEX IF NOT EXISTS idx_admin_plus_proxy_runtime_slots_status
    ON admin_plus_proxy_runtime_slots(status, updated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_proxy_assignments (
    id BIGSERIAL PRIMARY KEY,
    task_type TEXT NOT NULL DEFAULT '',
    task_id TEXT NOT NULL DEFAULT '',
    policy_id BIGINT NOT NULL REFERENCES admin_plus_proxy_policies(id) ON DELETE RESTRICT,
    slot_id BIGINT NOT NULL REFERENCES admin_plus_proxy_runtime_slots(id) ON DELETE RESTRICT,
    node_id BIGINT NULL REFERENCES admin_plus_proxy_nodes(id) ON DELETE SET NULL,
    target_host TEXT NOT NULL DEFAULT '',
    egress_ip TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    switch_count INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_proxy_assignments_status_check CHECK (status IN ('active', 'released', 'failed')),
    CONSTRAINT admin_plus_proxy_assignments_switch_count_check CHECK (switch_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_proxy_assignments_task
    ON admin_plus_proxy_assignments(task_type, task_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_proxy_assignments_status
    ON admin_plus_proxy_assignments(status, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_proxy_health_checks (
    id BIGSERIAL PRIMARY KEY,
    node_id BIGINT NOT NULL REFERENCES admin_plus_proxy_nodes(id) ON DELETE CASCADE,
    check_type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    latency_ms INTEGER NULL,
    egress_ip TEXT NOT NULL DEFAULT '',
    target_host TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_proxy_health_checks_type_check CHECK (check_type IN ('node_connectivity', 'egress_ip', 'target_reachability')),
    CONSTRAINT admin_plus_proxy_health_checks_status_check CHECK (status IN ('succeeded', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_proxy_health_checks_node
    ON admin_plus_proxy_health_checks(node_id, checked_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS admin_plus_proxy_audit_events (
    id BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL DEFAULT '',
    actor_id BIGINT NULL,
    task_type TEXT NOT NULL DEFAULT '',
    task_id TEXT NOT NULL DEFAULT '',
    policy_id BIGINT NULL REFERENCES admin_plus_proxy_policies(id) ON DELETE SET NULL,
    slot_id BIGINT NULL REFERENCES admin_plus_proxy_runtime_slots(id) ON DELETE SET NULL,
    node_id BIGINT NULL REFERENCES admin_plus_proxy_nodes(id) ON DELETE SET NULL,
    subscription_id BIGINT NULL REFERENCES admin_plus_proxy_subscriptions(id) ON DELETE SET NULL,
    target_host TEXT NOT NULL DEFAULT '',
    level TEXT NOT NULL DEFAULT 'info',
    message TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_proxy_audit_events_level_check CHECK (level IN ('info', 'warning', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_proxy_audit_events_created
    ON admin_plus_proxy_audit_events(created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_admin_plus_proxy_audit_events_task
    ON admin_plus_proxy_audit_events(task_type, task_id, created_at DESC, id DESC)
    WHERE task_type <> '' AND task_id <> '';
