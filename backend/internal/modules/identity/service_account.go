// M1 账号管理服务:创建/更新/状态机迁移/授撤管理员/重置密码/个人中心。
// 依据 docs/01 §3 接口、§4 权限、§5 状态机。
package identity

import (
	"context"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/db"
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

	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		// 校验组织存在(学生→班级,教师→院系)。
		if req.BaseIdentity == BaseIdentityStudent {
			if _, e := q.GetClassByID(ctx, orgID); e != nil {
				return apperr.ErrClassNotFound
			}
		} else {
			if _, e := q.GetDepartmentByID(ctx, orgID); e != nil {
				return apperr.ErrDepartmentNotFound
			}
		}
		// 建账号(待激活)。
		if _, e := q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
			ID:            accountID,
			TenantID:      tenantID,
			PhoneEnc:      phoneEnc,
			PhoneHash:     ph,
			PasswordHash:  credential.PasswordHash,
			Name:          req.Name,
			BaseIdentity:  req.BaseIdentity,
			Status:        AccountPending,
			MustChangePwd: credential.MustChangePassword,
		}); e != nil {
			if isUniqueViolation(e) {
				return apperr.ErrPhoneAlreadyExists
			}
			return e
		}
		// 建扩展信息(学号/工号 + 组织归属)。
		if _, e := q.CreateAccountProfile(ctx, sqlcgen.CreateAccountProfileParams{
			AccountID:      accountID,
			TenantID:       tenantID,
			No:             req.No,
			OrgID:          orgID,
			EnrollmentYear: pgInt2(req.EnrollmentYear, req.BaseIdentity == BaseIdentityStudent),
			Title:          pgText(req.Title),
		}); e != nil {
			if isUniqueViolation(e) {
				return apperr.ErrAccountNoAlreadyExists
			}
			return e
		}
		// 基础角色随 base_identity 维护一致(account_role 为鉴权权威源)。
		baseRole := RoleStudent
		if req.BaseIdentity == BaseIdentityTeacher {
			baseRole = RoleTeacher
		}
		if e := q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{
			ID: s.idgen.Generate(), TenantID: tenantID, AccountID: accountID, Role: baseRole,
		}); e != nil {
			return e
		}
		if credential.NeedsActivationCode {
			code, e := s.CreateActivationCode(ctx, q, tenantID, accountID, actorID)
			if e != nil {
				return e
			}
			activationCode = code
		}
		// 审计只记录账号开通方式与组织归属,不记录手机号明文、临时密码或激活码。
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionAccountCreate, AuditTargetAccount, accountID, map[string]any{
			"base_identity":          req.BaseIdentity,
			"org_id":                 ids.Format(orgID),
			"enable_activation_code": enableActivationCode,
		})
	}); err != nil {
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
	return s.mustTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetAccountByID(ctx, accountID); e != nil {
			return apperr.ErrAccountNotFound
		}
		if _, e := q.UpdateAccountName(ctx, sqlcgen.UpdateAccountNameParams{ID: accountID, Name: req.Name}); e != nil {
			return e
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionAccountUpdate, AuditTargetAccount, accountID, map[string]any{
			"fields": []string{"name"},
		})
	})
}

