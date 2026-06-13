-- name: GetPlatformAdminByUsername :one
SELECT id, username, password_hash, name, status, created_at, updated_at
FROM platform_admin
WHERE username = $1;

-- name: CreatePlatformAdminIfNotExists :exec
INSERT INTO platform_admin (id, username, password_hash, name, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, now(), now())
ON CONFLICT (username) DO NOTHING;

-- name: GetTenantByCode :one
SELECT id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at
FROM tenant
WHERE code = $1;

-- name: GetTenantByID :one
SELECT id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at
FROM tenant
WHERE id = $1;

-- name: ListTenants :many
SELECT id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at
FROM tenant
ORDER BY created_at DESC, id DESC;

-- name: CreateTenant :one
INSERT INTO tenant (id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now(), now())
RETURNING id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at;

-- name: UpdateTenantConfig :one
UPDATE tenant
SET logo_url = $2, display_name = $3, feature_flags = $4, auth_mode = $5, enable_activation_code = $6, updated_at = now()
WHERE id = $1
RETURNING id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at;

-- name: UpdateTenantStatus :one
UPDATE tenant
SET status = $2, expire_at = $3, updated_at = now()
WHERE id = $1
RETURNING id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at;

-- name: CreateTenantApplication :one
INSERT INTO tenant_application (id, school_name, school_type, contact_name, contact_phone, contact_email, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())
RETURNING id, school_name, school_type, contact_name, contact_phone, contact_email, status, reject_reason, reviewed_by, tenant_id, created_at, updated_at;

-- name: GetTenantApplication :one
SELECT id, school_name, school_type, contact_name, contact_phone, contact_email, status, reject_reason, reviewed_by, tenant_id, created_at, updated_at
FROM tenant_application
WHERE id = $1;

-- name: ListTenantApplications :many
SELECT id, school_name, school_type, contact_name, contact_phone, contact_email, status, reject_reason, reviewed_by, tenant_id, created_at, updated_at
FROM tenant_application
WHERE ($1::smallint = 0 OR status = $1)
ORDER BY created_at DESC, id DESC;

-- name: ApproveTenantApplication :one
UPDATE tenant_application
SET status = $2, reviewed_by = $3, tenant_id = $4, updated_at = now()
WHERE id = $1 AND status = 1
RETURNING id, school_name, school_type, contact_name, contact_phone, contact_email, status, reject_reason, reviewed_by, tenant_id, created_at, updated_at;

-- name: RejectTenantApplication :one
UPDATE tenant_application
SET status = $2, reject_reason = $3, reviewed_by = $4, updated_at = now()
WHERE id = $1 AND status = 1
RETURNING id, school_name, school_type, contact_name, contact_phone, contact_email, status, reject_reason, reviewed_by, tenant_id, created_at, updated_at;

-- name: CreateDepartment :one
INSERT INTO department (id, tenant_id, name, code)
VALUES ($1, $2, $3, $4)
RETURNING id, tenant_id, name, code, deleted_at;

-- name: ListDepartments :many
SELECT id, tenant_id, name, code, deleted_at
FROM department
WHERE deleted_at IS NULL
ORDER BY name, id;

-- name: UpdateDepartment :one
UPDATE department
SET name = $3, code = $4
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, name, code, deleted_at;

-- name: SoftDeleteDepartment :exec
UPDATE department SET deleted_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: DepartmentExists :one
SELECT EXISTS(SELECT 1 FROM department WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL);

-- name: CreateMajor :one
INSERT INTO major (id, tenant_id, department_id, name)
VALUES ($1, $2, $3, $4)
RETURNING id, tenant_id, department_id, name, deleted_at;

-- name: ListMajors :many
SELECT id, tenant_id, department_id, name, deleted_at
FROM major
WHERE deleted_at IS NULL AND ($1::bigint = 0 OR department_id = $1)
ORDER BY name, id;

-- name: MajorExists :one
SELECT EXISTS(SELECT 1 FROM major WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL);

-- name: UpdateMajor :one
UPDATE major
SET department_id = $3, name = $4
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, department_id, name, deleted_at;

