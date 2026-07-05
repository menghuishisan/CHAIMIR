ALTER TABLE judge_task
    DROP CONSTRAINT IF EXISTS judge_task_tenant_id_source_ref_key;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'judge_task'::regclass
          AND conname = 'judge_task_tenant_id_source_ref_problem_ref_key'
    ) THEN
        ALTER TABLE judge_task
            ADD CONSTRAINT judge_task_tenant_id_source_ref_problem_ref_key
            UNIQUE (tenant_id, source_ref, problem_ref);
    END IF;
END $$;
