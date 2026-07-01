ALTER TABLE judge_task
    DROP CONSTRAINT IF EXISTS judge_task_tenant_id_source_ref_problem_ref_key;

ALTER TABLE judge_task
    ADD CONSTRAINT judge_task_tenant_id_source_ref_key
    UNIQUE (tenant_id, source_ref);
