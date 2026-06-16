// identity service_me 文件实现个人中心信息、改密、换绑手机号和会话查看业务编排。
package identity

import (
	"context"
	"errors"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5"
)

// GetMe 读取当前登录账号信息,手机号只返回脱敏展示。
func (s *Service) GetMe(ctx context.Context) (MeResponse, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return MeResponse{}, apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return s.getPlatformMe(ctx, id.AccountID)
	}
	var account Account
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetAccount(ctx, id.AccountID)
		if err != nil {
			return err
		}
		account = row
		return nil
	}); err != nil {
		return MeResponse{}, apperr.ErrIdentitySessionInvalid.WithCause(err)
	}
	phone, err := s.decryptPhone(account.PhoneEnc)
	if err != nil {
		return MeResponse{}, apperr.ErrInternal.WithCause(err)
	}
	return MeResponse{Account: ToAccountDTO(account, phone)}, nil
}

// ChangeMyPassword 校验旧密码后更新为新密码,保留当前会话并吊销其他设备会话。
func (s *Service) ChangeMyPassword(ctx context.Context, currentSessionID int64, req ChangePasswordRequest) error {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if currentSessionID <= 0 {
		return apperr.ErrIdentitySessionContextMissing
	}
	if id.IsPlatform {
		return s.changePlatformPassword(ctx, id.AccountID, currentSessionID, req)
	}
	if id.TenantID <= 0 || id.AccountID <= 0 {
		return apperr.ErrForbidden
	}
	if err := ValidatePassword(req.NewPassword); err != nil {
		return err
	}
	var account Account
	passwordHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		// 改密前必须读取服务端账号状态并校验旧密码,不能只依赖当前 access token。
		row, err := tx.GetAccount(ctx, id.AccountID)
		if err != nil {
			return err
		}
		if err := EnsureAccountCanLogin(row, timex.Now()); err != nil {
			return err
		}
		ok, err := crypto.VerifyPassword(req.OldPassword, row.PasswordHash)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.ErrIdentityInvalidCredentials
		}
		updated, err := tx.UpdateAccountPassword(ctx, id.AccountID, id.TenantID, passwordHash, false, AccountStatusActive)
		if err != nil {
			return err
		}
		// 密码变更后保留当前已校验会话,吊销其他设备降低凭据泄露面且不打断首登改密。
		if err := tx.RevokeOtherAccountSessions(ctx, id.TenantID, id.AccountID, currentSessionID); err != nil {
			return err
		}
		account = row
		account.Status = updated.Status
		account.MustChangePwd = false
		return nil
	}); err != nil {
		return apperr.AsAppError(err)
	}
	return s.auditSelfAccount(ctx, id, account, "account.password.change")
}

// ChangeMyPhone 校验新手机号短信验证码后换绑手机号密文和查询哈希。
func (s *Service) ChangeMyPhone(ctx context.Context, req ChangePhoneRequest) error {
	id, err := requireTenantSession(ctx)
	if err != nil {
		return err
	}
	if err := ValidatePhone(req.Phone); err != nil {
		return err
	}
	phoneHash, err := s.phoneHash(req.Phone)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	if err := s.ensurePhoneAvailable(ctx, id, phoneHash); err != nil {
		return err
	}
	if err := s.verifySMSCode(ctx, id.TenantID, req.Phone, SMSSceneChangePhone, req.Code); err != nil {
		return err
	}
	phoneEnc, err := s.encryptPhone(req.Phone)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	var account Account
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		row, err := tx.UpdateAccountPhone(ctx, id.TenantID, id.AccountID, phoneEnc, phoneHash)
		if err != nil {
			return err
		}
		account = row
		return nil
	}); err != nil {
		return apperr.AsAppError(err)
	}
	return s.auditSelfAccount(ctx, id, account, "account.phone.change")
}

