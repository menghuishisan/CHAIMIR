// identity service_auth 文件实现登录、刷新、登出、找回密码和激活码状态机。
package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/logging"

	"github.com/jackc/pgx/v5"
)

// LoginPlatform 校验平台管理员凭证并签发平台级 Token。
func (s *Service) LoginPlatform(ctx context.Context, req LoginPlatformRequest, device, ip string) (LoginResponse, error) {
	if !s.deploy.PlatformEnabled {
		return LoginResponse{}, apperr.ErrIdentityPlatformLoginDisabled
	}
	var admin PlatformAdmin
	// 平台管理员不属于任何租户,必须走平台事务读取独立账号体系。
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetPlatformAdminByUsername(ctx, strings.TrimSpace(req.Username))
		if err != nil {
			return err
		}
		admin = row
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	if admin.Status != TenantStatusActive {
		return LoginResponse{}, apperr.ErrIdentityAccountDisabled
	}
	ok, err := crypto.VerifyPassword(req.Password, admin.PasswordHash)
	if err != nil || !ok {
		return LoginResponse{}, apperr.ErrIdentityInvalidCredentials
	}
	var refresh string
	var session PlatformAuthSession
	// 平台登录也执行单端语义:先吊销旧会话,再保存新 Refresh 哈希。
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		if err := tx.RevokePlatformSessions(ctx, admin.ID); err != nil {
			return err
		}
		token, err := crypto.RandomToken(48)
		if err != nil {
			return err
		}
		hash, err := s.hashSecret(token)
		if err != nil {
			return err
		}
		row, err := tx.CreatePlatformAuthSession(ctx, CreatePlatformSessionInput{
			ID:               s.ids.Generate(),
			PlatformAdminID:  admin.ID,
			RefreshTokenHash: hash,
			DeviceInfo:       device,
			IP:               ip,
			ExpireAt:         s.refreshExpireAt(),
		})
		if err != nil {
			return err
		}
		refresh = token
		session = row
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	access, err := s.auth.IssueAccess(0, admin.ID, session.ID, true)
	if err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	// 登录成功必须落审计,平台级审计 tenant_id 固定为 0。
	if err := s.auditLogin(ctx, 0, admin.ID, audit.ActorRolePlatformAdmin); err != nil {
		return LoginResponse{}, err
	}
	return LoginResponse{AccessToken: access, RefreshToken: refresh, Account: &AccountDTO{ID: admin.ID, Name: admin.Name, Roles: []int16{RolePlatformAdmin}, Status: admin.Status}}, nil
}

// LoginPhone 用手机号密码登录,一号多校时返回租户选择列表。
func (s *Service) LoginPhone(ctx context.Context, req LoginPhoneRequest, device, ip string) (LoginResponse, error) {
	if err := ValidatePhone(req.Phone); err != nil {
		return LoginResponse{}, err
	}
	hash, err := s.phoneHash(strings.TrimSpace(req.Phone))
	if err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	var candidates []LoginCandidate
	// 手机号登录先用特权路径定位候选租户,此时还没有租户上下文可注入 RLS。
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		rows, err := tx.ListAccountsByPhoneHash(ctx, hash)
		if err != nil {
			return err
		}
		candidates = rows
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	if len(candidates) == 0 {
		return LoginResponse{}, apperr.ErrIdentityInvalidCredentials
	}
	if req.TenantID == 0 && len(candidates) > 1 {
		// 一号多校不能默认选租户,必须让前端展示候选学校后再带 tenant_id 登录。
		opts := make([]TenantOptionDTO, 0, len(candidates))
		for _, c := range candidates {
			opts = append(opts, TenantOptionDTO{TenantID: c.TenantID, Name: c.TenantName, Code: c.TenantCode})
		}
		return LoginResponse{NeedSelectTenant: true, Tenants: opts}, nil
	}
	candidate := candidates[0]
	if req.TenantID > 0 {
		found := false
		for _, c := range candidates {
			if c.TenantID == req.TenantID {
				candidate = c
				found = true
				break
			}
		}
		if !found {
			return LoginResponse{}, apperr.ErrIdentityInvalidCredentials
		}
	}
	return s.finishPasswordLogin(ctx, candidate.TenantID, candidate.AccountID, req.Password, device, ip)
}

// LoginNo 用学校短码和学号工号完成备用登录。
func (s *Service) LoginNo(ctx context.Context, req LoginNoRequest, device, ip string) (LoginResponse, error) {
	if err := ValidateTenantCode(req.TenantCode); err != nil {
		return LoginResponse{}, err
	}
	var tenantID int64
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		t, err := tx.GetTenantByCode(ctx, strings.TrimSpace(req.TenantCode))
		if err != nil {
			return err
		}
		tenantID = t.ID
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	var account Account
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetAccountByNo(ctx, strings.TrimSpace(req.No))
		if err != nil {
			return err
		}
		account = row
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	return s.finishPasswordLogin(ctx, tenantID, account.ID, req.Password, device, ip)
}

