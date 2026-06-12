// identity service_session 文件实现 access token 对应服务端会话的即时有效性校验。
package identity

import (
	"context"
	"errors"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// ValidateAccessSession 校验 JWT 绑定的服务端会话仍有效,用于强制下线和单端登录即时生效。
func (s *Service) ValidateAccessSession(ctx context.Context, id auth.SessionIdentity) error {
	if id.IsPlatform {
		return s.validatePlatformAccessSession(ctx, id)
	}
	return s.validateTenantAccessSession(ctx, id)
}

// validateTenantAccessSession 检查租户会话的归属、状态和有效期。
func (s *Service) validateTenantAccessSession(ctx context.Context, id auth.SessionIdentity) error {
	if id.TenantID <= 0 || id.AccountID <= 0 || id.SessionID <= 0 {
		return apperr.ErrIdentitySessionInvalid
	}
	var session AuthSession
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetAuthSessionByID(ctx, id.TenantID, id.SessionID)
		if err != nil {
			return err
		}
		session = row
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrIdentitySessionInvalid
		}
		return apperr.ErrInternal.WithCause(err)
	}
	if session.TenantID != id.TenantID || session.AccountID != id.AccountID {
		return apperr.ErrIdentitySessionInvalid
	}
	if session.Status != SessionStatusActive || !timex.Now().Before(session.ExpireAt) {
		return apperr.ErrIdentitySessionInvalid
	}
	return nil
}

// validatePlatformAccessSession 检查平台管理员会话的归属、状态和有效期。
func (s *Service) validatePlatformAccessSession(ctx context.Context, id auth.SessionIdentity) error {
	if id.AccountID <= 0 || id.SessionID <= 0 {
		return apperr.ErrIdentitySessionInvalid
	}
	var session PlatformAuthSession
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetPlatformAuthSessionByID(ctx, id.SessionID)
		if err != nil {
			return err
		}
		session = row
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrIdentitySessionInvalid
		}
		return apperr.ErrInternal.WithCause(err)
	}
	if session.PlatformAdminID != id.AccountID {
		return apperr.ErrIdentitySessionInvalid
	}
	if session.Status != SessionStatusActive || !timex.Now().Before(session.ExpireAt) {
		return apperr.ErrIdentitySessionInvalid
	}
	return nil
}