// ListMySessions 读取当前账号的服务端 Refresh 会话,不返回任何令牌明文或哈希。
func (s *Service) ListMySessions(ctx context.Context) ([]SessionDTO, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return s.listPlatformSessions(ctx, id.AccountID)
	}
	if id.TenantID <= 0 || id.AccountID <= 0 {
		return nil, apperr.ErrForbidden
	}
	var sessions []AuthSession
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		rows, err := tx.ListAuthSessionsByAccount(ctx, id.TenantID, id.AccountID)
		if err != nil {
			return err
		}
		sessions = rows
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]SessionDTO, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, ToSessionDTO(session))
	}
	return out, nil
}

// getPlatformMe 返回平台管理员个人信息,平台账号不属于任何租户。
func (s *Service) getPlatformMe(ctx context.Context, accountID int64) (MeResponse, error) {
	if accountID <= 0 {
		return MeResponse{}, apperr.ErrForbidden
	}
	var admin PlatformAdmin
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetPlatformAdminByID(ctx, accountID)
		if err != nil {
			return err
		}
		admin = row
		return nil
	}); err != nil {
		return MeResponse{}, apperr.ErrIdentitySessionInvalid.WithCause(err)
	}
	return MeResponse{Account: AccountDTO{ID: admin.ID, Name: admin.Name, Roles: []int16{contracts.RoleNumPlatformAdmin}, Status: admin.Status}}, nil
}

// changePlatformPassword 校验平台管理员旧密码并保留当前会话、吊销其他会话。
func (s *Service) changePlatformPassword(ctx context.Context, accountID, currentSessionID int64, req ChangePasswordRequest) error {
	if accountID <= 0 {
		return apperr.ErrForbidden
	}
	if err := ValidatePassword(req.NewPassword); err != nil {
		return err
	}
	passwordHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	var admin PlatformAdmin
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetPlatformAdminByID(ctx, accountID)
		if err != nil {
			return err
		}
		ok, err := crypto.VerifyPassword(req.OldPassword, row.PasswordHash)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.ErrIdentityPlatformOldPasswordInvalid
		}
		if err := tx.UpdatePlatformAdminPassword(ctx, accountID, passwordHash); err != nil {
			return err
		}
		if err := tx.RevokeOtherPlatformSessions(ctx, accountID, currentSessionID); err != nil {
			return err
		}
		admin = row
		return nil
	}); err != nil {
		return apperr.AsAppError(err)
	}
	return s.auditPlatformOperation(ctx, admin.ID, "platform_admin.password.change", "identity.platform_admin", admin.ID, map[string]any{})
}

// listPlatformSessions 返回平台管理员服务端会话列表,不暴露 Refresh 哈希。
func (s *Service) listPlatformSessions(ctx context.Context, accountID int64) ([]SessionDTO, error) {
	if accountID <= 0 {
		return nil, apperr.ErrForbidden
	}
	var sessions []PlatformAuthSession
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		rows, err := tx.ListPlatformAuthSessionsByAdmin(ctx, accountID)
		if err != nil {
			return err
		}
		sessions = rows
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]SessionDTO, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, ToPlatformSessionDTO(session))
	}
	return out, nil
}

// ensurePhoneAvailable 确认新手机号未被同租户其他账号占用。
func (s *Service) ensurePhoneAvailable(ctx context.Context, id tenant.Identity, phoneHash string) error {
	return s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		account, err := tx.GetAccountByPhoneHash(ctx, id.TenantID, phoneHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if account.ID != id.AccountID {
			return apperr.ErrIdentityPhoneAlreadyUsed
		}
		return nil
	})
}

// requireTenantSession 读取当前租户登录身份,拒绝平台身份进入租户个人中心。
func requireTenantSession(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	if id.IsPlatform || id.TenantID <= 0 || id.AccountID <= 0 {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	return id, nil
}

// auditSelfAccount 写入个人中心敏感操作审计,角色来自服务端账号快照。
func (s *Service) auditSelfAccount(ctx context.Context, id tenant.Identity, account Account, action string) error {
	entry, err := audit.BuildEntry(ctx, id.TenantID, id.AccountID, audit.ActorRoleFromAccount(ToContractAccount(account, "")), action, "identity.account", id.AccountID, map[string]any{})
	if err != nil {
		return err
	}
	return s.auditWriter.Write(ctx, entry)
}
