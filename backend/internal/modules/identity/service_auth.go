// M1 认证服务:登录(手机号/学号/短信)、刷新、登出、短信、找回密码。
// 依据 docs/01 §3 接口、§5 状态机、§6 安全。
package identity

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// LoginByPhone 手机号密码登录(支持一号多校)。
func (s *Service) LoginByPhone(ctx context.Context, req LoginPhoneRequest, device, ip string) (*LoginResult, error) {
	if !validCNPhone(req.Phone) {
		return nil, apperr.ErrPhoneInvalid
	}
	ph := s.phoneHash(req.Phone)

	// 跨租户定位该手机号的账号(特权连接绕 RLS,只取登录定位最小字段)。
	accts, err := s.findAccountsByPhone(ctx, ph)
	if err != nil {
		return nil, err
	}
	if len(accts) == 0 {
		return nil, apperr.ErrWrongCredentials // 不暴露"账号不存在"防枚举。
	}

	// 多租户且未指定 → 返回租户列表让用户选。
	target, selected := resolveTenant(accts, req.TenantID)
	if !selected {
		return &LoginResult{NeedSelectTenant: true, Tenants: briefs(accts)}, nil
	}
	if err := ensureSelectedTenantLoginAllowed(target); err != nil {
		return nil, err
	}

	// 在目标租户上下文内校验密码并签发。
	acc, err := s.loadAccountByPhone(ctx, target.TenantID, ph)
	if err != nil {
		return nil, err
	}
	return s.finishPasswordLogin(ctx, acc, req.Password, device, ip)
}

// LoginByNo 学号/工号登录(备用)。先按短码定位租户,再按 no 查账号。
func (s *Service) LoginByNo(ctx context.Context, req LoginNoRequest, device, ip string) (*LoginResult, error) {
	tenantID, err := s.tenantIDByCode(ctx, req.TenantCode)
	if err != nil {
		return nil, err
	}
	var acc sqlcgen.Account
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		p, e := q.GetAccountProfileByNo(ctx, req.No)
		if e != nil {
			return apperr.ErrWrongCredentials
		}
		a, e := q.GetAccountByID(ctx, p.AccountID)
		if e != nil {
			return apperr.ErrWrongCredentials
		}
		acc = a
		return nil
	}); err != nil {
		return nil, err
	}
	return s.finishPasswordLogin(ctx, acc, req.Password, device, ip)
}

// LoginBySms 短信验证码登录(支持一号多校)。
func (s *Service) LoginBySms(ctx context.Context, req LoginSmsRequest, device, ip string) (*LoginResult, error) {
	if !validCNPhone(req.Phone) {
		return nil, apperr.ErrPhoneInvalid
	}
	ph := s.phoneHash(req.Phone)
	accts, err := s.findAccountsByPhone(ctx, ph)
	if err != nil {
		return nil, err
	}
	if len(accts) == 0 {
		return nil, apperr.ErrWrongCredentials
	}
	target, selected := resolveTenant(accts, req.TenantID)
	if !selected {
		return &LoginResult{NeedSelectTenant: true, Tenants: briefs(accts)}, nil
	}
	if err := ensureSelectedTenantLoginAllowed(target); err != nil {
		return nil, err
	}

	// 校验验证码(在目标租户上下文)。
	acc, err := s.loadAccountByPhone(ctx, target.TenantID, ph)
	if err != nil {
		return nil, err
	}
	if err := s.verifySmsCode(ctx, target.TenantID, ph, SmsSceneLogin, req.Code); err != nil {
		return nil, err
	}
	if err := loginableStatus(acc.Status); err != nil {
		return nil, err
	}
	return s.issueLogin(ctx, acc, device, ip, true)
}

