DROP POLICY IF EXISTS sim_share_tenant_rls ON sim_share;
DROP POLICY IF EXISTS sim_checkpoint_tenant_rls ON sim_checkpoint;
DROP POLICY IF EXISTS sim_action_log_tenant_rls ON sim_action_log;
DROP POLICY IF EXISTS sim_session_tenant_rls ON sim_session;

DROP TABLE IF EXISTS sim_share;
DROP TABLE IF EXISTS sim_checkpoint;
DROP TABLE IF EXISTS sim_action_log;
DROP TABLE IF EXISTS sim_session;
DROP TABLE IF EXISTS sim_package_review;
DROP TABLE IF EXISTS sim_package;
