// M1 激活码服务:生成一次性激活码与用户自设密码激活账号。
package identity

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// CreateActivationCode 为待激活账号生成一次性激活码,明文只返回本次调用。
func (s *Service) CreateActivationCode(ctx context.Context, q *sqlcgen.Queries, tenantID, accountID, createdBy int64) (string, error) {
	code, err := s.genActivationCode()
	if err != nil {
		return "", apperr.ErrActivationCodeIssueFailed.WithCause(err)
	}
	_, err = q.CreateActivationCode(ctx, sqlcgen.CreateActivationCodeParams{
		ID:        s.idgen.Generate(),
		TenantID:  tenantID,
		AccountID: accountID,
		CodeHash:  s.activationCodeHash(code),
		ExpireAt:  timex.RequiredTimestamptz(timex.Now().Add(s.activationCodeTTL)),
		CreatedBy: pgInt8(createdBy, createdBy != 0),
	})
	if err != nil {
		return "", err
	}
	return code, nil
}

// activationCodeEnabled 读取租户是否启用激活码开通。
func (s *Service) activationCodeEnabled(ctx context.Context, tenantID int64) (bool, error) {
	var enabled bool
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		t, e := q.GetTenantByID(ctx, tenantID)
		if e != nil {
			return apperr.ErrTenantNotFound
		}
		enabled = t.EnableActivationCode
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return false, ae
		}
		return false, apperr.ErrTenantQueryFailed.WithCause(err)
	}
	return enabled, nil
}

// ActivateAccount 校验一次性激活码,设置用户密码并激活账号。
func (s *Service) ActivateAccount(ctx context.Context, req ActivateAccountRequest) error {
	if !validPassword(req.Password) {
		return apperr.ErrWeakPassword
	}
	hash := s.activationCodeHash(req.ActivationCode)
	row, err := s.findActivationCodeByHash(ctx, hash)
	if err != nil {
		return err
	}
	if row.Status != ActivationCodeActive || row.ExpireAt.Time.Before(timex.Now()) {
		return apperr.ErrActivationCodeInvalid
	}
	pwdHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	if err := s.repo.inTenantID(ctx, row.TenantID, func(q *sqlcgen.Queries) error {
		acc, e := q.GetAccountByID(ctx, row.AccountID)
		if e != nil {
			return apperr.ErrAccountNotFound
		}
		if acc.Status != AccountPending {
			return apperr.ErrActivationCodeInvalid
		}
		if e := q.UpdateAccountPassword(ctx, sqlcgen.UpdateAccountPasswordParams{
			ID: row.AccountID, PasswordHash: pgText(pwdHash), MustChangePwd: false,
		}); e != nil {
			return e
		}
		if e := q.SetAccountActivated(ctx, row.AccountID); e != nil {
			return e
		}
		if e := q.MarkActivationCodeUsed(ctx, row.ID); e != nil {
			return e
		}
		roles, e := q.ListAccountRoles(ctx, row.AccountID)
		if e != nil {
			return e
		}
		return s.writeAccountAuditInTx(ctx, q, row.TenantID, row.AccountID, audit.ActorRoleFromAccount(contracts.AccountInfo{
			BaseIdentity: acc.BaseIdentity,
			Roles:        roleCodesOf(roles),
		}), AuditActionAccountUpdate, AuditTargetAccount, row.AccountID, map[string]any{
			"fields": []string{"password", "status"},
			"source": "activation_code",
		})
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrAccountMutationFailed.WithCause(err)
	}
	return nil
}

// findActivationCodeByHash 使用特权连接按激活码哈希定位租户与账号。
func (s *Service) findActivationCodeByHash(ctx context.Context, hash string) (sqlcgen.ActivationCode, error) {
	if !s.repo.hasPrivileged() {
		return sqlcgen.ActivationCode{}, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("激活码登录前定位需特权连接"))
	}
	var row sqlcgen.ActivationCode
	if err := s.repo.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		r, e := q.GetActivationCodeByHash(ctx, hash)
		if e != nil {
			return apperr.ErrActivationCodeInvalid
		}
		row = r
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.ActivationCode{}, ae
		}
		return sqlcgen.ActivationCode{}, apperr.ErrActivationLookupUnavailable.WithCause(err)
	}
	return row, nil
}

// genActivationCode 生成高熵激活码,明文只用于本次开通响应。
func (s *Service) genActivationCode() (string, error) {
	return crypto.RandomToken(24)
}

// activationCodeHash 计算激活码哈希。
func (s *Service) activationCodeHash(code string) string {
	return crypto.HMACHash(s.hmacKey, code)
}
