DROP POLICY IF EXISTS submission_fingerprint_tenant_rls ON submission_fingerprint;
DROP POLICY IF EXISTS judge_event_outbox_tenant_rls ON judge_event_outbox;
DROP POLICY IF EXISTS judge_result_tenant_rls ON judge_result;
DROP POLICY IF EXISTS judge_task_tenant_rls ON judge_task;

DROP TABLE IF EXISTS submission_fingerprint;
DROP TABLE IF EXISTS judge_event_outbox;
DROP TABLE IF EXISTS judge_result;
DROP TABLE IF EXISTS judge_task;
DROP TABLE IF EXISTS judger;
