// M1 认证服务:登录(手机号/学号/短信)、刷新、登出、短信、找回密码。
// 依据 docs/01 §3 接口、§5 状态机、§6 安全。
package identity

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
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
	accts, err := s.repo.findLoginCandidatesByPhone(ctx, ph)
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
	acc, err := s.repo.loadAccountByPhone(ctx, target.TenantID, ph)
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
	acc, err := s.repo.loadAccountByNo(ctx, tenantID, req.No)
	if err != nil {
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
	accts, err := s.repo.findLoginCandidatesByPhone(ctx, ph)
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
	acc, err := s.repo.loadAccountByPhone(ctx, target.TenantID, ph)
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
func (s *Service) finishPasswordLogin(ctx context.Context, acc LoginAccountSnapshot, password, device, ip string) (*LoginResult, error) {
	// 锁定检查。
	if acc.HasLockedUntil && acc.LockedUntil.After(timex.Now()) {
		return nil, apperr.ErrAccountLocked
	}
	// 状态机:正常账号可登录;初始密码开通的待激活账号允许进入首登改密流程。
	if err := passwordLoginableStatus(acc); err != nil {
		return nil, err
	}
	// SSO 账号无密码,不能走密码登录。
	if !acc.HasPassword {
		return nil, apperr.ErrWrongCredentials
	}
	ok, err := crypto.VerifyPassword(password, acc.PasswordHash)
	if err != nil {
		return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	if !ok {
		// 失败计数 +1,达阈值锁定。
		if e := s.repo.incrementPasswordFailure(ctx, acc.TenantID, acc.ID, s.passwordMaxFailedCount, s.passwordLockMinutes); e != nil {
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
func passwordLoginableStatus(acc LoginAccountSnapshot) error {
	if acc.Status == AccountPending && acc.MustChangePwd && acc.HasPassword {
		return nil
	}
	return loginableStatus(acc.Status)
}

// issueLogin 清失败计数、单端踢人、创建会话、签发双 Token。
func (s *Service) issueLogin(ctx context.Context, acc LoginAccountSnapshot, device, ip string, resetFailed bool) (*LoginResult, error) {
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

	entry, err := buildAccountAuditEntry(ctx, acc.TenantID, acc.ID, audit.ActorRoleFromAccount(contracts.AccountInfo{
		BaseIdentity: acc.BaseIdentity,
		Roles:        acc.Roles,
	}), AuditActionAuthLogin, AuditTargetAuthSession, sessionID, map[string]any{
		"device_recorded": device != "",
		"ip_recorded":     ip != "",
	})
	if err != nil {
		return nil, err
	}
	if err := s.repo.createLoginSession(ctx, acc, sessionID, refreshHash, device, ip, timex.Now().Add(s.refreshTTL), resetFailed, buildAuditLogCreate(s.idgen.Generate(), entry)); err != nil {
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
			Roles:        acc.Roles,
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
	sess, found, err := s.repo.findSessionByTokenHash(ctx, refreshHash)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, apperr.ErrRefreshInvalid
	}
	// 已吊销的 token 再次出现 → 重放攻击,吊销该账号全部会话并拒绝。
	if sess.Status == SessionRevoked {
		if err := s.repo.revokeAllAccountSessions(ctx, sess.TenantID, sess.AccountID); err != nil {
			return nil, apperr.ErrAuthReplayRevokeFailed.WithCause(fmt.Errorf("Refresh Token 重放吊销会话失败: %w", err))
		}
		return nil, apperr.ErrRefreshReused
	}
	if sess.ExpireAt.Before(timex.Now()) {
		return nil, apperr.ErrRefreshInvalid
	}
	if err := s.ensureTenantActiveByID(ctx, sess.TenantID); err != nil {
		return nil, err
	}

	// 取账号,签发新对(issueLogin 内部吊销旧会话含本次)。
	acc, err := s.repo.loadAccountByIDForAuth(ctx, sess.TenantID, sess.AccountID)
	if err != nil {
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
	acc, err := s.repo.loadAccountByIDForAuth(ctx, tenantID, accountID)
	if err != nil {
		return apperr.ErrAuthSessionStoreFailed.WithCause(err)
	}
	entry, err := buildAccountAuditEntry(ctx, tenantID, accountID, audit.ActorRoleFromAccount(contracts.AccountInfo{
		BaseIdentity: acc.BaseIdentity,
		Roles:        acc.Roles,
	}), AuditActionAuthLogout, AuditTargetAuthSession, sessionID, nil)
	if err != nil {
		return err
	}
	if err := s.repo.revokeAuthSessionWithAudit(ctx, tenantID, sessionID, buildAuditLogCreate(s.idgen.Generate(), entry)); err != nil {
		return apperr.ErrAuthSessionStoreFailed.WithCause(err)
	}
	return nil
}

// refreshPlatform 轮转平台管理员 Refresh Token;未命中返回 handled=false 交给租户会话路径。
func (s *Service) refreshPlatform(ctx context.Context, refreshHash, device, ip string) (*TokenPair, bool, error) {
	sess, found, err := s.repo.findPlatformSessionByTokenHash(ctx, refreshHash)
	if err != nil {
		return nil, true, err
	}
	if !found {
		return nil, false, nil
	}
	if sess.Status == SessionRevoked {
		if err := s.repo.revokeAllPlatformAdminSessions(ctx, sess.PlatformAdminID); err != nil {
			return nil, true, apperr.ErrAuthReplayRevokeFailed.WithCause(fmt.Errorf("平台 Refresh Token 重放吊销会话失败: %w", err))
		}
		return nil, true, apperr.ErrRefreshReused
	}
	if sess.ExpireAt.Before(timex.Now()) {
		return nil, true, apperr.ErrRefreshInvalid
	}

	admin, err := s.repo.getPlatformAdminByID(ctx, sess.PlatformAdminID)
	if err != nil {
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
	if err := s.repo.revokePlatformSession(ctx, sessionID); err != nil {
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
	accts, err := s.repo.findLoginCandidatesByPhone(ctx, ph)
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
	acc, err := s.repo.loadAccountByIDForAuth(ctx, target.TenantID, target.AccountID)
	if err != nil {
		return toAppErrWith(err, apperr.ErrAccountMutationFailed)
	}
	entry, err := buildAccountAuditEntry(ctx, target.TenantID, target.AccountID, audit.ActorRoleFromAccount(contracts.AccountInfo{
		BaseIdentity: acc.BaseIdentity,
		Roles:        acc.Roles,
	}), AuditActionAccountResetPwd, AuditTargetAccount, target.AccountID, map[string]any{
		"self_service":     true,
		"sessions_revoked": true,
	})
	if err != nil {
		return err
	}
	if err := s.repo.resetPasswordWithAudit(ctx, target, hash, buildAuditLogCreate(s.idgen.Generate(), entry)); err != nil {
		return toAppErrWith(err, apperr.ErrAccountMutationFailed)
	}
	return nil
}

// ---- 内部辅助 ----

// selectResetPasswordTarget 选择找回密码目标账号;一号多校必须由用户选择学校。
func selectResetPasswordTarget(accts []LoginTenantCandidate, reqTenantID string) (LoginTenantCandidate, error) {
	if len(accts) == 1 {
		return accts[0], nil
	}
	if reqTenantID == "" {
		// 无登录态的找回流程不能替用户猜学校,否则一号多校手机号会误改其中一个学校账号。
		return LoginTenantCandidate{}, apperr.ErrResetPasswordTenantAmbiguous
	}
	tid, ok := ids.Parse(reqTenantID)
	if !ok {
		return LoginTenantCandidate{}, apperr.ErrResetPasswordTenantInvalid
	}
	for _, acct := range accts {
		if acct.TenantID == tid {
			return acct, nil
		}
	}
	return LoginTenantCandidate{}, apperr.ErrResetPasswordTenantInvalid
}

// resetSmsVerificationTenantID 返回找回短信校验使用的租户范围。
func resetSmsVerificationTenantID(LoginTenantCandidate) int64 {
	return 0
}

// tenantIDByCode 按短码取租户 ID,校验状态。
func (s *Service) tenantIDByCode(ctx context.Context, code string) (int64, error) {
	t, err := s.repo.getTenantByCode(ctx, code)
	if err != nil {
		return 0, err
	}
	if err := ensureTenantLoginAllowed(t.Status); err != nil {
		return 0, err
	}
	return t.ID, nil
}

// ensureTenantActiveByID 校验租户仍允许认证与刷新。
func (s *Service) ensureTenantActiveByID(ctx context.Context, tenantID int64) error {
	t, err := s.repo.getTenantByIDForLogin(ctx, tenantID)
	if err != nil {
		return err
	}
	return ensureTenantLoginAllowed(t.Status)
}

// ensureSelectedTenantLoginAllowed 校验手机号定位出的租户仍允许登录。
func ensureSelectedTenantLoginAllowed(row LoginTenantCandidate) error {
	return ensureTenantLoginAllowed(row.TenantStatus)
}

// ensureTenantLoginAllowed 判断租户状态是否允许认证入口继续。
func ensureTenantLoginAllowed(status int16) error {
	if status != TenantActive {
		return apperr.ErrTenantDisabled
	}
	return nil
}

// resolveTenant 根据请求选定的 tenant_id 从候选账号中定位目标。
func resolveTenant(accts []LoginTenantCandidate, reqTenantID string) (LoginTenantCandidate, bool) {
	if len(accts) == 1 {
		return accts[0], true
	}
	if reqTenantID == "" {
		return LoginTenantCandidate{}, false
	}
	tid, ok := ids.Parse(reqTenantID)
	if !ok {
		return LoginTenantCandidate{}, false
	}
	for _, a := range accts {
		if a.TenantID == tid {
			return a, true
		}
	}
	return LoginTenantCandidate{}, false
}

// briefs 把候选账号转为租户选择列表。
func briefs(accts []LoginTenantCandidate) []TenantBrief {
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