// LoginSMS 使用短信验证码登录指定租户账号,验证码只作为认证因子不改变账号状态校验。
func (s *Service) LoginSMS(ctx context.Context, req LoginSMSRequest, device, ip string) (LoginResponse, error) {
	if err := ValidatePhone(req.Phone); err != nil {
		return LoginResponse{}, err
	}
	if req.TenantID <= 0 {
		return LoginResponse{}, apperr.ErrIdentitySMSNeedsTenant
	}
	if err := s.verifySMSCode(ctx, req.TenantID, req.Phone, SMSSceneLogin, req.Code); err != nil {
		return LoginResponse{}, err
	}
	// 短信只证明手机号持有,仍要读取租户和账号状态,不能绕过停用/到期/锁定判断。
	hash, err := s.phoneHash(strings.TrimSpace(req.Phone))
	if err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	var tenantSnapshot Tenant
	var account Account
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		t, err := tx.GetTenantByID(ctx, req.TenantID)
		if err != nil {
			return err
		}
		a, err := tx.GetAccountByPhoneHash(ctx, req.TenantID, hash)
		if err != nil {
			return err
		}
		tenantSnapshot = t
		account = a
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	now := timex.Now()
	if err := EnsureTenantCanLogin(tenantSnapshot, now); err != nil {
		return LoginResponse{}, err
	}
	if err := EnsureAccountCanLogin(account, now); err != nil {
		return LoginResponse{}, err
	}
	return s.issueTenantLogin(ctx, account, device, ip)
}

// RefreshToken 校验并轮转 Refresh Token,旧 token 立即失效。
func (s *Service) RefreshToken(ctx context.Context, req RefreshRequest, device, ip string) (LoginResponse, error) {
	hash, err := s.hashSecret(strings.TrimSpace(req.RefreshToken))
	if err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	var tenantSession AuthSession
	err = s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetAuthSessionByRefreshHash(ctx, hash)
		if err != nil {
			return err
		}
		tenantSession = row
		return nil
	})
	if err == nil {
		return s.refreshTenantSession(ctx, tenantSession, device, ip)
	}
	var platformSession PlatformAuthSession
	err = s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetPlatformAuthSessionByRefreshHash(ctx, hash)
		if err != nil {
			return err
		}
		platformSession = row
		return nil
	})
	if err != nil {
		return LoginResponse{}, apperr.ErrIdentitySessionInvalid
	}
	return s.refreshPlatformSession(ctx, platformSession, device, ip)
}

// Logout 吊销当前 access token 对应的服务端会话。
func (s *Service) Logout(ctx context.Context, sessionID int64) error {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return apperr.ErrIdentitySessionInvalid
	}
	if id.IsPlatform {
		if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
			return tx.RevokePlatformAuthSession(ctx, sessionID)
		}); err != nil {
			return apperr.ErrInternal.WithCause(err)
		}
		return s.auditLogout(ctx, 0, id.AccountID, audit.ActorRolePlatformAdmin)
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		return tx.RevokeAuthSession(ctx, id.TenantID, sessionID)
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	actorID, actorRole, err := audit.ResolveActor(ctx, s)
	if err != nil {
		return err
	}
	return s.auditLogout(ctx, id.TenantID, actorID, actorRole)
}

// Activate 使用一次性激活码设置密码并激活账号。
func (s *Service) Activate(ctx context.Context, req ActivateRequest) (LoginResponse, error) {
	if err := ValidatePassword(req.Password); err != nil {
		return LoginResponse{}, err
	}
	hash, err := s.hashSecret(strings.TrimSpace(req.ActivationCode))
	if err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	var code ActivationCode
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetActivationCodeByHash(ctx, hash)
		if err != nil {
			return err
		}
		code = row
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrIdentityActivationInvalid
	}
	if code.Status != ActivationStatusActive || timex.Now().After(code.ExpireAt) {
		return LoginResponse{}, apperr.ErrIdentityActivationInvalid
	}
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	if err := s.store.TenantTx(ctx, code.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpdateAccountPassword(ctx, code.AccountID, code.TenantID, passwordHash, false, AccountStatusActive); err != nil {
			return err
		}
		return tx.UseActivationCode(ctx, code.TenantID, code.ID)
	}); err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	return LoginResponse{}, nil
}

