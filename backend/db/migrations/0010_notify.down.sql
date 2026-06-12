DROP POLICY IF EXISTS announcement_read_tenant_rls ON announcement_read;
DROP POLICY IF EXISTS system_announcement_tenant_rls ON system_announcement;
DROP POLICY IF EXISTS notification_preference_tenant_rls ON notification_preference;
DROP POLICY IF EXISTS notification_tenant_rls ON notification;

DROP TABLE IF EXISTS announcement_read;
DROP TABLE IF EXISTS system_announcement;
DROP TABLE IF EXISTS notification_preference;
DROP TABLE IF EXISTS notification_template;
DROP TABLE IF EXISTS notification;
