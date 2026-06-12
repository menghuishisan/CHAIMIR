-- 0007_experiment.down.sql 回滚 M7 实验模块自有表和 RLS 策略。
DROP POLICY IF EXISTS experiment_report_tenant_rls ON experiment_report;
DROP POLICY IF EXISTS checkpoint_result_tenant_rls ON checkpoint_result;
DROP POLICY IF EXISTS group_member_tenant_rls ON group_member;
DROP POLICY IF EXISTS experiment_group_tenant_rls ON experiment_group;
DROP POLICY IF EXISTS experiment_instance_tenant_rls ON experiment_instance;
DROP POLICY IF EXISTS experiment_tenant_rls ON experiment;

DROP TABLE IF EXISTS experiment_report;
DROP TABLE IF EXISTS checkpoint_result;
DROP TABLE IF EXISTS experiment_instance;
DROP TABLE IF EXISTS group_member;
DROP TABLE IF EXISTS experiment_group;
DROP TABLE IF EXISTS experiment;
