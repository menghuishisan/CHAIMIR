CREATE TABLE IF NOT EXISTS transfer_task (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    channel VARCHAR(16) NOT NULL,
    subject VARCHAR(128) NOT NULL,
    status VARCHAR(16) NOT NULL,
    content_type VARCHAR(128) NOT NULL DEFAULT '',
    file_name VARCHAR(255) NOT NULL DEFAULT '',
    attempt_count INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    artifact_ref TEXT NOT NULL DEFAULT '',
    artifact_size BIGINT NOT NULL DEFAULT 0,
    artifact_content_type VARCHAR(128) NOT NULL DEFAULT '',
    artifact_file_name VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    next_attempt_after TIMESTAMPTZ,
    CONSTRAINT transfer_task_channel_check CHECK (channel IN ('import', 'export')),
    CONSTRAINT transfer_task_status_check CHECK (status IN ('pending', 'running', 'retrying', 'succeeded', 'failed')),
    CONSTRAINT transfer_task_tenant_check CHECK (tenant_id >= 0),
    CONSTRAINT transfer_task_attempts_check CHECK (attempt_count >= 0 AND max_attempts > 0),
    CONSTRAINT transfer_task_artifact_size_check CHECK (artifact_size >= 0)
);

CREATE INDEX IF NOT EXISTS idx_transfer_task_tenant_account_status ON transfer_task(tenant_id, account_id, status);
CREATE INDEX IF NOT EXISTS idx_transfer_task_tenant_created ON transfer_task(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transfer_task_due ON transfer_task(status, next_attempt_after) WHERE next_attempt_after IS NOT NULL;

ALTER TABLE transfer_task ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS transfer_task_tenant_rls ON transfer_task;
CREATE POLICY transfer_task_tenant_rls ON transfer_task USING (
    tenant_id = COALESCE(NULLIF(current_setting('app.tenant_id', true), '')::BIGINT, 0)
);
