DROP POLICY IF EXISTS transcript_record_tenant_rls ON transcript_record;
DROP POLICY IF EXISTS academic_warning_tenant_rls ON academic_warning;
DROP POLICY IF EXISTS grade_appeal_tenant_rls ON grade_appeal;
DROP POLICY IF EXISTS student_semester_grade_tenant_rls ON student_semester_grade;
DROP POLICY IF EXISTS grade_review_tenant_rls ON grade_review;
DROP POLICY IF EXISTS semester_tenant_rls ON semester;
DROP POLICY IF EXISTS grade_level_config_tenant_rls ON grade_level_config;

DROP TABLE IF EXISTS transcript_record;
DROP TABLE IF EXISTS academic_warning;
DROP TABLE IF EXISTS grade_appeal;
DROP TABLE IF EXISTS student_semester_grade;
DROP TABLE IF EXISTS grade_review;
DROP TABLE IF EXISTS semester;
DROP TABLE IF EXISTS grade_level_config;
