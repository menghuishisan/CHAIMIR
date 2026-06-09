// M1 激活码数据访问:集中处理 activation_code 读取、写入和账号激活事务。
package identity

import (
	"context"
	"errors"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// createActivationCodeInTx 在已有租户事务内写入一次性激活码。
func (r *repo) createActivationCodeInTx(ctx context.Context, q *sqlcgen.Queries, id, tenantID, accountID int64, codeHash string, expireAt time.Time, createdBy int64) error {
	_, err := q.CreateActivationCode(ctx, sqlcgen.CreateActivationCodeParams{
		ID:        id,
		TenantID:  tenantID,
		AccountID: accountID,
		CodeHash:  codeHash,
		ExpireAt:  timex.RequiredTimestamptz(expireAt),
		CreatedBy: pgtypex.Int8When(createdBy, createdBy != 0),
	})
	return err
}

// activationCodeEnabled 读取租户是否启用激活码开通。
func (r *repo) activationCodeEnabled(ctx context.Context, tenantID int64) (bool, error) {
	var enabled bool
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		t, err := q.GetTenantByID(ctx, tenantID)
		if err != nil {
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

// findActivationCodeByHash 使用特权连接按激活码哈希定位租户与账号。
func (r *repo) findActivationCodeByHash(ctx context.Context, hash string) (ActivationCodeSnapshot, error) {
	if !r.hasPrivileged() {
		return ActivationCodeSnapshot{}, apperr.ErrIdentityPrivilegedRequired.WithCause(errors.New("激活码登录前定位需特权连接"))
	}
	var row sqlcgen.ActivationCode
	if err := r.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.GetActivationCodeByHash(ctx, hash)
		if err != nil {
			return apperr.ErrActivationCodeInvalid
		}
		row = found
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ActivationCodeSnapshot{}, ae
		}
		return ActivationCodeSnapshot{}, apperr.ErrActivationLookupUnavailable.WithCause(err)
	}
	var acc sqlcgen.Account
	var roles []int16
	if err := r.inTenantID(ctx, row.TenantID, func(q *sqlcgen.Queries) error {
		found, err := q.GetAccountByID(ctx, row.AccountID)
		if err != nil {
			return apperr.ErrAccountNotFound
		}
		acc = found
		roles, err = q.ListAccountRoles(ctx, row.AccountID)
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ActivationCodeSnapshot{}, ae
		}
		return ActivationCodeSnapshot{}, err
	}
	return ActivationCodeSnapshot{
		ID:           row.ID,
		TenantID:     row.TenantID,
		AccountID:    row.AccountID,
		Status:       row.Status,
		ExpireAt:     timex.FromTimestamptz(row.ExpireAt),
		BaseIdentity: acc.BaseIdentity,
		Roles:        roleCodesOf(roles),
	}, nil
}

// activateAccountWithCode 原子完成密码写入、账号激活、激活码作废和审计。
func (r *repo) activateAccountWithCode(ctx context.Context, row ActivationCodeSnapshot, passwordHash string, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, row.TenantID, func(q *sqlcgen.Queries) error {
		acc, err := q.GetAccountByID(ctx, row.AccountID)
		if err != nil {
			return apperr.ErrAccountNotFound
		}
		if acc.Status != AccountPending {
			return apperr.ErrActivationCodeInvalid
		}
		if err := q.UpdateAccountPassword(ctx, sqlcgen.UpdateAccountPasswordParams{
			ID: row.AccountID, PasswordHash: pgtypex.Text(passwordHash), MustChangePwd: false,
		}); err != nil {
			return err
		}
		if err := q.SetAccountActivated(ctx, row.AccountID); err != nil {
			return err
		}
		if err := q.MarkActivationCodeUsed(ctx, row.ID); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}
