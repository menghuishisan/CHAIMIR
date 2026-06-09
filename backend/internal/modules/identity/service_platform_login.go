// M1 平台管理员认证服务:独立于租户账号体系签发平台级 Token。
package identity

import (
	"context"

	"chaimir/internal/contracts"
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
	admin, err := s.repo.getPlatformAdminByUsername(ctx, req.Username)
	if err != nil {
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

// issuePlatformLogin 创建平台级会话并签发 plat=true 的 Access Token,平台会话独立于学校租户 RLS。
func (s *Service) issuePlatformLogin(ctx context.Context, admin PlatformAdminSnapshot, device, ip string) (*LoginResult, error) {
	sessionID := s.idgen.Generate()
	// 先生成并哈希 refresh token,数据库只保存 HMAC 摘要。
	refreshPlain, err := crypto.RandomToken(48)
	if err != nil {
		return nil, apperr.ErrAuthTokenIssueFailed.WithCause(err)
	}
	refreshHash := crypto.HMACHash(s.hmacKey, refreshPlain)
	access, err := s.auth.IssueAccess(0, admin.ID, sessionID, true)
	if err != nil {
		return nil, apperr.ErrAuthTokenIssueFailed.WithCause(err)
	}

	// 再构造平台审计记录,审计与会话写入同一特权事务避免登录无留痕。
	entry, err := buildPlatformAuditEntry(ctx, admin.ID, AuditActionAuthLogin, AuditTargetAuthSession, sessionID, map[string]any{
		"device_recorded": device != "",
		"ip_recorded":     ip != "",
	})
	if err != nil {
		return nil, err
	}
	if err := s.repo.createPlatformLoginSession(ctx, admin.ID, sessionID, refreshHash, device, ip, timex.Now().Add(s.refreshTTL), buildAuditLogCreate(s.idgen.Generate(), entry)); err != nil {
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
