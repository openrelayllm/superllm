CREATE TABLE IF NOT EXISTS admin_plus_supplier_browser_sessions (
    supplier_id BIGINT PRIMARY KEY REFERENCES admin_plus_suppliers(id) ON DELETE CASCADE,
    origin TEXT NOT NULL DEFAULT '',
    api_base_url TEXT NOT NULL DEFAULT '',
    session_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    session_bundle_ciphertext TEXT NOT NULL DEFAULT '',
    captured_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NULL,
    source_extension_task_id BIGINT NULL REFERENCES admin_plus_extension_tasks(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_plus_supplier_browser_sessions_captured
    ON admin_plus_supplier_browser_sessions(captured_at DESC, supplier_id);
