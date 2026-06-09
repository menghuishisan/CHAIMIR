// M1 短信验证码数据访问:集中处理 sms_code 和手机号租户候选查询。
package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// createSmsCode 写入验证码哈希记录,找回场景允许 tenant_id 为空。
func (r *repo) createSmsCode(ctx context.Context, tenantID, id int64, phoneHash, codeHash string, scene int16, expireAt time.Time) error {
	params := sqlcgen.CreateSmsCodeParams{
		ID:        id,
		TenantID:  pgtypex.Int8When(tenantID, tenantID != 0),
		PhoneHash: phoneHash,
		CodeHash:  codeHash,
		Scene:     scene,
		ExpireAt:  timex.RequiredTimestamptz(expireAt),
	}
	writeFn := func(q *sqlcgen.Queries) error {
		_, err := q.CreateSmsCode(ctx, params)
		return err
	}
	return r.smsScopedExec(ctx, tenantID, "找回验证码需特权连接写入", writeFn)
}

// getLatestSmsCode 读取最新未使用且未过期的验证码。
func (r *repo) getLatestSmsCode(ctx context.Context, tenantID int64, phoneHash string, scene int16) (SmsCodeSnapshot, error) {
	var row sqlcgen.SmsCode
	readFn := func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetLatestSmsCode(ctx, sqlcgen.GetLatestSmsCodeParams{PhoneHash: phoneHash, Scene: scene})
		if err != nil {
			return apperr.ErrSmsCodeInvalid
		}
		return nil
	}
	if err := r.smsScopedExec(ctx, tenantID, "找回验证码需特权连接校验", readFn); err != nil {
		return SmsCodeSnapshot{}, err
	}
	return SmsCodeSnapshot{
		ID:       row.ID,
		CodeHash: row.CodeHash,
		ExpireAt: timex.FromTimestamptz(row.ExpireAt),
	}, nil
}

// markSmsCodeUsed 把验证码标记为已使用或失效。
func (r *repo) markSmsCodeUsed(ctx context.Context, tenantID, codeID int64) error {
	return r.smsScopedExec(ctx, tenantID, "找回验证码需特权连接校验", func(q *sqlcgen.Queries) error {
		return q.MarkSmsCodeUsed(ctx, codeID)
	})
}

// findAccountTenantCandidatesByPhone 跨租户按手机号哈希查找可登录账号候选。
func (r *repo) findAccountTenantCandidatesByPhone(ctx context.Context, phoneHash string) ([]AccountTenantCandidate, error) {
	if !r.hasPrivileged() {
		return nil, apperr.ErrIdentityPrivilegedRequired.WithCause(fmt.Errorf("手机号跨租户定位需要特权连接"))
	}
	var rows []sqlcgen.FindAccountsByPhoneAllTenantsRow
	if err := r.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.FindAccountsByPhoneAllTenants(ctx, phoneHash)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]AccountTenantCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, AccountTenantCandidate{TenantID: row.TenantID})
	}
	return out, nil
}

// smsScopedExec 根据验证码是否已定位租户选择 RLS 或特权事务。
func (r *repo) smsScopedExec(ctx context.Context, tenantID int64, privilegedMessage string, fn queryFunc) error {
	if tenantID != 0 {
		return r.inTenantID(ctx, tenantID, fn)
	}
	if !r.hasPrivileged() {
		return apperr.ErrIdentityPrivilegedRequired.WithCause(errors.New(privilegedMessage))
	}
	return r.inPrivileged(ctx, fn)
}
