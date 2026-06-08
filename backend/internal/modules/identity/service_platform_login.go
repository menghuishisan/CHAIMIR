// M1 平台管理员认证服务:独立于租户账号体系签发平台级 Token。
package identity

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// LoginPlatform 校验 SaaS 平台管理员凭据并签发平台级双 Token。
func (s *Service) LoginPlatform(ctx context.Context, req PlatformLoginRequest, device, ip string) (*LoginResult, error) {
	if !s.cfg.PlatformEnabled {
		return nil, apperr.ErrForbidden
	}
	var admin sqlcgen.PlatformAdmin
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.GetPlatformAdminByUsername(ctx, req.Username)
		if e != nil {
			return apperr.ErrWrongCredentials
		}
		admin = row
		return nil
	}); err != nil {
		return nil, toAppErr(err)
	}
	if admin.Status != TenantActive {
		return nil, apperr.ErrAccountDisabled
	}
	ok, err := crypto.VerifyPassword(req.Password, admin.PasswordHash)
	if err != nil {
		return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	if !ok {
		return nil, apperr.ErrWrongCredentials
	}
	return s.issuePlatformLogin(ctx, admin, device, ip)
}

// issuePlatformLogin 创建平台级会话并签发 plat=true 的 Access Token。
func (s *Service) issuePlatformLogin(ctx context.Context, admin sqlcgen.PlatformAdmin, device, ip string) (*LoginResult, error) {
	sessionID := s.idgen.Generate()
	refreshPlain, err := crypto.RandomToken(48)
	if err != nil {
		return nil, apperr.ErrAuthTokenIssueFailed.WithCause(err)
	}
	refreshHash := crypto.HMACHash(s.hmacKey, refreshPlain)
	access, err := s.auth.IssueAccess(0, admin.ID, sessionID, true)
	if err != nil {
		return nil, apperr.ErrAuthTokenIssueFailed.WithCause(err)
	}

	entry, err := buildPlatformAuditEntry(ctx, admin.ID, AuditActionAuthLogin, AuditTargetAuthSession, sessionID, map[string]any{
		"device_recorded": device != "",
		"ip_recorded":     ip != "",
	})
	if err != nil {
		return nil, err
	}
	if !s.repo.hasPrivileged() {
		return nil, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("平台登录审计写入需要特权连接"))
	}
	if err := s.repo.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		if e := q.RevokeAllPlatformAdminSessions(ctx, admin.ID); e != nil {
			return e
		}
		if _, e := q.CreatePlatformAuthSession(ctx, sqlcgen.CreatePlatformAuthSessionParams{
			ID:               sessionID,
			PlatformAdminID:  admin.ID,
			RefreshTokenHash: refreshHash,
			DeviceInfo:       pgText(device),
			Ip:               pgText(ip),
			ExpireAt:         timex.RequiredTimestamptz(timex.Now().Add(s.refreshTTL)),
		}); e != nil {
			return e
		}
		return q.CreateAuditLog(ctx, buildAuditLogParams(s.idgen.Generate(), entry))
	}); err != nil {
		return nil, apperr.ErrPlatformAuthSessionFailed.WithCause(err)
	}

	return &LoginResult{
		AccessToken:  access,
		RefreshToken: refreshPlain,
		Account: &AccountBrief{
			ID:           ids.Format(admin.ID),
			Name:         admin.Name,
			BaseIdentity: RolePlatformAdmin,
			Roles:        []string{contracts.RoleCode(RolePlatformAdmin)},
		},
	}, nil
}