// finishPasswordLogin 完成密码校验 + 状态机检查 + 锁定 + 签发。
func (s *Service) finishPasswordLogin(ctx context.Context, acc sqlcgen.Account, password, device, ip string) (*LoginResult, error) {
	// 锁定检查。
	if acc.LockedUntil.Valid && acc.LockedUntil.Time.After(timex.Now()) {
		return nil, apperr.ErrAccountLocked
	}
	// 状态机:正常账号可登录;初始密码开通的待激活账号允许进入首登改密流程。
	if err := passwordLoginableStatus(acc); err != nil {
		return nil, err
	}
	// SSO 账号无密码,不能走密码登录。
	if !acc.PasswordHash.Valid {
		return nil, apperr.ErrWrongCredentials
	}
	ok, err := crypto.VerifyPassword(password, acc.PasswordHash.String)
	if err != nil {
		return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	if !ok {
		// 失败计数 +1,达阈值锁定。
		if e := s.repo.inTenantID(ctx, acc.TenantID, func(q *sqlcgen.Queries) error {
			_, ie := q.IncrAccountPwdFailed(ctx, sqlcgen.IncrAccountPwdFailedParams{
				ID:             acc.ID,
				PwdFailedCount: s.passwordMaxFailedCount,
				Column3:        pgText(fmt.Sprintf("%d", s.passwordLockMinutes)),
			})
			return ie
		}); e != nil {
			return nil, apperr.ErrAccountMutationFailed.WithCause(e)
		}
		return nil, apperr.ErrWrongCredentials
	}
	if err := passwordLoginPostVerifyError(acc.MustChangePwd); err != nil {
		return nil, err
	}
	return s.issueLogin(ctx, acc, device, ip, true)
}

// passwordLoginPostVerifyError 处理密码已校验通过后的登录门禁;首登改密不是登录失败。
func passwordLoginPostVerifyError(mustChangePwd bool) error {
	// must_change_pwd 必须随登录结果返回,让前端在已鉴权会话中进入改密流程。
	return nil
}

// passwordLoginableStatus 判断密码登录是否可进入会话签发。
func passwordLoginableStatus(acc sqlcgen.Account) error {
	if acc.Status == AccountPending && acc.MustChangePwd && acc.PasswordHash.Valid {
		return nil
	}
	return loginableStatus(acc.Status)
}

// issueLogin 清失败计数、单端踢人、创建会话、签发双 Token。
func (s *Service) issueLogin(ctx context.Context, acc sqlcgen.Account, device, ip string, resetFailed bool) (*LoginResult, error) {
	roles, err := s.loadRoleCodes(ctx, acc.TenantID, acc.ID)
	if err != nil {
		return nil, err
	}

	sessionID := s.idgen.Generate()
	// 不透明 Refresh Token 只返回一次,数据库仅保存其 HMAC 哈希用于校验。
	refreshPlain, err := crypto.RandomToken(48)
	if err != nil {
		return nil, apperr.ErrAuthTokenIssueFailed.WithCause(err)
	}
	refreshHash := crypto.HMACHash(s.hmacKey, refreshPlain)

	access, err := s.auth.IssueAccess(acc.TenantID, acc.ID, sessionID, false)
	if err != nil {
		return nil, apperr.ErrAuthTokenIssueFailed.WithCause(err)
	}

	if err := s.repo.inTenantID(ctx, acc.TenantID, func(q *sqlcgen.Queries) error {
		if resetFailed {
			if e := q.ResetAccountPwdFailed(ctx, acc.ID); e != nil {
				return e
			}
		}
		// 单端登录:吊销该账号其余有效会话。
		if e := q.RevokeAllAccountSessions(ctx, acc.ID); e != nil {
			return e
		}
		_, e := q.CreateAuthSession(ctx, sqlcgen.CreateAuthSessionParams{
			ID:               sessionID,
			TenantID:         acc.TenantID,
			AccountID:        acc.ID,
			RefreshTokenHash: refreshHash,
			DeviceInfo:       pgText(device),
			Ip:               pgText(ip),
			ExpireAt:         timex.RequiredTimestamptz(timex.Now().Add(s.refreshTTL)),
		})
		if e != nil {
			return e
		}
		return s.writeAccountAuditInTx(ctx, q, acc.TenantID, acc.ID, audit.ActorRoleFromAccount(contracts.AccountInfo{
			BaseIdentity: acc.BaseIdentity,
			Roles:        roles,
		}), AuditActionAuthLogin, AuditTargetAuthSession, sessionID, map[string]any{
			"device_recorded": device != "",
			"ip_recorded":     ip != "",
		})
	}); err != nil {
		return nil, apperr.ErrAuthSessionStoreFailed.WithCause(err)
	}

	return &LoginResult{
		AccessToken:   access,
		RefreshToken:  refreshPlain,
		MustChangePwd: acc.MustChangePwd,
		Account: &AccountBrief{
			ID:           ids.Format(acc.ID),
			Name:         acc.Name,
			BaseIdentity: acc.BaseIdentity,
			Roles:        roles,
		},
	}, nil
}

// Refresh 轮转 Refresh Token:校验 → 重放检测 → 签发新对。
func (s *Service) Refresh(ctx context.Context, req RefreshRequest, device, ip string) (*TokenPair, error) {
	refreshHash := crypto.HMACHash(s.hmacKey, req.RefreshToken)
	if pair, handled, err := s.refreshPlatform(ctx, refreshHash, device, ip); handled || err != nil {
		return pair, err
	}

	// 跨租户按 token hash 定位会话(特权连接绕 RLS)。
	sess, found, err := s.findSessionByTokenHash(ctx, refreshHash)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, apperr.ErrRefreshInvalid
	}
	// 已吊销的 token 再次出现 → 重放攻击,吊销该账号全部会话并拒绝。
	if sess.Status == SessionRevoked {
		if err := s.repo.inTenantID(ctx, sess.TenantID, func(q *sqlcgen.Queries) error {
			return q.RevokeAllAccountSessions(ctx, sess.AccountID)
		}); err != nil {
			return nil, apperr.ErrAuthReplayRevokeFailed.WithCause(fmt.Errorf("Refresh Token 重放吊销会话失败: %w", err))
		}
		return nil, apperr.ErrRefreshReused
	}
	if sess.ExpireAt.Time.Before(timex.Now()) {
		return nil, apperr.ErrRefreshInvalid
	}
	if err := s.ensureTenantActiveByID(ctx, sess.TenantID); err != nil {
		return nil, err
	}

	// 取账号,签发新对(issueLogin 内部吊销旧会话含本次)。
	var acc sqlcgen.Account
	if err := s.repo.inTenantID(ctx, sess.TenantID, func(q *sqlcgen.Queries) error {
		a, e := q.GetAccountByID(ctx, sess.AccountID)
		if e != nil {
			return apperr.ErrRefreshInvalid
		}
		acc = a
		return nil
	}); err != nil {
		return nil, err
	}
	if err := loginableStatus(acc.Status); err != nil {
		return nil, err
	}
	res, err := s.issueLogin(ctx, acc, device, ip, false)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: res.AccessToken, RefreshToken: res.RefreshToken}, nil
}

