-- M1 identity sqlc 查询源。
-- 约定:租户表查询不显式写 tenant_id 条件(RLS 透明过滤),仅写业务条件;插入带 tenant_id(WITH CHECK)。
-- 雪花 ID 由应用传入。全局表(platform_admin/tenant/tenant_application)无 RLS。
-- 受控特权路径(预认证定位、平台级 tenant_id=NULL 审计/验证码):由 service 在【特权连接】上执行
--   (属主绕 RLS,见 docs/01 §8);sqlc 正常类型化,无手写 SQL。

-- ============================================================
-- platform_admin
-- ============================================================

-- name: GetPlatformAdminByUsername :one
SELECT id, username, password_hash, name, status, created_at, updated_at FROM platform_admin WHERE username = $1;

-- name: GetPlatformAdminByID :one
SELECT id, username, password_hash, name, status, created_at, updated_at FROM platform_admin WHERE id = $1;

-- name: CreatePlatformAdmin :one
INSERT INTO platform_admin (id, username, password_hash, name, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, username, password_hash, name, status, created_at, updated_at;

-- name: CreatePlatformAuthSession :one
INSERT INTO platform_auth_session (id, platform_admin_id, refresh_token_hash, device_info, ip, status, expire_at)
VALUES ($1, $2, $3, $4, $5, 1, $6)
RETURNING id, platform_admin_id, refresh_token_hash, device_info, ip, status, expire_at, created_at;

-- name: FindPlatformSessionByTokenHash :one
SELECT id, platform_admin_id, status, expire_at FROM platform_auth_session WHERE refresh_token_hash = $1;

-- name: RevokePlatformAuthSession :exec
UPDATE platform_auth_session SET status = 2 WHERE id = $1;

-- name: RevokeAllPlatformAdminSessions :exec
UPDATE platform_auth_session SET status = 2 WHERE platform_admin_id = $1 AND status = 1;

-- ============================================================
-- tenant
-- ============================================================

-- name: GetTenantByID :one
SELECT id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at FROM tenant WHERE id = $1;

-- name: GetTenantByCode :one
SELECT id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at FROM tenant WHERE code = $1;

-- name: ListTenants :many
SELECT id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at FROM tenant
WHERE (sqlc.narg('status')::SMALLINT IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountTenants :one
SELECT count(*) FROM tenant
WHERE (sqlc.narg('status')::SMALLINT IS NULL OR status = sqlc.narg('status'));

-- name: CountAllAccounts :one
SELECT count(*)::bigint FROM account
WHERE status <> 5
  AND (sqlc.narg('base_identity')::SMALLINT IS NULL OR base_identity = sqlc.narg('base_identity'));

-- name: CountTenantApplications :one
SELECT count(*)::bigint FROM tenant_application
WHERE (sqlc.narg('status')::SMALLINT IS NULL OR status = sqlc.narg('status'));

-- name: CreateTenant :one
INSERT INTO tenant (id, code, name, type, status, deploy_mode, expire_at, auth_mode, enable_activation_code)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at;

-- name: UpdateTenantStatus :one
UPDATE tenant SET status = $2, expire_at = $3 WHERE id = $1
RETURNING id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at;

-- name: UpdateTenantConfig :one
UPDATE tenant
SET logo_url = $2, display_name = $3, feature_flags = $4, auth_mode = $5, enable_activation_code = $6
WHERE id = $1
RETURNING id, code, name, type, status, deploy_mode, expire_at, logo_url, display_name, feature_flags, auth_mode, enable_activation_code, created_at, updated_at;

-- ============================================================
-- tenant_application
-- ============================================================

-- name: CreateTenantApplication :one
INSERT INTO tenant_application (id, school_name, school_type, contact_name, contact_phone, contact_email, status)
VALUES ($1, $2, $3, $4, $5, $6, 1)
RETURNING id, school_name, school_type, contact_name, contact_phone, contact_email, status, reject_reason, reviewed_by, tenant_id, created_at, updated_at;

-- name: GetTenantApplicationByID :one
SELECT id, school_name, school_type, contact_name, contact_phone, contact_email, status, reject_reason, reviewed_by, tenant_id, created_at, updated_at FROM tenant_application WHERE id = $1;

-- name: ListTenantApplications :many
SELECT id, school_name, school_type, contact_name, contact_phone, contact_email, status, reject_reason, reviewed_by, tenant_id, created_at, updated_at FROM tenant_application
WHERE (sqlc.narg('status')::SMALLINT IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ApproveTenantApplication :one
UPDATE tenant_application SET status = 2, reviewed_by = $2, tenant_id = $3
WHERE id = $1 AND status = 1
RETURNING id, school_name, school_type, contact_name, contact_phone, contact_email, status, reject_reason, reviewed_by, tenant_id, created_at, updated_at;

-- name: RejectTenantApplication :one
UPDATE tenant_application SET status = 3, reviewed_by = $2, reject_reason = $3
WHERE id = $1 AND status = 1
RETURNING id, school_name, school_type, contact_name, contact_phone, contact_email, status, reject_reason, reviewed_by, tenant_id, created_at, updated_at;

-- ============================================================
-- department / major / class(租户表,RLS 透明)
-- ============================================================

-- name: CreateDepartment :one
INSERT INTO department (id, tenant_id, name, code) VALUES ($1, $2, $3, $4) RETURNING id, tenant_id, name, code, created_at, updated_at, deleted_at;

-- name: GetDepartmentByID :one
SELECT id, tenant_id, name, code, created_at, updated_at, deleted_at FROM department WHERE id = $1 AND deleted_at IS NULL;

-- name: ListDepartments :many
SELECT id, tenant_id, name, code, created_at, updated_at, deleted_at FROM department WHERE deleted_at IS NULL ORDER BY name;

-- name: UpdateDepartment :one
UPDATE department SET name = $2, code = $3 WHERE id = $1 AND deleted_at IS NULL RETURNING id, tenant_id, name, code, created_at, updated_at, deleted_at;

-- name: SoftDeleteDepartment :exec
UPDATE department SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateMajor :one
INSERT INTO major (id, tenant_id, department_id, name) VALUES ($1, $2, $3, $4) RETURNING id, tenant_id, department_id, name, created_at, updated_at, deleted_at;

-- name: GetMajorByID :one
SELECT id, tenant_id, department_id, name, created_at, updated_at, deleted_at FROM major WHERE id = $1 AND deleted_at IS NULL;

-- name: ListMajorsByDepartment :many
SELECT id, tenant_id, department_id, name, created_at, updated_at, deleted_at FROM major WHERE department_id = $1 AND deleted_at IS NULL ORDER BY name;

-- name: UpdateMajor :one
UPDATE major SET name = $2 WHERE id = $1 AND deleted_at IS NULL RETURNING id, tenant_id, department_id, name, created_at, updated_at, deleted_at;

-- name: SoftDeleteMajor :exec
UPDATE major SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateClass :one
INSERT INTO class (id, tenant_id, major_id, name, enrollment_year, status)
VALUES ($1, $2, $3, $4, $5, 1) RETURNING id, tenant_id, major_id, name, enrollment_year, status, created_at, updated_at, deleted_at;

-- name: GetClassByID :one
SELECT id, tenant_id, major_id, name, enrollment_year, status, created_at, updated_at, deleted_at FROM class WHERE id = $1 AND deleted_at IS NULL;

-- name: ListClassesByMajor :many
SELECT id, tenant_id, major_id, name, enrollment_year, status, created_at, updated_at, deleted_at FROM class WHERE major_id = $1 AND deleted_at IS NULL ORDER BY enrollment_year DESC, name;

-- name: UpdateClass :one
UPDATE class SET name = $2, enrollment_year = $3 WHERE id = $1 AND deleted_at IS NULL RETURNING id, tenant_id, major_id, name, enrollment_year, status, created_at, updated_at, deleted_at;

-- name: ArchiveClass :exec
UPDATE class SET status = 2 WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteClass :exec
UPDATE class SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: ArchiveAccountsByClass :exec
-- 班级归档级联:该班级在读学生账号一并归档(status 2正常 → 4归档)。
UPDATE account SET status = 4
WHERE base_identity = 1 AND status = 2
  AND id IN (SELECT account_id FROM account_profile WHERE org_id = $1);

-- ============================================================
-- account
-- ============================================================

-- name: CreateAccount :one
INSERT INTO account (id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, pwd_failed_count, locked_until, activated_at, created_at, updated_at;

-- name: GetAccountByID :one
SELECT id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, pwd_failed_count, locked_until, activated_at, created_at, updated_at FROM account WHERE id = $1;

-- name: GetAccountByPhoneHash :one
SELECT id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, pwd_failed_count, locked_until, activated_at, created_at, updated_at FROM account WHERE phone_hash = $1;

-- name: FindAccountsByPhoneAllTenants :many
-- 一号多校登录定位:跨租户按 phone_hash 查账号。
-- ★ 必须在特权连接(属主,绕 RLS)上执行;返回登录定位最小字段(不含手机号/密码)。
SELECT a.id AS account_id, a.tenant_id, a.name, t.code AS tenant_code, t.name AS tenant_name, t.status AS tenant_status
FROM account a
JOIN tenant t ON t.id = a.tenant_id
WHERE a.phone_hash = $1 AND a.status <> 5;

-- name: ListAccounts :many
SELECT a.id, a.tenant_id, a.phone_enc, a.phone_hash, a.password_hash, a.name, a.base_identity, a.status, a.must_change_pwd, a.pwd_failed_count, a.locked_until, a.activated_at, a.created_at, a.updated_at FROM account a
LEFT JOIN account_profile p ON p.account_id = a.id
WHERE (sqlc.narg('status')::SMALLINT IS NULL OR a.status = sqlc.narg('status'))
  AND (sqlc.narg('role')::SMALLINT IS NULL OR EXISTS (
    SELECT 1 FROM account_role ar WHERE ar.account_id = a.id AND ar.role = sqlc.narg('role')
  ))
  AND (sqlc.narg('class_id')::BIGINT IS NULL OR (
    a.base_identity = 1 AND p.org_id = sqlc.narg('class_id')
  ))
  AND (sqlc.narg('keyword')::TEXT IS NULL OR a.name ILIKE '%' || sqlc.narg('keyword') || '%')
ORDER BY a.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAccounts :one
SELECT count(*) FROM account a
LEFT JOIN account_profile p ON p.account_id = a.id
WHERE (sqlc.narg('status')::SMALLINT IS NULL OR a.status = sqlc.narg('status'))
  AND (sqlc.narg('base_identity')::SMALLINT IS NULL OR a.base_identity = sqlc.narg('base_identity'))
  AND (sqlc.narg('role')::SMALLINT IS NULL OR EXISTS (
    SELECT 1 FROM account_role ar WHERE ar.account_id = a.id AND ar.role = sqlc.narg('role')
  ))
  AND (sqlc.narg('class_id')::BIGINT IS NULL OR (
    a.base_identity = 1 AND p.org_id = sqlc.narg('class_id')
  ))
  AND (sqlc.narg('keyword')::TEXT IS NULL OR a.name ILIKE '%' || sqlc.narg('keyword') || '%');

-- name: UpdateAccountName :one
UPDATE account SET name = $2 WHERE id = $1 RETURNING id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, pwd_failed_count, locked_until, activated_at, created_at, updated_at;

-- name: UpdateAccountStatus :one
UPDATE account SET status = $2 WHERE id = $1 RETURNING id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, pwd_failed_count, locked_until, activated_at, created_at, updated_at;

-- name: ArchiveStudentAccountsByEnrollmentYear :many
-- 学年归档:仅归档当前租户内正常状态学生账号,避免教师或停用/注销账号被误改。
UPDATE account a SET status = 4
FROM account_profile p
WHERE p.account_id = a.id
  AND a.base_identity = 1
  AND a.status = 2
  AND p.enrollment_year = $1
RETURNING a.id;

-- name: UpdateAccountPassword :exec
UPDATE account SET password_hash = $2, must_change_pwd = $3 WHERE id = $1;

-- name: SetAccountActivated :exec
UPDATE account SET status = 2, must_change_pwd = false, activated_at = now() WHERE id = $1;

-- name: UpdateAccountPhone :exec
UPDATE account SET phone_enc = $2, phone_hash = $3 WHERE id = $1;

-- name: IncrAccountPwdFailed :one
-- 密码失败计数 +1,达阈值($2)则锁定 $3 分钟;返回更新后账号。
UPDATE account
SET pwd_failed_count = pwd_failed_count + 1,
    locked_until = CASE WHEN pwd_failed_count + 1 >= $2
                        THEN now() + ($3 || ' minutes')::interval
                        ELSE locked_until END
WHERE id = $1
RETURNING id, tenant_id, phone_enc, phone_hash, password_hash, name, base_identity, status, must_change_pwd, pwd_failed_count, locked_until, activated_at, created_at, updated_at;

-- name: ResetAccountPwdFailed :exec
UPDATE account SET pwd_failed_count = 0, locked_until = NULL WHERE id = $1;

-- ============================================================
-- account_role
-- ============================================================

-- name: AddAccountRole :exec
INSERT INTO account_role (id, tenant_id, account_id, role)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tenant_id, account_id, role) DO NOTHING;

-- name: RemoveAccountRole :exec
DELETE FROM account_role WHERE account_id = $1 AND role = $2;

-- name: ListAccountRoles :many
SELECT role FROM account_role WHERE account_id = $1 ORDER BY role;

-- name: HasAccountRole :one
SELECT EXISTS(SELECT 1 FROM account_role WHERE account_id = $1 AND role = $2);

-- ============================================================
-- account_profile
-- ============================================================

-- name: CreateAccountProfile :one
INSERT INTO account_profile (account_id, tenant_id, no, org_id, enrollment_year, title)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING account_id, tenant_id, no, org_id, enrollment_year, title;

-- name: GetAccountProfile :one
SELECT account_id, tenant_id, no, org_id, enrollment_year, title FROM account_profile WHERE account_id = $1;

-- name: GetAccountProfileByNo :one
SELECT account_id, tenant_id, no, org_id, enrollment_year, title FROM account_profile WHERE no = $1;

-- name: UpdateAccountProfileOrg :exec
UPDATE account_profile SET org_id = $2, title = $3 WHERE account_id = $1;

-- ============================================================
-- auth_session
-- ============================================================

-- name: CreateAuthSession :one
INSERT INTO auth_session (id, tenant_id, account_id, refresh_token_hash, device_info, ip, status, expire_at)
VALUES ($1, $2, $3, $4, $5, $6, 1, $7)
RETURNING id, tenant_id, account_id, refresh_token_hash, device_info, ip, status, expire_at, created_at;

-- name: FindSessionByTokenHash :one
-- Refresh 轮转/重放检测:跨租户按 refresh_token_hash 定位会话。
-- ★ 必须在特权连接(属主,绕 RLS)上执行(Refresh Token 不含租户信息)。
SELECT id, tenant_id, account_id, status, expire_at FROM auth_session WHERE refresh_token_hash = $1;

-- name: RevokeAuthSession :exec
UPDATE auth_session SET status = 2 WHERE id = $1;

-- name: RevokeAllAccountSessions :exec
-- 单端登录踢人 / Refresh 重放检测:吊销该账号全部有效会话。
UPDATE auth_session SET status = 2 WHERE account_id = $1 AND status = 1;

-- name: ListActiveSessions :many
SELECT id, tenant_id, account_id, refresh_token_hash, device_info, ip, status, expire_at, created_at FROM auth_session WHERE account_id = $1 AND status = 1 ORDER BY created_at DESC;

-- ============================================================
-- sms_code
-- ============================================================

-- name: CreateSmsCode :one
INSERT INTO sms_code (id, tenant_id, phone_hash, code_hash, scene, expire_at, used)
VALUES ($1, $2, $3, $4, $5, $6, false)
RETURNING id, tenant_id, phone_hash, code_hash, scene, expire_at, used, created_at;

-- name: GetLatestSmsCode :one
SELECT id, tenant_id, phone_hash, code_hash, scene, expire_at, used, created_at FROM sms_code
WHERE phone_hash = $1 AND scene = $2 AND used = false AND expire_at > now()
ORDER BY created_at DESC LIMIT 1;

-- name: MarkSmsCodeUsed :exec
UPDATE sms_code SET used = true WHERE id = $1;

-- ============================================================
-- activation_code
-- ============================================================

-- name: CreateActivationCode :one
INSERT INTO activation_code (id, tenant_id, account_id, code_hash, status, expire_at, created_by)
VALUES ($1, $2, $3, $4, 1, $5, $6)
RETURNING id, tenant_id, account_id, code_hash, status, expire_at, used_at, created_by, created_at;

-- name: GetActivationCodeByHash :one
-- 登录前激活码定位:必须在特权连接上执行,仅返回激活码最小字段用于定位租户与账号。
SELECT id, tenant_id, account_id, code_hash, status, expire_at, used_at, created_by, created_at FROM activation_code WHERE code_hash = $1;

-- name: MarkActivationCodeUsed :exec
UPDATE activation_code SET status = 2, used_at = now() WHERE id = $1 AND status = 1;

-- ============================================================
-- sso_config
-- ============================================================

-- name: GetSsoConfig :one
SELECT id, tenant_id, type, config, match_field, enabled, created_at, updated_at FROM sso_config WHERE tenant_id = $1 AND enabled = true LIMIT 1;

-- name: UpsertSsoConfig :one
INSERT INTO sso_config (id, tenant_id, type, config, match_field, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (tenant_id) DO UPDATE
SET type = EXCLUDED.type,
    config = EXCLUDED.config,
    match_field = EXCLUDED.match_field,
    enabled = EXCLUDED.enabled,
    updated_at = now()
RETURNING id, tenant_id, type, config, match_field, enabled, created_at, updated_at;

-- ============================================================
-- import_batch
-- ============================================================

-- name: CreateImportPreview :one
INSERT INTO import_preview (id, tenant_id, operator_id, target_type, file_name, rows, preview_result, status, expire_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8)
RETURNING id, tenant_id, operator_id, target_type, file_name, rows, preview_result, status, expire_at, created_at, submitted_at;

-- name: GetPendingImportPreview :one
SELECT id, tenant_id, operator_id, target_type, file_name, rows, preview_result, status, expire_at, created_at, submitted_at FROM import_preview
WHERE id = $1 AND operator_id = $2 AND status = 1 AND expire_at > now()
FOR UPDATE;

-- name: MarkImportPreviewSubmitted :exec
UPDATE import_preview SET status = 2, submitted_at = now()
WHERE id = $1 AND operator_id = $2 AND status = 1;

-- name: CreateImportBatch :one
INSERT INTO import_batch (id, tenant_id, operator_id, target_type, file_name, total, success, failed, error_detail, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, tenant_id, operator_id, target_type, file_name, total, success, failed, error_detail, status, created_at;

-- name: ListImportBatches :many
SELECT id, tenant_id, operator_id, target_type, file_name, total, success, failed, error_detail, status, created_at FROM import_batch ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CountImportBatches :one
SELECT count(*)::bigint FROM import_batch;

-- ============================================================
-- audit_log(全平台唯一审计表)
-- ============================================================

-- name: CreateAuditLog :exec
INSERT INTO audit_log (id, tenant_id, actor_id, actor_role, action, target_type, target_id, detail, ip, trace_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListAuditLogs :many
SELECT id, tenant_id, actor_id, actor_role, action, target_type, target_id, detail, ip, trace_id, created_at FROM audit_log
WHERE (sqlc.narg('actor_id')::BIGINT IS NULL OR actor_id = sqlc.narg('actor_id'))
  AND (sqlc.narg('action')::TEXT IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('target_type')::TEXT IS NULL OR target_type = sqlc.narg('target_type'))
  AND (sqlc.narg('from_time')::TIMESTAMPTZ IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::TIMESTAMPTZ IS NULL OR created_at <= sqlc.narg('to_time'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAuditLogs :one
SELECT count(*)::bigint FROM audit_log
WHERE (sqlc.narg('actor_id')::BIGINT IS NULL OR actor_id = sqlc.narg('actor_id'))
  AND (sqlc.narg('action')::TEXT IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('target_type')::TEXT IS NULL OR target_type = sqlc.narg('target_type'))
  AND (sqlc.narg('from_time')::TIMESTAMPTZ IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::TIMESTAMPTZ IS NULL OR created_at <= sqlc.narg('to_time'));

-- name: ListPlatformAuditLogs :many
-- 平台管理员查平台级与全校审计;必须走特权连接,由 M1 contract 收敛权限入口。
SELECT id, tenant_id, actor_id, actor_role, action, target_type, target_id, detail, ip, trace_id, created_at FROM audit_log
WHERE (sqlc.narg('actor_id')::BIGINT IS NULL OR actor_id = sqlc.narg('actor_id'))
  AND (sqlc.narg('action')::TEXT IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('target_type')::TEXT IS NULL OR target_type = sqlc.narg('target_type'))
  AND (sqlc.narg('from_time')::TIMESTAMPTZ IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::TIMESTAMPTZ IS NULL OR created_at <= sqlc.narg('to_time'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPlatformAuditLogs :one
SELECT count(*)::bigint FROM audit_log
WHERE (sqlc.narg('actor_id')::BIGINT IS NULL OR actor_id = sqlc.narg('actor_id'))
  AND (sqlc.narg('action')::TEXT IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('target_type')::TEXT IS NULL OR target_type = sqlc.narg('target_type'))
  AND (sqlc.narg('from_time')::TIMESTAMPTZ IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::TIMESTAMPTZ IS NULL OR created_at <= sqlc.narg('to_time'));