-- name: SoftDeleteMajor :exec
UPDATE major SET deleted_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: CreateClass :one
INSERT INTO class (id, tenant_id, major_id, name, enrollment_year, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, tenant_id, major_id, name, enrollment_year, status, deleted_at;

-- name: ListClasses :many
SELECT id, tenant_id, major_id, name, enrollment_year, status, deleted_at
FROM class
WHERE deleted_at IS NULL AND ($1::bigint = 0 OR major_id = $1)
ORDER BY enrollment_year DESC, name, id;

-- name: ClassExists :one
SELECT EXISTS(SELECT 1 FROM class WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL);

-- name: UpdateClass :one
UPDATE class
SET major_id = $3, name = $4, enrollment_year = $5, status = $6
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
RETURNING id, tenant_id, major_id, name, enrollment_year, status, deleted_at;

-- name: SoftDeleteClass :exec
UPDATE class SET deleted_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: ArchiveClassesByEnrollmentYear :exec
UPDATE class
SET status = 2
WHERE tenant_id = $1 AND enrollment_year = $2 AND deleted_at IS NULL;

-- name: ArchiveStudentAccountsByEnrollmentYear :exec
UPDATE account
SET status = 4, updated_at = now()
FROM account_profile
WHERE account.id = account_profile.account_id
  AND account.tenant_id = account_profile.tenant_id
  AND account.tenant_id = $1
  AND account_profile.enrollment_year = $2
  AND account.base_identity = 1
  AND account.status = 2
  AND account.deleted_at IS NULL;

-- name: RevokeStudentSessionsByEnrollmentYear :exec
UPDATE auth_session
SET status = 2
FROM account
JOIN account_profile ON account.id = account_profile.account_id AND account.tenant_id = account_profile.tenant_id
WHERE auth_session.account_id = account.id
  AND auth_session.tenant_id = account.tenant_id
  AND auth_session.tenant_id = $1
  AND account_profile.enrollment_year = $2
  AND account.base_identity = 1
  AND account.status = 4
  AND auth_session.status = 1;

-- name: PromoteClasses :exec
UPDATE class
SET enrollment_year = enrollment_year + 1
WHERE tenant_id = $1 AND status = 1 AND deleted_at IS NULL;

-- name: CreateAccount :one
INSERT INTO account (id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, activated_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now(), now())
RETURNING id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, pwd_failed_count, locked_until, activated_at, deleted_at, created_at, updated_at;

-- name: CreateAccountRole :exec
INSERT INTO account_role (id, tenant_id, account_id, role)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tenant_id, account_id, role) DO NOTHING;

-- name: DeleteAccountRole :exec
DELETE FROM account_role
WHERE tenant_id = $1 AND account_id = $2 AND role = $3;

-- name: CreateAccountProfile :exec
INSERT INTO account_profile (account_id, tenant_id, no, org_id, enrollment_year, title)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetAccountByID :one
SELECT a.id, a.tenant_id, a.phone_enc, a.phone_hash, a.password_hash, a.name, a.base_identity, a.status,
       a.must_change_pwd, a.pwd_failed_count, a.locked_until, a.activated_at, a.deleted_at, a.created_at, a.updated_at,
       p.no, p.org_id, p.enrollment_year, p.title,
       COALESCE(array_agg(ar.role ORDER BY ar.role) FILTER (WHERE ar.role IS NOT NULL), ARRAY[]::smallint[])::smallint[] AS roles
FROM account a
LEFT JOIN account_profile p ON p.account_id = a.id AND p.tenant_id = a.tenant_id
LEFT JOIN account_role ar ON ar.account_id = a.id AND ar.tenant_id = a.tenant_id
WHERE a.id = $1 AND a.deleted_at IS NULL
GROUP BY a.id, p.no, p.org_id, p.enrollment_year, p.title;

-- name: BatchGetAccounts :many
SELECT a.id, a.tenant_id, a.phone_enc, a.phone_hash, a.password_hash, a.name, a.base_identity, a.status,
       a.must_change_pwd, a.pwd_failed_count, a.locked_until, a.activated_at, a.deleted_at, a.created_at, a.updated_at,
       p.no, p.org_id, p.enrollment_year, p.title,
       COALESCE(array_agg(ar.role ORDER BY ar.role) FILTER (WHERE ar.role IS NOT NULL), ARRAY[]::smallint[])::smallint[] AS roles
FROM account a
LEFT JOIN account_profile p ON p.account_id = a.id AND p.tenant_id = a.tenant_id
LEFT JOIN account_role ar ON ar.account_id = a.id AND ar.tenant_id = a.tenant_id
WHERE a.id = ANY($1::bigint[]) AND a.deleted_at IS NULL
GROUP BY a.id, p.no, p.org_id, p.enrollment_year, p.title;