// SetAccountStatus 状态机迁移(停用/启用/归档/恢复/注销)。校验合法迁移。
func (s *Service) SetAccountStatus(ctx context.Context, accountID int64, target int16) error {
	return s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		acc, e := q.GetAccountByID(ctx, accountID)
		if e != nil {
			return apperr.ErrAccountNotFound
		}
		if !canTransit(acc.Status, target) {
			return apperr.ErrAccountStatusTransitionInvalid
		}
		if _, e = q.UpdateAccountStatus(ctx, sqlcgen.UpdateAccountStatusParams{ID: accountID, Status: target}); e != nil {
			return e
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionAccountStatus, AuditTargetAccount, accountID, map[string]any{
			"from_status": acc.Status,
			"to_status":   target,
		})
	})
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
	var archived []int64
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		rows, err := q.ArchiveStudentAccountsByEnrollmentYear(ctx, pgInt2(req.EnrollmentYear, true))
		if err != nil {
			return err
		}
		archived = rows
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionAccountStatus, AuditTargetAccount, 0, map[string]any{
			"enrollment_year": req.EnrollmentYear,
			"base_identity":   BaseIdentityStudent,
			"from_status":     AccountActive,
			"to_status":       AccountArchived,
			"archived_count":  len(rows),
		})
	}); err != nil {
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
	return s.mustTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetAccountByID(ctx, accountID); e != nil {
			return apperr.ErrAccountNotFound
		}
		if err := q.RevokeAllAccountSessions(ctx, accountID); err != nil {
			return err
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionAccountForceLogout, AuditTargetAccount, accountID, nil)
	})
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
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetAccountByID(ctx, accountID); e != nil {
			return apperr.ErrAccountNotFound
		}
		if e := q.UpdateAccountPassword(ctx, sqlcgen.UpdateAccountPasswordParams{
			ID: accountID, PasswordHash: pgText(hash), MustChangePwd: true,
		}); e != nil {
			return e
		}
		if e := q.ResetAccountPwdFailed(ctx, accountID); e != nil {
			return e
		}
		if e := q.RevokeAllAccountSessions(ctx, accountID); e != nil {
			return e
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionAccountResetPwd, AuditTargetAccount, accountID, map[string]any{
			"must_change_pwd":  true,
			"sessions_revoked": true,
		})
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return "", ae
		}
		return "", apperr.ErrAccountMutationFailed.WithCause(err)
	}
	return temp, nil
}

// GrantAdmin 授予学校管理员(仅教师账号可被授予)。
func (s *Service) GrantAdmin(ctx context.Context, accountID int64) error {
	return s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		acc, e := q.GetAccountByID(ctx, accountID)
		if e != nil {
			return apperr.ErrAccountNotFound
		}
		if acc.BaseIdentity != BaseIdentityTeacher {
			return apperr.ErrGrantAdminNonTeacher // 学生不可授予管理员。
		}
		if e := q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{
			ID: s.idgen.Generate(), TenantID: acc.TenantID, AccountID: accountID, Role: RoleSchoolAdmin,
		}); e != nil {
			return e
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionAccountGrantAdmin, AuditTargetAccount, accountID, nil)
	})
}

// RevokeAdmin 撤销学校管理员角色(保留教师身份)。
func (s *Service) RevokeAdmin(ctx context.Context, accountID int64) error {
	return s.mustTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetAccountByID(ctx, accountID); e != nil {
			return apperr.ErrAccountNotFound
		}
		if err := q.RemoveAccountRole(ctx, sqlcgen.RemoveAccountRoleParams{AccountID: accountID, Role: RoleSchoolAdmin}); err != nil {
			return err
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionAccountRevokeAdmin, AuditTargetAccount, accountID, nil)
	})
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
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		acc, e := q.GetAccountByID(ctx, accountID)
		if e != nil {
			return apperr.ErrAccountNotFound
		}
		// 非首登改密场景须校验旧密码。
		if !acc.MustChangePwd {
			if !acc.PasswordHash.Valid {
				return apperr.ErrOldPasswordWrong
			}
			ok, ve := crypto.VerifyPassword(req.OldPassword, acc.PasswordHash.String)
			if ve != nil {
				return apperr.ErrAccountCredentialFailed.WithCause(ve)
			}
			if !ok {
				return apperr.ErrOldPasswordWrong
			}
		}
		if e := q.UpdateAccountPassword(ctx, sqlcgen.UpdateAccountPasswordParams{
			ID: accountID, PasswordHash: pgText(hash), MustChangePwd: false,
		}); e != nil {
			return e
		}
		// 首登改密的账号:激活转正常。
		if acc.Status == AccountPending {
			if e := q.SetAccountActivated(ctx, accountID); e != nil {
				return e
			}
		}
		roles, e := q.ListAccountRoles(ctx, accountID)
		if e != nil {
			return e
		}
		return s.writeAuditInTx(ctx, q, audit.ActorRoleFromAccount(contracts.AccountInfo{
			BaseIdentity: acc.BaseIdentity,
			Roles:        roleCodesOf(roles),
		}), AuditActionAccountUpdate, AuditTargetAccount, accountID, map[string]any{
			"fields":          []string{"password"},
			"activated_after": acc.Status == AccountPending,
		})
	}); err != nil {
		return toAppErrWith(err, apperr.ErrAccountMutationFailed)
	}
	return nil
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
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		acc, e := q.GetAccountByID(ctx, accountID)
		if e != nil {
			return apperr.ErrAccountNotFound
		}
		if e := q.UpdateAccountPhone(ctx, sqlcgen.UpdateAccountPhoneParams{ID: accountID, PhoneEnc: enc, PhoneHash: newHash}); isUniqueViolation(e) {
			return apperr.ErrPhoneAlreadyExists
		} else if e != nil {
			return e
		}
		roles, e := q.ListAccountRoles(ctx, accountID)
		if e != nil {
			return e
		}
		return s.writeAuditInTx(ctx, q, audit.ActorRoleFromAccount(contracts.AccountInfo{
			BaseIdentity: acc.BaseIdentity,
			Roles:        roleCodesOf(roles),
		}), AuditActionAccountUpdate, AuditTargetAccount, accountID, map[string]any{
			"fields": []string{"phone"},
		})
	}); err != nil {
		return toAppErrWith(err, apperr.ErrAccountMutationFailed)
	}
	return nil
}

