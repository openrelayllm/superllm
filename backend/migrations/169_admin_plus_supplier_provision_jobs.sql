CREATE TABLE IF NOT EXISTS supplier_provision_jobs (
    id BIGSERIAL PRIMARY KEY,
    job_type TEXT NOT NULL,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'queued',
    idempotency_key_hash TEXT NOT NULL DEFAULT '',
    requested_by BIGINT NOT NULL DEFAULT 0,
    request_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    result_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    total_steps INTEGER NOT NULL DEFAULT 0,
    succeeded_steps INTEGER NOT NULL DEFAULT 0,
    failed_steps INTEGER NOT NULL DEFAULT 0,
    manual_required_steps INTEGER NOT NULL DEFAULT 0,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_by TEXT NOT NULL DEFAULT '',
    locked_until TIMESTAMPTZ NULL,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ NULL,
    CONSTRAINT supplier_provision_jobs_type_check CHECK (
        job_type IN ('sync_groups', 'provision_group_key', 'provision_all_group_keys', 'repair_binding')
    ),
    CONSTRAINT supplier_provision_jobs_status_check CHECK (
        status IN ('queued', 'running', 'succeeded', 'partial_succeeded', 'retryable_failed', 'manual_required', 'dead', 'cancelled')
    ),
    CONSTRAINT supplier_provision_jobs_attempts_check CHECK (
        attempts >= 0 AND max_attempts > 0 AND total_steps >= 0 AND succeeded_steps >= 0 AND failed_steps >= 0 AND manual_required_steps >= 0
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_supplier_provision_jobs_idempotency
    ON supplier_provision_jobs(job_type, supplier_id, idempotency_key_hash)
    WHERE idempotency_key_hash <> '';

CREATE INDEX IF NOT EXISTS idx_supplier_provision_jobs_claim
    ON supplier_provision_jobs(status, next_run_at, id)
    WHERE status IN ('queued', 'retryable_failed');

CREATE INDEX IF NOT EXISTS idx_supplier_provision_jobs_supplier_created
    ON supplier_provision_jobs(supplier_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS supplier_provision_steps (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT NOT NULL REFERENCES supplier_provision_jobs(id) ON DELETE CASCADE,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    supplier_group_id BIGINT NOT NULL DEFAULT 0,
    step_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    idempotency_key TEXT NOT NULL DEFAULT '',
    external_resource_type TEXT NOT NULL DEFAULT '',
    external_resource_id TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_by TEXT NOT NULL DEFAULT '',
    locked_until TIMESTAMPTZ NULL,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    request_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    result_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ NULL,
    CONSTRAINT supplier_provision_steps_type_check CHECK (
        step_type IN (
            'ensure_supplier_session',
            'sync_supplier_group',
            'ensure_third_party_key',
            'ensure_sub2api_group',
            'ensure_sub2api_account',
            'upsert_admin_plus_binding',
            'enqueue_initial_collection',
            'provision_all_group_keys',
            'repair_binding'
        )
    ),
    CONSTRAINT supplier_provision_steps_status_check CHECK (
        status IN ('queued', 'running', 'succeeded', 'retryable_failed', 'manual_required', 'dead', 'skipped')
    ),
    CONSTRAINT supplier_provision_steps_attempts_check CHECK (
        attempts >= 0 AND max_attempts > 0
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_supplier_provision_steps_job_group_type
    ON supplier_provision_steps(job_id, supplier_group_id, step_type);

CREATE UNIQUE INDEX IF NOT EXISTS idx_supplier_provision_steps_active_group
    ON supplier_provision_steps(supplier_id, supplier_group_id, step_type)
    WHERE supplier_group_id > 0
      AND status IN ('queued', 'running', 'retryable_failed');

CREATE INDEX IF NOT EXISTS idx_supplier_provision_steps_job
    ON supplier_provision_steps(job_id, id);

CREATE TABLE IF NOT EXISTS admin_plus_outbox_events (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id BIGINT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_outbox_events_status_check CHECK (
        status IN ('pending', 'published', 'failed')
    )
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_outbox_events_pending
    ON admin_plus_outbox_events(status, available_at, created_at);

CREATE TABLE IF NOT EXISTS processed_events (
    event_id TEXT PRIMARY KEY,
    consumer TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS supplier_provision_attempts (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT NOT NULL REFERENCES supplier_provision_jobs(id) ON DELETE CASCADE,
    step_id BIGINT NULL REFERENCES supplier_provision_steps(id) ON DELETE SET NULL,
    supplier_id BIGINT NOT NULL REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    supplier_group_id BIGINT NOT NULL DEFAULT 0,
    operation TEXT NOT NULL,
    status TEXT NOT NULL,
    request_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_supplier_provision_attempts_job
    ON supplier_provision_attempts(job_id, created_at DESC, id DESC);