// ResetPassword 通过短信验证码重置密码。
func (s *Service) ResetPassword(ctx context.Context, req PasswordResetRequest) error {
	if err := ValidatePhone(req.Phone); err != nil {
		return err
	}
	if err := ValidatePassword(req.NewPassword); err != nil {
		return err
	}
	if err := s.verifySMSCode(ctx, req.TenantID, req.Phone, SMSSceneReset, req.Code); err != nil {
		return err
	}
	hash, err := s.phoneHash(req.Phone)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	passwordHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		account, err := tx.GetAccountByPhoneHash(ctx, req.TenantID, hash)
		if err != nil {
			return err
		}
		_, err = tx.UpdateAccountPassword(ctx, account.ID, req.TenantID, passwordHash, false, AccountStatusActive)
		return err
	})
}

// finishPasswordLogin 校验密码和状态后创建单端会话并签发 Token。
func (s *Service) finishPasswordLogin(ctx context.Context, tenantID, accountID int64, password, device, ip string) (LoginResponse, error) {
	var tenantSnapshot Tenant
	var account Account
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		t, err := tx.GetTenantByID(ctx, tenantID)
		if err != nil {
			return err
		}
		a, err := tx.GetAccount(ctx, accountID)
		if err != nil {
			return err
		}
		tenantSnapshot = t
		account = a
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	now := timex.Now()
	if err := EnsureTenantCanLogin(tenantSnapshot, now); err != nil {
		return LoginResponse{}, err
	}
	if err := EnsureAccountCanLogin(account, now); err != nil {
		return LoginResponse{}, err
	}
	ok, err := crypto.VerifyPassword(password, account.PasswordHash)
	if err != nil || !ok {
		if recordErr := s.recordPasswordFailure(ctx, account); recordErr != nil {
			logging.ErrorContext(ctx, "记录密码失败次数失败", recordErr.Error(), slog.Int64("tenant_id", account.TenantID), slog.Int64("account_id", account.ID))
		}
		return LoginResponse{}, apperr.ErrIdentityInvalidCredentials
	}
	if clearErr := s.clearPasswordFailure(ctx, account); clearErr != nil {
		logging.ErrorContext(ctx, "清理密码失败次数失败", clearErr.Error(), slog.Int64("tenant_id", account.TenantID), slog.Int64("account_id", account.ID))
	}
	return s.issueTenantLogin(ctx, account, device, ip)
}

// issueTenantLogin 吊销旧会话后签发新的租户 Token 对。
func (s *Service) issueTenantLogin(ctx context.Context, account Account, device, ip string) (LoginResponse, error) {
	var refresh string
	var session AuthSession
	if err := s.store.TenantTx(ctx, account.TenantID, func(ctx context.Context, tx TxStore) error {
		// 单端登录要求同账号旧 Refresh 立即失效,因此吊销和新会话创建必须在同一事务。
		if err := tx.RevokeAccountSessions(ctx, account.TenantID, account.ID); err != nil {
			return err
		}
		token, err := crypto.RandomToken(48)
		if err != nil {
			return err
		}
		hash, err := s.hashSecret(token)
		if err != nil {
			return err
		}
		row, err := tx.CreateAuthSession(ctx, CreateSessionInput{
			ID:               s.ids.Generate(),
			TenantID:         account.TenantID,
			AccountID:        account.ID,
			RefreshTokenHash: hash,
			DeviceInfo:       device,
			IP:               ip,
			ExpireAt:         s.refreshExpireAt(),
		})
		if err != nil {
			return err
		}
		refresh = token
		session = row
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	access, err := s.auth.IssueAccess(account.TenantID, account.ID, session.ID, false)
	if err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	// 响应前解密手机号只用于 DTO 脱敏和角色审计,不把明文写回数据库或日志。
	phonePlain, err := s.decryptPhone(account.PhoneEnc)
	if err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	if err := s.auditLogin(ctx, account.TenantID, account.ID, audit.ActorRoleFromAccount(ToContractAccount(account, phonePlain))); err != nil {
		return LoginResponse{}, err
	}
	return LoginResponse{AccessToken: access, RefreshToken: refresh, MustChangePwd: account.MustChangePwd, Account: ptrAccountDTO(ToAccountDTO(account, phonePlain))}, nil
}

// refreshTenantSession 轮转租户 Refresh 会话并签发新 Token。
func (s *Service) refreshTenantSession(ctx context.Context, old AuthSession, device, ip string) (LoginResponse, error) {
	if old.Status != SessionStatusActive || timex.Now().After(old.ExpireAt) {
		if revokeErr := s.revokeAllTenantSessions(ctx, old.TenantID, old.AccountID); revokeErr != nil {
			logging.ErrorContext(ctx, "吊销过期租户会话失败", revokeErr.Error(), slog.Int64("tenant_id", old.TenantID), slog.Int64("account_id", old.AccountID))
		}
		return LoginResponse{}, apperr.ErrIdentitySessionInvalid
	}
	var account Account
	if err := s.store.TenantTx(ctx, old.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.RevokeAuthSession(ctx, old.TenantID, old.ID); err != nil {
			return err
		}
		a, err := tx.GetAccount(ctx, old.AccountID)
		if err != nil {
			return err
		}
		account = a
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrIdentitySessionInvalid.WithCause(err)
	}
	return s.issueTenantLogin(ctx, account, device, ip)
}

// refreshPlatformSession 轮转平台管理员 Refresh 会话并签发新 Token。
func (s *Service) refreshPlatformSession(ctx context.Context, old PlatformAuthSession, device, ip string) (LoginResponse, error) {
	if old.Status != SessionStatusActive || timex.Now().After(old.ExpireAt) {
		if revokeErr := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
			return tx.RevokePlatformSessions(ctx, old.PlatformAdminID)
		}); revokeErr != nil {
			logging.ErrorContext(ctx, "吊销过期平台会话失败", revokeErr.Error(), slog.Int64("tenant_id", 0), slog.String("operation_scope", "platform_session"), slog.Int64("platform_admin_id", old.PlatformAdminID))
		}
		return LoginResponse{}, apperr.ErrIdentitySessionInvalid
	}
	var refresh string
	var session PlatformAuthSession
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		if err := tx.RevokePlatformAuthSession(ctx, old.ID); err != nil {
			return err
		}
		token, err := crypto.RandomToken(48)
		if err != nil {
			return err
		}
		hash, err := s.hashSecret(token)
		if err != nil {
			return err
		}
		row, err := tx.CreatePlatformAuthSession(ctx, CreatePlatformSessionInput{ID: s.ids.Generate(), PlatformAdminID: old.PlatformAdminID, RefreshTokenHash: hash, DeviceInfo: device, IP: ip, ExpireAt: s.refreshExpireAt()})
		if err != nil {
			return err
		}
		refresh = token
		session = row
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	access, err := s.auth.IssueAccess(0, old.PlatformAdminID, session.ID, true)
	if err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	return LoginResponse{AccessToken: access, RefreshToken: refresh}, nil
}

