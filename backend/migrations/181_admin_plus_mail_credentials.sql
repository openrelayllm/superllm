CREATE TABLE IF NOT EXISTS admin_plus_mail_credentials (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    email_masked TEXT NOT NULL DEFAULT '',
    access_token_ciphertext TEXT NOT NULL DEFAULT '',
    refresh_token_ciphertext TEXT NOT NULL DEFAULT '',
    scopes TEXT NOT NULL DEFAULT '',
    token_type TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_checked_at TIMESTAMPTZ NULL,
    last_error_code TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_plus_mail_credentials_provider_check CHECK (provider IN ('gmail'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_plus_mail_credentials_provider_email
    ON admin_plus_mail_credentials(provider, email)
    WHERE email <> '';

CREATE INDEX IF NOT EXISTS idx_admin_plus_mail_credentials_provider_updated
    ON admin_plus_mail_credentials(provider, updated_at DESC, id DESC);
