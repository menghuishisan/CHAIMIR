DROP POLICY IF EXISTS tenant_quota_tenant_rls ON tenant_quota;
DROP POLICY IF EXISTS sandbox_recycle_outbox_tenant_rls ON sandbox_recycle_outbox;
DROP POLICY IF EXISTS sandbox_event_tenant_rls ON sandbox_event;
DROP POLICY IF EXISTS sandbox_tool_tenant_rls ON sandbox_tool;
DROP POLICY IF EXISTS sandbox_tenant_rls ON sandbox;

DROP INDEX IF EXISTS idx_sandbox_recycle_outbox_tenant_sandbox;
DROP INDEX IF EXISTS idx_sandbox_recycle_outbox_tenant_status;
DROP INDEX IF EXISTS idx_sandbox_recycle_outbox_status;
DROP INDEX IF EXISTS idx_sandbox_event_tenant_sandbox_created;
DROP INDEX IF EXISTS idx_sandbox_snapshot_expire;
DROP INDEX IF EXISTS idx_sandbox_source_ref;
DROP INDEX IF EXISTS idx_sandbox_last_active;
DROP INDEX IF EXISTS idx_sandbox_tenant_owner;
DROP INDEX IF EXISTS idx_sandbox_tenant_status;
DROP INDEX IF EXISTS idx_tool_status;
DROP INDEX IF EXISTS idx_runtime_image_runtime_status;
DROP INDEX IF EXISTS idx_runtime_status;
DROP INDEX IF EXISTS uk_runtime_image_default;

DROP TABLE IF EXISTS tenant_quota;
DROP TABLE IF EXISTS sandbox_recycle_outbox;
DROP TABLE IF EXISTS sandbox_event;
DROP TABLE IF EXISTS sandbox_tool;
DROP TABLE IF EXISTS sandbox;
DROP TABLE IF EXISTS tool;
DROP TABLE IF EXISTS runtime_image;
DROP TABLE IF EXISTS runtime;