-- name: ListAccountsByPhoneHashPrivileged :many
SELECT a.id, a.tenant_id, a.phone_enc, a.phone_hash, a.password_hash, a.name, a.base_identity, a.status,
       a.must_change_pwd, a.pwd_failed_count, a.locked_until, a.activated_at, a.deleted_at, a.created_at, a.updated_at,
       t.code AS tenant_code, t.name AS tenant_name
FROM account a
JOIN tenant t ON t.id = a.tenant_id
WHERE a.phone_hash = $1 AND a.deleted_at IS NULL
ORDER BY t.name, a.id;

-- name: GetAccountByPhoneHash :one
SELECT a.id, a.tenant_id, a.phone_enc, a.phone_hash, a.password_hash, a.name, a.base_identity, a.status,
       a.must_change_pwd, a.pwd_failed_count, a.locked_until, a.activated_at, a.deleted_at, a.created_at, a.updated_at,
       p.no, p.org_id, p.enrollment_year, p.title,
       COALESCE(array_agg(ar.role ORDER BY ar.role) FILTER (WHERE ar.role IS NOT NULL), ARRAY[]::smallint[])::smallint[] AS roles
FROM account a
LEFT JOIN account_profile p ON p.account_id = a.id AND p.tenant_id = a.tenant_id
LEFT JOIN account_role ar ON ar.account_id = a.id AND ar.tenant_id = a.tenant_id
WHERE a.tenant_id = $1 AND a.phone_hash = $2 AND a.deleted_at IS NULL
GROUP BY a.id, p.no, p.org_id, p.enrollment_year, p.title;

-- name: GetAccountByNo :one
SELECT a.id, a.tenant_id, a.phone_enc, a.phone_hash, a.password_hash, a.name, a.base_identity, a.status,
       a.must_change_pwd, a.pwd_failed_count, a.locked_until, a.activated_at, a.deleted_at, a.created_at, a.updated_at,
       p.no, p.org_id, p.enrollment_year, p.title,
       COALESCE(array_agg(ar.role ORDER BY ar.role) FILTER (WHERE ar.role IS NOT NULL), ARRAY[]::smallint[])::smallint[] AS roles
FROM account a
JOIN account_profile p ON p.account_id = a.id AND p.tenant_id = a.tenant_id
LEFT JOIN account_role ar ON ar.account_id = a.id AND ar.tenant_id = a.tenant_id
WHERE p.no = $1 AND a.deleted_at IS NULL
GROUP BY a.id, p.no, p.org_id, p.enrollment_year, p.title;

-- name: ListAccounts :many
SELECT a.id, a.tenant_id, a.phone_enc, a.phone_hash, a.password_hash, a.name, a.base_identity, a.status,
       a.must_change_pwd, a.pwd_failed_count, a.locked_until, a.activated_at, a.deleted_at, a.created_at, a.updated_at,
       p.no, p.org_id, p.enrollment_year, p.title,
       COALESCE(array_agg(ar.role ORDER BY ar.role) FILTER (WHERE ar.role IS NOT NULL), ARRAY[]::smallint[])::smallint[] AS roles,
       COUNT(*) OVER() AS total_count
FROM account a
LEFT JOIN account_profile p ON p.account_id = a.id AND p.tenant_id = a.tenant_id
LEFT JOIN account_role ar ON ar.account_id = a.id AND ar.tenant_id = a.tenant_id
WHERE a.deleted_at IS NULL
  AND ($1::smallint = 0 OR a.status = $1)
  AND ($2::smallint = 0 OR a.base_identity = $2)
  AND ($3::bigint = 0 OR p.org_id = $3)
  AND ($4::text = '' OR a.name ILIKE '%' || $4 || '%' OR p.no ILIKE '%' || $4 || '%')
GROUP BY a.id, p.no, p.org_id, p.enrollment_year, p.title
ORDER BY a.created_at DESC, a.id DESC
LIMIT $5 OFFSET $6;

-- name: UpdateAccountStatus :one
UPDATE account
SET status = $3, deleted_at = $4, updated_at = now()
WHERE id = $1 AND tenant_id = $2
RETURNING id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, pwd_failed_count, locked_until, activated_at, deleted_at, created_at, updated_at;

-- name: UpdateAccountBasic :exec
UPDATE account
SET name = $3, updated_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: UpdateAccountProfileEditable :exec
UPDATE account_profile
SET org_id = $3, enrollment_year = $4, title = $5
WHERE account_id = $1 AND tenant_id = $2;

-- name: UpdateAccountPassword :one
UPDATE account
SET password_hash = $3, must_change_pwd = $4, status = $5, activated_at = $6, pwd_failed_count = 0, locked_until = NULL, updated_at = now()
WHERE id = $1 AND tenant_id = $2
RETURNING id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, pwd_failed_count, locked_until, activated_at, deleted_at, created_at, updated_at;