// Logout 吊销当前会话(按 access claims 的 session_id)。
func (s *Service) Logout(ctx context.Context, tenantID, accountID, sessionID int64) error {
	if tenantID == 0 {
		return s.LogoutPlatform(ctx, accountID, sessionID)
	}
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if err := q.RevokeAuthSession(ctx, sessionID); err != nil {
			return err
		}
		roles, err := q.ListAccountRoles(ctx, accountID)
		if err != nil {
			return err
		}
		return s.writeAccountAuditInTx(ctx, q, tenantID, accountID, audit.ActorRoleFromAccount(contracts.AccountInfo{
			Roles: roleCodesOf(roles),
		}), AuditActionAuthLogout, AuditTargetAuthSession, sessionID, nil)
	}); err != nil {
		return apperr.ErrAuthSessionStoreFailed.WithCause(err)
	}
	return nil
}

// refreshPlatform 轮转平台管理员 Refresh Token;未命中返回 handled=false 交给租户会话路径。
func (s *Service) refreshPlatform(ctx context.Context, refreshHash, device, ip string) (*TokenPair, bool, error) {
	sess, found, err := s.findPlatformSessionByTokenHash(ctx, refreshHash)
	if err != nil {
		return nil, true, err
	}
	if !found {
		return nil, false, nil
	}
	if sess.Status == SessionRevoked {
		if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
			return q.RevokeAllPlatformAdminSessions(ctx, sess.PlatformAdminID)
		}); err != nil {
			return nil, true, apperr.ErrAuthReplayRevokeFailed.WithCause(fmt.Errorf("平台 Refresh Token 重放吊销会话失败: %w", err))
		}
		return nil, true, apperr.ErrRefreshReused
	}
	if sess.ExpireAt.Time.Before(timex.Now()) {
		return nil, true, apperr.ErrRefreshInvalid
	}

	var admin sqlcgen.PlatformAdmin
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.GetPlatformAdminByID(ctx, sess.PlatformAdminID)
		if e != nil {
			return apperr.ErrRefreshInvalid
		}
		admin = row
		return nil
	}); err != nil {
		return nil, true, toAppErr(err)
	}
	if admin.Status != TenantActive {
		return nil, true, apperr.ErrRefreshInvalid
	}
	res, err := s.issuePlatformLogin(ctx, admin, device, ip)
	if err != nil {
		return nil, true, err
	}
	return &TokenPair{AccessToken: res.AccessToken, RefreshToken: res.RefreshToken}, true, nil
}

