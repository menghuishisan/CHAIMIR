DROP POLICY IF EXISTS platform_statistics_tenant_rls ON platform_statistics;
DROP POLICY IF EXISTS alert_event_tenant_rls ON alert_event;
DROP POLICY IF EXISTS alert_rule_tenant_rls ON alert_rule;
DROP POLICY IF EXISTS config_change_log_tenant_rls ON config_change_log;
DROP POLICY IF EXISTS system_config_tenant_rls ON system_config;

DROP TABLE IF EXISTS backup_record;
DROP TABLE IF EXISTS platform_statistics;
DROP TABLE IF EXISTS alert_event;
DROP TABLE IF EXISTS alert_rule;
DROP TABLE IF EXISTS config_change_log;
DROP TABLE IF EXISTS system_config;
