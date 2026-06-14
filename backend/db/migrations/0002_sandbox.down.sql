DROP POLICY IF EXISTS tenant_quota_tenant_rls ON tenant_quota;
DROP POLICY IF EXISTS sandbox_recycle_outbox_tenant_rls ON sandbox_recycle_outbox;
DROP POLICY IF EXISTS sandbox_event_tenant_rls ON sandbox_event;
DROP POLICY IF EXISTS sandbox_tool_tenant_rls ON sandbox_tool;
DROP POLICY IF EXISTS sandbox_tenant_rls ON sandbox;

DROP TABLE IF EXISTS tenant_quota;
DROP TABLE IF EXISTS sandbox_recycle_outbox;
DROP TABLE IF EXISTS sandbox_event;
DROP TABLE IF EXISTS sandbox_tool;
DROP TABLE IF EXISTS sandbox;
DROP TABLE IF EXISTS tool;
DROP TABLE IF EXISTS runtime_image;
DROP TABLE IF EXISTS runtime;