// LogoutPlatform 吊销平台管理员当前会话。
func (s *Service) LogoutPlatform(ctx context.Context, platformAdminID, sessionID int64) error {
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		return q.RevokePlatformAuthSession(ctx, sessionID)
	}); err != nil {
		return apperr.ErrPlatformAuthSessionFailed.WithCause(err)
	}
	return s.writePlatformAudit(ctx, platformAdminID, AuditActionAuthLogout, AuditTargetAuthSession, sessionID, nil)
}

// ResetPassword 找回密码:校验验证码 → 设新密码(清首登标记 + 失败计数)。
func (s *Service) ResetPassword(ctx context.Context, req ResetPasswordRequest) error {
	if !validPassword(req.NewPassword) {
		return apperr.ErrWeakPassword
	}
	if !validCNPhone(req.Phone) {
		return apperr.ErrPhoneInvalid
	}
	ph := s.phoneHash(req.Phone)
	accts, err := s.findAccountsByPhone(ctx, ph)
	if err != nil {
		return err
	}
	if len(accts) == 0 {
		return apperr.ErrAccountNotFound
	}
	target, err := selectResetPasswordTarget(accts, req.TenantID)
	if err != nil {
		return err
	}
	// 找回验证码发送时允许 tenant_id=NULL,校验也必须走同一全局路径;账号写入仍回到目标租户事务。
	if err := s.verifySmsCode(ctx, resetSmsVerificationTenantID(target), ph, SmsSceneReset, req.Code); err != nil {
		return err
	}
	hash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	if err := s.repo.inTenantID(ctx, target.TenantID, func(q *sqlcgen.Queries) error {
		acc, e := q.GetAccountByID(ctx, target.AccountID)
		if e != nil {
			return apperr.ErrAccountNotFound
		}
		if e := q.UpdateAccountPassword(ctx, sqlcgen.UpdateAccountPasswordParams{
			ID: target.AccountID, PasswordHash: pgText(hash), MustChangePwd: false,
		}); e != nil {
			return e
		}
		if e := q.ResetAccountPwdFailed(ctx, target.AccountID); e != nil {
			return e
		}
		// 找回密码后吊销全部会话(强制重新登录),并在同一事务内记录账号安全变更审计。
		if e := q.RevokeAllAccountSessions(ctx, target.AccountID); e != nil {
			return e
		}
		roles, e := q.ListAccountRoles(ctx, target.AccountID)
		if e != nil {
			return e
		}
		return s.writeAccountAuditInTx(ctx, q, target.TenantID, target.AccountID, audit.ActorRoleFromAccount(contracts.AccountInfo{
			BaseIdentity: acc.BaseIdentity,
			Roles:        roleCodesOf(roles),
		}), AuditActionAccountResetPwd, AuditTargetAccount, target.AccountID, map[string]any{
			"self_service":     true,
			"sessions_revoked": true,
		})
	}); err != nil {
		return toAppErrWith(err, apperr.ErrAccountMutationFailed)
	}
	return nil
}