-- name: UpdateAccountPhone :exec
UPDATE account
SET phone_enc = $3, phone_hash = $4, updated_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: RecordPasswordFailure :one
UPDATE account
SET pwd_failed_count = $3, locked_until = $4, updated_at = now()
WHERE id = $1 AND tenant_id = $2
RETURNING id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, pwd_failed_count, locked_until, activated_at, deleted_at, created_at, updated_at;

-- name: ClearPasswordFailure :exec
UPDATE account SET pwd_failed_count = 0, locked_until = NULL, updated_at = now()
WHERE id = $1 AND tenant_id = $2;

-- name: RevokeAccountSessions :exec
UPDATE auth_session SET status = 2
WHERE tenant_id = $1 AND account_id = $2 AND status = 1;

-- name: CreateAuthSession :one
INSERT INTO auth_session (id, tenant_id, account_id, refresh_token_hash, device_info, ip, status, expire_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
RETURNING id, tenant_id, account_id, refresh_token_hash, device_info, ip, status, expire_at, created_at;

-- name: GetAuthSessionByRefreshHashPrivileged :one
SELECT id, tenant_id, account_id, refresh_token_hash, device_info, ip, status, expire_at, created_at
FROM auth_session
WHERE refresh_token_hash = $1;

-- name: GetAuthSessionByID :one
SELECT id, tenant_id, account_id, refresh_token_hash, device_info, ip, status, expire_at, created_at
FROM auth_session
WHERE tenant_id = $1 AND id = $2;

-- name: ListAuthSessionsByAccount :many
SELECT id, tenant_id, account_id, refresh_token_hash, device_info, ip, status, expire_at, created_at
FROM auth_session
WHERE tenant_id = $1 AND account_id = $2
ORDER BY created_at DESC, id DESC;

-- name: RevokeAuthSessionByID :exec
UPDATE auth_session SET status = 2
WHERE tenant_id = $1 AND id = $2;

-- name: RevokePlatformSessions :exec
UPDATE platform_auth_session SET status = 2
WHERE platform_admin_id = $1 AND status = 1;

-- name: CreatePlatformAuthSession :one
INSERT INTO platform_auth_session (id, platform_admin_id, refresh_token_hash, device_info, ip, status, expire_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
RETURNING id, platform_admin_id, refresh_token_hash, device_info, ip, status, expire_at, created_at;

-- name: GetPlatformAuthSessionByRefreshHash :one
SELECT id, platform_admin_id, refresh_token_hash, device_info, ip, status, expire_at, created_at
FROM platform_auth_session
WHERE refresh_token_hash = $1;

-- name: GetPlatformAuthSessionByID :one
SELECT id, platform_admin_id, refresh_token_hash, device_info, ip, status, expire_at, created_at
FROM platform_auth_session
WHERE id = $1;

-- name: RevokePlatformAuthSessionByID :exec
UPDATE platform_auth_session SET status = 2
WHERE id = $1;

-- name: CreateSMSCode :one
INSERT INTO sms_code (id, tenant_id, phone_hash, code_hash, scene, expire_at, used, created_at)
VALUES ($1, $2, $3, $4, $5, $6, false, now())
RETURNING id, tenant_id, phone_hash, code_hash, scene, expire_at, verify_attempts, used, created_at;

-- name: GetLatestSMSCode :one
SELECT id, tenant_id, phone_hash, code_hash, scene, expire_at, verify_attempts, used, created_at
FROM sms_code
WHERE tenant_id = $1 AND phone_hash = $2 AND scene = $3
ORDER BY created_at DESC
LIMIT 1;

-- name: MarkSMSCodeUsed :exec
UPDATE sms_code SET used = true
WHERE id = $1 AND tenant_id = $2;

-- name: IncrementSMSVerifyAttempts :exec
UPDATE sms_code SET verify_attempts = verify_attempts + 1
WHERE id = $1 AND tenant_id = $2;

-- name: CreateActivationCode :one
INSERT INTO activation_code (id, tenant_id, account_id, code_hash, status, expire_at, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
RETURNING id, tenant_id, account_id, code_hash, status, expire_at, used_at, created_by, created_at;

-- name: GetActivationCodeByHashPrivileged :one
SELECT id, tenant_id, account_id, code_hash, status, expire_at, used_at, created_by, created_at
FROM activation_code
WHERE code_hash = $1;

-- name: UseActivationCode :exec
UPDATE activation_code SET status = 2, used_at = now()
WHERE id = $1 AND tenant_id = $2 AND status = 1;

-- name: UpsertSSOConfig :one
INSERT INTO sso_config (id, tenant_id, type, config, match_field, enabled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now(), now())
ON CONFLICT (tenant_id, type) DO UPDATE
SET config = EXCLUDED.config, match_field = EXCLUDED.match_field, enabled = EXCLUDED.enabled, updated_at = now()
RETURNING id, tenant_id, type, config, match_field, enabled, created_at, updated_at;

-- name: GetSSOConfig :one
SELECT id, tenant_id, type, config, match_field, enabled, created_at, updated_at
FROM sso_config
WHERE tenant_id = $1 AND type = $2;

-- name: ListSSOConfigs :many
SELECT id, tenant_id, type, config, match_field, enabled, created_at, updated_at
FROM sso_config
WHERE tenant_id = $1
ORDER BY type;

-- name: CreateImportPreview :one
INSERT INTO import_preview (id, tenant_id, operator_id, target_type, file_name, rows, preview_result, status, expire_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
RETURNING id, tenant_id, operator_id, target_type, file_name, rows, preview_result, status, expire_at, submitted_at, created_at;

-- name: GetImportPreview :one
SELECT id, tenant_id, operator_id, target_type, file_name, rows, preview_result, status, expire_at, submitted_at, created_at
FROM import_preview
WHERE id = $1 AND tenant_id = $2;

-- name: MarkImportPreviewSubmitted :exec
UPDATE import_preview SET status = 2, submitted_at = now()
WHERE id = $1 AND tenant_id = $2 AND status = 1;

-- name: CreateImportBatch :one
INSERT INTO import_batch (id, tenant_id, operator_id, target_type, file_name, total, success, failed, error_detail, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
RETURNING id, tenant_id, operator_id, target_type, file_name, total, success, failed, error_detail, status, created_at;

-- name: ListImportBatches :many
SELECT id, tenant_id, operator_id, target_type, file_name, total, success, failed, error_detail, status, created_at
FROM import_batch
WHERE tenant_id = $1
ORDER BY created_at DESC, id DESC;

-- name: CreateAuditLog :one
INSERT INTO audit_log (id, tenant_id, actor_id, actor_role, action, target_type, target_id, detail, ip, trace_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
RETURNING id, tenant_id, actor_id, actor_role, action, target_type, target_id, detail, ip, trace_id, created_at;

-- name: QueryAuditLogs :many
SELECT id, tenant_id, actor_id, actor_role, action, target_type, target_id, detail, ip, trace_id, created_at, COUNT(*) OVER() AS total_count
FROM audit_log
WHERE ($1::bigint = 0 OR tenant_id = $1)
  AND ($2::bigint = 0 OR actor_id = $2)
  AND ($3::text = '' OR action = $3)
  AND ($4::text = '' OR target_type = $4)
  AND ($5::timestamptz IS NULL OR created_at >= $5)
  AND ($6::timestamptz IS NULL OR created_at <= $6)
ORDER BY created_at DESC, id DESC
LIMIT $7 OFFSET $8;

-- name: PlatformStats :one
SELECT
  (SELECT COUNT(*) FROM tenant) AS tenant_count,
  (SELECT COUNT(*) FROM account) AS account_count,
  (SELECT COUNT(*) FROM account WHERE base_identity = 2) AS teacher_count,
  (SELECT COUNT(*) FROM account WHERE base_identity = 1) AS student_count,
  (SELECT COUNT(*) FROM account_role WHERE role = 2) AS school_admin_count,
  (SELECT COUNT(*) FROM platform_admin WHERE status = 1) AS platform_admin_count,
  (SELECT COUNT(*) FROM account WHERE status = 2) AS active_account_count,
  (SELECT COUNT(*) FROM tenant WHERE status = 1) AS active_tenant_count,
  (SELECT COUNT(*) FROM tenant_application WHERE status = 1) AS pending_apply_count,
  (SELECT COUNT(*) FROM account WHERE status = 3) AS disabled_account_count;

-- name: TenantStats :one
SELECT
  COUNT(*) AS account_count,
  COUNT(*) FILTER (WHERE base_identity = 2) AS teacher_count,
  COUNT(*) FILTER (WHERE base_identity = 1) AS student_count,
  COUNT(*) FILTER (WHERE status = 2) AS active_account_count,
  COUNT(*) FILTER (WHERE status = 3) AS disabled_account_count
FROM account
WHERE tenant_id = $1;