// recordPasswordFailure 记录失败次数并在达到阈值时锁定账号。
func (s *Service) recordPasswordFailure(ctx context.Context, account Account) error {
	count := account.PwdFailedCount + 1
	var lockedUntil *time.Time
	if int(count) >= s.cfg.PasswordMaxFailedCount {
		t := timex.Now().Add(time.Duration(s.cfg.PasswordLockMinutes) * time.Minute)
		lockedUntil = &t
	}
	return s.store.TenantTx(ctx, account.TenantID, func(ctx context.Context, tx TxStore) error {
		return tx.RecordPasswordFailure(ctx, account.ID, account.TenantID, count, lockedUntil)
	})
}

// clearPasswordFailure 清理成功登录后的失败计数。
func (s *Service) clearPasswordFailure(ctx context.Context, account Account) error {
	return s.store.TenantTx(ctx, account.TenantID, func(ctx context.Context, tx TxStore) error {
		return tx.ClearPasswordFailure(ctx, account.ID, account.TenantID)
	})
}

// revokeAllTenantSessions 吊销账号全部会话,用于 refresh 重放处理。
func (s *Service) revokeAllTenantSessions(ctx context.Context, tenantID, accountID int64) error {
	return s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		return tx.RevokeAccountSessions(ctx, tenantID, accountID)
	})
}

// ptrAccountDTO 返回账号 DTO 指针。
func ptrAccountDTO(dto AccountDTO) *AccountDTO {
	return &dto
}

// appErrFromNoRows 将数据库未命中统一折叠为未登录语义。
func appErrFromNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrIdentityInvalidCredentials
	}
	return err
}

// auditLogin 构造登录审计条目,仅在业务明确调用时写入。
func (s *Service) auditLogin(ctx context.Context, tenantID, accountID int64, role int16) error {
	entry, err := audit.BuildEntry(ctx, tenantID, accountID, role, "auth.login", "identity.account", accountID, map[string]any{"result": "success"})
	if err != nil {
		return fmt.Errorf("构造登录审计失败: %w", err)
	}
	return s.auditWriter.Write(ctx, entry)
}

// auditLogout 构造登出审计条目,用于证明服务端会话已被主动吊销。
func (s *Service) auditLogout(ctx context.Context, tenantID, accountID int64, role int16) error {
	entry, err := audit.BuildEntry(ctx, tenantID, accountID, role, "auth.logout", "identity.auth_session", accountID, map[string]any{"result": "success"})
	if err != nil {
		return fmt.Errorf("构造登出审计失败: %w", err)
	}
	return s.auditWriter.Write(ctx, entry)
}