// ---- 内部辅助 ----

// selectResetPasswordTarget 选择找回密码目标账号;一号多校必须由用户选择学校。
func selectResetPasswordTarget(accts []sqlcgen.FindAccountsByPhoneAllTenantsRow, reqTenantID string) (sqlcgen.FindAccountsByPhoneAllTenantsRow, error) {
	if len(accts) == 1 {
		return accts[0], nil
	}
	if reqTenantID == "" {
		// 无登录态的找回流程不能替用户猜学校,否则一号多校手机号会误改其中一个学校账号。
		return sqlcgen.FindAccountsByPhoneAllTenantsRow{}, apperr.ErrResetPasswordTenantAmbiguous
	}
	tid, ok := ids.Parse(reqTenantID)
	if !ok {
		return sqlcgen.FindAccountsByPhoneAllTenantsRow{}, apperr.ErrResetPasswordTenantInvalid
	}
	for _, acct := range accts {
		if acct.TenantID == tid {
			return acct, nil
		}
	}
	return sqlcgen.FindAccountsByPhoneAllTenantsRow{}, apperr.ErrResetPasswordTenantInvalid
}

// resetSmsVerificationTenantID 返回找回短信校验使用的租户范围。
func resetSmsVerificationTenantID(sqlcgen.FindAccountsByPhoneAllTenantsRow) int64 {
	return 0
}

// findAccountsByPhone 跨租户定位手机号账号;无特权连接则报错(部署须配)。
func (s *Service) findAccountsByPhone(ctx context.Context, phoneHash string) ([]sqlcgen.FindAccountsByPhoneAllTenantsRow, error) {
	if !s.repo.hasPrivileged() {
		return nil, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("未配置特权连接,无法执行跨租户登录定位"))
	}
	var rows []sqlcgen.FindAccountsByPhoneAllTenantsRow
	if err := s.repo.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		r, e := q.FindAccountsByPhoneAllTenants(ctx, phoneHash)
		if e != nil {
			return e
		}
		rows = r
		return nil
	}); err != nil {
		return nil, apperr.ErrAuthLookupUnavailable.WithCause(err)
	}
	return rows, nil
}

// findSessionByTokenHash 跨租户定位会话(特权连接)。
func (s *Service) findSessionByTokenHash(ctx context.Context, tokenHash string) (sqlcgen.FindSessionByTokenHashRow, bool, error) {
	if !s.repo.hasPrivileged() {
		return sqlcgen.FindSessionByTokenHashRow{}, false, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("未配置特权连接,无法刷新令牌"))
	}
	var row sqlcgen.FindSessionByTokenHashRow
	found := true
	if err := s.repo.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		r, e := q.FindSessionByTokenHash(ctx, tokenHash)
		if e != nil {
			if db.IsNoRows(e) {
				found = false
				return nil
			}
			return e
		}
		row = r
		return nil
	}); err != nil {
		return sqlcgen.FindSessionByTokenHashRow{}, false, apperr.ErrAuthSessionQueryFailed.WithCause(err)
	}
	return row, found, nil
}

// findPlatformSessionByTokenHash 定位平台管理员 Refresh 会话。
func (s *Service) findPlatformSessionByTokenHash(ctx context.Context, tokenHash string) (sqlcgen.FindPlatformSessionByTokenHashRow, bool, error) {
	var row sqlcgen.FindPlatformSessionByTokenHashRow
	found := true
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		r, e := q.FindPlatformSessionByTokenHash(ctx, tokenHash)
		if e != nil {
			if db.IsNoRows(e) {
				found = false
				return nil
			}
			return e
		}
		row = r
		return nil
	}); err != nil {
		return sqlcgen.FindPlatformSessionByTokenHashRow{}, false, apperr.ErrAuthSessionQueryFailed.WithCause(err)
	}
	return row, found, nil
}

