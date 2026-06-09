// M1 激活码服务:生成一次性激活码与用户自设密码激活账号。
package identity

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// activationCodeEnabled 读取租户是否启用激活码开通。
func (s *Service) activationCodeEnabled(ctx context.Context, tenantID int64) (bool, error) {
	return s.repo.activationCodeEnabled(ctx, tenantID)
}

// ActivateAccount 校验一次性激活码,设置用户密码并激活账号;激活码登录前使用哈希定位租户。
func (s *Service) ActivateAccount(ctx context.Context, req ActivateAccountRequest) error {
	if !validPassword(req.Password) {
		return apperr.ErrWeakPassword
	}
	// 先用不可逆哈希定位激活码,避免明文激活码进入数据库查询和日志链路。
	hash := s.activationCodeHash(req.ActivationCode)
	row, err := s.repo.findActivationCodeByHash(ctx, hash)
	if err != nil {
		return err
	}
	if row.Status != ActivationCodeActive || row.ExpireAt.Before(timex.Now()) {
		return apperr.ErrActivationCodeInvalid
	}
	// 再生成密码哈希,明文密码只在本次请求内短暂存在。
	pwdHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	entry, err := buildAccountAuditEntry(ctx, row.TenantID, row.AccountID, audit.ActorRoleFromAccount(contracts.AccountInfo{
		BaseIdentity: row.BaseIdentity,
		Roles:        row.Roles,
	}), AuditActionAccountUpdate, AuditTargetAccount, row.AccountID, map[string]any{
		"fields": []string{"password", "status"},
		"source": "activation_code",
	})
	if err != nil {
		return err
	}
	// 最后进入目标租户事务,原子完成密码写入、账号激活、激活码作废和审计。
	if err := s.repo.activateAccountWithCode(ctx, row, pwdHash, buildAuditLogCreate(s.idgen.Generate(), entry)); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrAccountMutationFailed.WithCause(err)
	}
	return nil
}

// genActivationCode 生成高熵激活码,明文只用于本次开通响应。
func (s *Service) genActivationCode() (string, error) {
	return crypto.RandomToken(24)
}

// activationCodeHash 计算激活码哈希。
func (s *Service) activationCodeHash(code string) string {
	return crypto.HMACHash(s.hmacKey, code)
}