// GetMe 个人中心信息(含学籍只读字段)。
func (s *Service) GetMe(ctx context.Context, accountID int64) (*MeView, error) {
	var view MeView
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		acc, e := q.GetAccountByID(ctx, accountID)
		if e != nil {
			return apperr.ErrAccountNotFound
		}
		prof, e := q.GetAccountProfile(ctx, accountID)
		hasProfile := true
		if e != nil && db.IsNoRows(e) {
			hasProfile = false
		} else if e != nil {
			return e
		}
		roles, e := q.ListAccountRoles(ctx, accountID)
		if e != nil {
			return e
		}
		phone, e := s.decryptPhone(acc.PhoneEnc)
		if e != nil {
			return apperr.ErrAccountCredentialFailed.WithCause(e)
		}
		view = MeView{
			ID: ids.Format(acc.ID), Name: acc.Name, Phone: maskPhone(phone),
			BaseIdentity: acc.BaseIdentity, Roles: roleCodesOf(roles),
		}
		applyProfileToMeView(&view, prof, hasProfile)
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrAccountQueryFailed.WithCause(err)
	}
	return &view, nil
}

// ListMySessions 查询当前账号有效会话。
func (s *Service) ListMySessions(ctx context.Context, accountID int64) ([]SessionView, error) {
	var views []SessionView
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		rows, e := q.ListActiveSessions(ctx, accountID)
		if e != nil {
			return e
		}
		for _, row := range rows {
			views = append(views, SessionView{
				ID:         ids.Format(row.ID),
				DeviceInfo: textVal(row.DeviceInfo),
				IP:         textVal(row.Ip),
				ExpireAt:   timex.FromTimestamptz(row.ExpireAt).Format(time.RFC3339),
				CreatedAt:  timex.FromTimestamptz(row.CreatedAt).Format(time.RFC3339),
			})
		}
		return nil
	}); err != nil {
		return nil, apperr.ErrAuthSessionQueryFailed.WithCause(err)
	}
	return views, nil
}

// applyProfileToMeView 把可选组织档案合入个人中心视图;首个学校管理员允许暂缺档案。
func applyProfileToMeView(view *MeView, prof sqlcgen.AccountProfile, exists bool) {
	if !exists {
		return
	}
	view.No = prof.No
	view.OrgID = ids.Format(prof.OrgID)
	view.Title = textVal(prof.Title)
}

// ---- 内部辅助 ----

// mustTenant 在租户事务内执行,错误统一转应用错误。
func (s *Service) mustTenant(ctx context.Context, fn func(q *sqlcgen.Queries) error) error {
	return s.mustTenantWith(ctx, fn, apperr.ErrAccountMutationFailed)
}

// mustTenantWith 在租户事务内执行,未知底层错误按调用场景转为专属错误码。
func (s *Service) mustTenantWith(ctx context.Context, fn func(q *sqlcgen.Queries) error, code *apperr.Error) error {
	if err := s.repo.inTenant(ctx, fn); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return code.WithCause(err)
	}
	return nil
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