// loadAccountByPhone 在指定租户内按 phone_hash 取账号。
func (s *Service) loadAccountByPhone(ctx context.Context, tenantID int64, phoneHash string) (sqlcgen.Account, error) {
	var acc sqlcgen.Account
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		a, e := q.GetAccountByPhoneHash(ctx, phoneHash)
		if e != nil {
			return apperr.ErrWrongCredentials
		}
		acc = a
		return nil
	}); err != nil {
		return sqlcgen.Account{}, err
	}
	return acc, nil
}

// tenantIDByCode 按短码取租户 ID,校验状态。
func (s *Service) tenantIDByCode(ctx context.Context, code string) (int64, error) {
	var tid int64
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		t, e := q.GetTenantByCode(ctx, code)
		if e != nil {
			return apperr.ErrTenantNotFound
		}
		if err := ensureTenantLoginAllowed(t.Status); err != nil {
			return err
		}
		tid = t.ID
		return nil
	}); err != nil {
		return 0, err
	}
	return tid, nil
}

// ensureTenantActiveByID 校验租户仍允许认证与刷新。
func (s *Service) ensureTenantActiveByID(ctx context.Context, tenantID int64) error {
	return s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		t, err := q.GetTenantByID(ctx, tenantID)
		if err != nil {
			return apperr.ErrTenantNotFound
		}
		return ensureTenantLoginAllowed(t.Status)
	})
}

// ensureSelectedTenantLoginAllowed 校验手机号定位出的租户仍允许登录。
func ensureSelectedTenantLoginAllowed(row sqlcgen.FindAccountsByPhoneAllTenantsRow) error {
	return ensureTenantLoginAllowed(row.TenantStatus)
}

// ensureTenantLoginAllowed 判断租户状态是否允许认证入口继续。
func ensureTenantLoginAllowed(status int16) error {
	if status != TenantActive {
		return apperr.ErrTenantDisabled
	}
	return nil
}

// loadRoleCodes 取账号角色编码列表。
func (s *Service) loadRoleCodes(ctx context.Context, tenantID, accountID int64) ([]string, error) {
	var nums []int16
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		rs, e := q.ListAccountRoles(ctx, accountID)
		if e != nil {
			return e
		}
		nums = rs
		return nil
	}); err != nil {
		return nil, apperr.ErrAccountQueryFailed.WithCause(err)
	}
	codes := make([]string, 0, len(nums))
	for _, r := range nums {
		codes = append(codes, contracts.RoleCode(r))
	}
	return codes, nil
}

// resolveTenant 根据请求选定的 tenant_id 从候选账号中定位目标。
func resolveTenant(accts []sqlcgen.FindAccountsByPhoneAllTenantsRow, reqTenantID string) (sqlcgen.FindAccountsByPhoneAllTenantsRow, bool) {
	if len(accts) == 1 {
		return accts[0], true
	}
	if reqTenantID == "" {
		return sqlcgen.FindAccountsByPhoneAllTenantsRow{}, false
	}
	tid, ok := ids.Parse(reqTenantID)
	if !ok {
		return sqlcgen.FindAccountsByPhoneAllTenantsRow{}, false
	}
	for _, a := range accts {
		if a.TenantID == tid {
			return a, true
		}
	}
	return sqlcgen.FindAccountsByPhoneAllTenantsRow{}, false
}

// briefs 把候选账号转为租户选择列表。
func briefs(accts []sqlcgen.FindAccountsByPhoneAllTenantsRow) []TenantBrief {
	out := make([]TenantBrief, 0, len(accts))
	for _, a := range accts {
		out = append(out, TenantBrief{
			TenantID:   ids.Format(a.TenantID),
			TenantCode: a.TenantCode,
			TenantName: a.TenantName,
		})
	}
	return out
}
