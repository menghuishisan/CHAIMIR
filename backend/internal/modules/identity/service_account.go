// M1 账号管理服务:创建/更新/状态机迁移/授撤管理员/重置密码/个人中心。
// 依据 docs/01 §3 接口、§4 权限、§5 状态机。
package identity

import (
	"context"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreateAccount 学校管理员单个建账号(待激活)。初始密码为空则生成临时密码 + 首登改密。
func (s *Service) CreateAccount(ctx context.Context, req CreateAccountRequest) (*CreateAccountResult, error) {
	if req.BaseIdentity != BaseIdentityStudent && req.BaseIdentity != BaseIdentityTeacher {
		return nil, apperr.ErrAccountIdentityInvalid
	}
	if !validCNPhone(req.Phone) {
		return nil, apperr.ErrPhoneInvalid
	}
	orgID, ok := ids.Parse(req.OrgID)
	if !ok {
		return nil, apperr.ErrAccountOrgIDInvalid
	}

	tenantID := tenantFromCtx(ctx)
	enableActivationCode, err := s.activationCodeEnabled(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	initPlain := req.InitPassword
	if initPlain == "" {
		initPlain, err = s.genTempPassword()
		if err != nil {
			return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
		}
	}
	if !enableActivationCode && !validPassword(initPlain) {
		return nil, apperr.ErrWeakPassword
	}
	credential, err := buildAccountOpeningCredential(enableActivationCode, initPlain)
	if err != nil {
		return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
	}

	phoneEnc, err := s.encryptPhone(req.Phone)
	if err != nil {
		return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	ph := s.phoneHash(req.Phone)
	accountID := s.idgen.Generate()
	actorID := int64(0)
	if id, ok := CurrentIdentity(ctx); ok {
		actorID = id.AccountID
	}
	var activationCode string

	// 激活码明文只在 service 内生成并返回一次,repo 只接收哈希和过期时间。
	activationCodeHash := ""
	var activationExpireAt time.Time
	if credential.NeedsActivationCode {
		code, err := s.genActivationCode()
		if err != nil {
			return nil, apperr.ErrActivationCodeIssueFailed.WithCause(err)
		}
		activationCode = code
		activationCodeHash = s.activationCodeHash(code)
		activationExpireAt = timex.Now().Add(s.activationCodeTTL)
	}
	auditEntry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionAccountCreate, AuditTargetAccount, accountID, map[string]any{
		"base_identity":          req.BaseIdentity,
		"org_id":                 ids.Format(orgID),
		"enable_activation_code": enableActivationCode,
	})
	if err != nil {
		return nil, err
	}
	if err := s.repo.createAccountWithAudit(ctx, tenantID, accountID, orgID, req, phoneEnc, ph, credential, activationCodeHash, activationExpireAt, actorID, s.idgen.Generate, buildAuditLogCreate(s.idgen.Generate(), auditEntry)); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrAccountMutationFailed.WithCause(err)
	}

	res := &CreateAccountResult{ID: ids.Format(accountID)}
	if credential.NeedsActivationCode {
		res.ActivationCode = activationCode
	} else {
		res.InitPassword = credential.InitPassword // 系统生成时返回供管理员转交,仅此一次。
	}
	return res, nil
}

// UpdateAccount 更新账号可变字段(姓名;不含学号/工号/身份)。
func (s *Service) UpdateAccount(ctx context.Context, accountID int64, req UpdateAccountRequest) error {
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionAccountUpdate, AuditTargetAccount, accountID, map[string]any{
		"fields": []string{"name"},
	})
	if err != nil {
		return err
	}
	return accountMutationErr(s.repo.updateAccountNameWithAudit(ctx, accountID, req.Name, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrAccountMutationFailed)
}

// SetAccountStatus 状态机迁移(停用/启用/归档/恢复/注销)。校验合法迁移。
func (s *Service) SetAccountStatus(ctx context.Context, accountID int64, target int16) error {
	tenantID := tenantFromCtx(ctx)
	acc, err := s.repo.loadAccountForMutation(ctx, tenantID, accountID)
	if err != nil {
		return accountMutationErr(err, apperr.ErrAccountMutationFailed)
	}
	if !canTransit(acc.Status, target) {
		return apperr.ErrAccountStatusTransitionInvalid
	}
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionAccountStatus, AuditTargetAccount, accountID, map[string]any{
		"from_status": acc.Status,
		"to_status":   target,
	})
	if err != nil {
		return err
	}
	return accountMutationErr(s.repo.updateAccountStatusWithAudit(ctx, accountID, tenantID, target, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrAccountMutationFailed)
}

// BatchSetAccountStatus 批量迁移账号状态,逐账号返回结果并保留状态机校验。
func (s *Service) BatchSetAccountStatus(ctx context.Context, accountIDs []string, target int16) (*BatchAccountStatusResult, error) {
	if len(accountIDs) == 0 {
		return nil, apperr.ErrBatchAccountIDsInvalid
	}
	result := &BatchAccountStatusResult{Total: len(accountIDs)}
	for _, rawID := range accountIDs {
		accountID, ok := ids.Parse(rawID)
		row := BatchAccountStatus{AccountID: rawID}
		if !ok {
			row.Error = "账号 ID 不正确"
			result.Failed++
			result.Rows = append(result.Rows, row)
			continue
		}
		if err := s.SetAccountStatus(ctx, accountID, target); err != nil {
			row.Error = "状态变更失败"
			if ae, ok := apperr.As(err); ok {
				row.Error = ae.Message
			}
			result.Failed++
			result.Rows = append(result.Rows, row)
			continue
		}
		result.Success++
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

// BatchArchiveAccounts 按学年归档当前租户内正常状态的学生账号。
func (s *Service) BatchArchiveAccounts(ctx context.Context, req BatchArchiveAccountsRequest) (*BatchAccountStatusResult, error) {
	if req.EnrollmentYear <= 0 {
		return nil, apperr.ErrBatchAccountArchiveInvalid
	}
	archived, err := s.repo.archiveStudentAccountsByEnrollmentYearWithAudit(ctx, req.EnrollmentYear, func(rows []int64) (AuditLogCreate, error) {
		entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionAccountStatus, AuditTargetAccount, 0, map[string]any{
			"enrollment_year": req.EnrollmentYear,
			"base_identity":   BaseIdentityStudent,
			"from_status":     AccountActive,
			"to_status":       AccountArchived,
			"archived_count":  len(rows),
		})
		if err != nil {
			return AuditLogCreate{}, err
		}
		return buildAuditLogCreate(s.idgen.Generate(), entry), nil
	})
	if err != nil {
		return nil, toAppErr(err)
	}

	result := &BatchAccountStatusResult{Total: len(archived), Success: len(archived)}
	for _, accountID := range archived {
		result.Rows = append(result.Rows, BatchAccountStatus{AccountID: ids.Format(accountID)})
	}
	return result, nil
}

// ForceLogout 吊销某账号全部会话(管理员踢人)。
func (s *Service) ForceLogout(ctx context.Context, accountID int64) error {
	tenantID := tenantFromCtx(ctx)
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionAccountForceLogout, AuditTargetAccount, accountID, nil)
	if err != nil {
		return err
	}
	return accountMutationErr(s.repo.forceLogoutWithAudit(ctx, tenantID, accountID, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrAccountMutationFailed)
}

// ResetAccountPassword 管理员重置他人密码:生成临时密码 + 首登改密,返回临时密码。
func (s *Service) ResetAccountPassword(ctx context.Context, accountID int64) (string, error) {
	temp, err := s.genTempPassword()
	if err != nil {
		return "", apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	hash, err := crypto.HashPassword(temp)
	if err != nil {
		return "", apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionAccountResetPwd, AuditTargetAccount, accountID, map[string]any{
		"must_change_pwd":  true,
		"sessions_revoked": true,
	})
	if err != nil {
		return "", err
	}
	if err := s.repo.resetAccountPasswordWithAudit(ctx, tenantFromCtx(ctx), accountID, hash, buildAuditLogCreate(s.idgen.Generate(), entry)); err != nil {
		return "", accountMutationErr(err, apperr.ErrAccountMutationFailed)
	}
	return temp, nil
}

// GrantAdmin 授予学校管理员(仅教师账号可被授予)。
func (s *Service) GrantAdmin(ctx context.Context, accountID int64) error {
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionAccountGrantAdmin, AuditTargetAccount, accountID, nil)
	if err != nil {
		return err
	}
	return accountMutationErr(s.repo.grantAdminWithAudit(ctx, accountID, s.idgen.Generate(), buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrAccountMutationFailed)
}

// RevokeAdmin 撤销学校管理员角色(保留教师身份)。
func (s *Service) RevokeAdmin(ctx context.Context, accountID int64) error {
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionAccountRevokeAdmin, AuditTargetAccount, accountID, nil)
	if err != nil {
		return err
	}
	return accountMutationErr(s.repo.revokeAdminWithAudit(ctx, tenantFromCtx(ctx), accountID, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrAccountMutationFailed)
}

// ChangeMyPassword 本人改密(校验旧密码或首登改密)。
func (s *Service) ChangeMyPassword(ctx context.Context, accountID int64, req ChangePasswordRequest) error {
	if !validPassword(req.NewPassword) {
		return apperr.ErrWeakPassword
	}
	hash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	tenantID := tenantFromCtx(ctx)
	acc, err := s.repo.loadAccountForMutation(ctx, tenantID, accountID)
	if err != nil {
		return toAppErrWith(err, apperr.ErrAccountMutationFailed)
	}
	// 非首登改密场景必须校验旧密码,首登改密只校验新密码强度并完成激活。
	if !acc.MustChangePwd {
		if !acc.HasPassword {
			return apperr.ErrOldPasswordWrong
		}
		ok, verifyErr := crypto.VerifyPassword(req.OldPassword, acc.PasswordHash)
		if verifyErr != nil {
			return apperr.ErrAccountCredentialFailed.WithCause(verifyErr)
		}
		if !ok {
			return apperr.ErrOldPasswordWrong
		}
	}
	entry, err := buildAuditEntry(ctx, audit.ActorRoleFromAccount(contracts.AccountInfo{
		BaseIdentity: acc.BaseIdentity,
		Roles:        acc.Roles,
	}), AuditActionAccountUpdate, AuditTargetAccount, accountID, map[string]any{
		"fields":          []string{"password"},
		"activated_after": acc.Status == AccountPending,
	})
	if err != nil {
		return err
	}
	return toAppErrWith(s.repo.changeAccountPasswordWithAudit(ctx, tenantID, accountID, hash, acc.Status == AccountPending, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrAccountMutationFailed)
}

// ChangeMyPhone 本人换绑手机(校验验证码)。
func (s *Service) ChangeMyPhone(ctx context.Context, tenantID, accountID int64, req ChangePhoneRequest) error {
	if !validCNPhone(req.NewPhone) {
		return apperr.ErrPhoneInvalid
	}
	newHash := s.phoneHash(req.NewPhone)
	if err := s.verifySmsCode(ctx, tenantID, newHash, SmsSceneRebind, req.Code); err != nil {
		return err
	}
	enc, err := s.encryptPhone(req.NewPhone)
	if err != nil {
		return apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	acc, err := s.repo.loadAccountForMutation(ctx, tenantID, accountID)
	if err != nil {
		return toAppErrWith(err, apperr.ErrAccountMutationFailed)
	}
	entry, err := buildAuditEntry(ctx, audit.ActorRoleFromAccount(contracts.AccountInfo{
		BaseIdentity: acc.BaseIdentity,
		Roles:        acc.Roles,
	}), AuditActionAccountUpdate, AuditTargetAccount, accountID, map[string]any{
		"fields": []string{"phone"},
	})
	if err != nil {
		return err
	}
	return toAppErrWith(s.repo.changeAccountPhoneWithAudit(ctx, tenantID, accountID, enc, newHash, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrAccountMutationFailed)
}

// GetMe 个人中心信息(含学籍只读字段)。
func (s *Service) GetMe(ctx context.Context, accountID int64) (*MeView, error) {
	acc, err := s.repo.loadAccountForMutation(ctx, tenantFromCtx(ctx), accountID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrAccountQueryFailed.WithCause(err)
	}
	phone, err := s.decryptPhone(acc.PhoneEnc)
	if err != nil {
		return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	view := MeView{
		ID: ids.Format(acc.ID), Name: acc.Name, Phone: maskPhone(phone),
		BaseIdentity: acc.BaseIdentity, Roles: acc.Roles,
		No: acc.No, Title: acc.Title,
	}
	if acc.OrgID != 0 {
		view.OrgID = ids.Format(acc.OrgID)
	}
	return &view, nil
}

// ListMySessions 查询当前账号有效会话。
func (s *Service) ListMySessions(ctx context.Context, accountID int64) ([]SessionView, error) {
	rows, err := s.repo.listActiveSessions(ctx, tenantFromCtx(ctx), accountID)
	if err != nil {
		return nil, apperr.ErrAuthSessionQueryFailed.WithCause(err)
	}
	views := make([]SessionView, 0, len(rows))
	for _, row := range rows {
		views = append(views, SessionView{
			ID: ids.Format(row.ID), DeviceInfo: row.DeviceInfo, IP: row.IP,
			ExpireAt: row.ExpireAt.Format(time.RFC3339), CreatedAt: row.CreatedAt.Format(time.RFC3339),
		})
	}
	return views, nil
}

// ---- 内部辅助 ----

// accountMutationErr 保留 repo 返回的业务错误,其余底层错误转为账号场景错误。
func accountMutationErr(err error, code *apperr.Error) error {
	if err == nil {
		return nil
	}
	if ae, ok := apperr.As(err); ok {
		return ae
	}
	return code.WithCause(err)
}

// canTransit 账号状态机合法迁移(docs/01 §5)。
func canTransit(from, to int16) bool {
	switch from {
	case AccountActive:
		return to == AccountDisabled || to == AccountArchived || to == AccountCancelled
	case AccountDisabled:
		return to == AccountActive || to == AccountCancelled
	case AccountArchived:
		return to == AccountActive || to == AccountCancelled
	case AccountPending:
		return to == AccountActive || to == AccountCancelled
	default:
		return false // 注销为终态。
	}
}

// roleCodesOf 角色枚举切片 → 编码切片。
func roleCodesOf(nums []int16) []string {
	out := make([]string, 0, len(nums))
	for _, r := range nums {
		out = append(out, contracts.RoleCode(r))
	}
	return out
}

// textVal 取 pgtype.Text 值。
func textVal(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

// tenantFromCtx 从 ctx 取租户 ID(已由鉴权中间件注入;RLS 事务内必有)。
func tenantFromCtx(ctx context.Context) int64 {
	id, _ := tenantIdentity(ctx)
	return id
}

// isUniqueViolation 判断是否唯一约束冲突(PostgreSQL 23505)。
func isUniqueViolation(err error) bool {
	return err != nil && pgErrCode(err) == "23505"
}
