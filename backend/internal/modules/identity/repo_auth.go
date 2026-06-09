// M1 认证数据访问:集中处理租户账号登录、Refresh 会话和找回密码的持久化事务。
package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// findLoginCandidatesByPhone 跨租户按手机号哈希读取登录候选。
func (r *repo) findLoginCandidatesByPhone(ctx context.Context, phoneHash string) ([]LoginTenantCandidate, error) {
	if !r.hasPrivileged() {
		return nil, apperr.ErrIdentityPrivilegedRequired.WithCause(errors.New("未配置特权连接,无法执行跨租户登录定位"))
	}
	var rows []sqlcgen.FindAccountsByPhoneAllTenantsRow
	if err := r.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.FindAccountsByPhoneAllTenants(ctx, phoneHash)
		if err != nil {
			return err
		}
		rows = found
		return nil
	}); err != nil {
		return nil, apperr.ErrAuthLookupUnavailable.WithCause(err)
	}
	out := make([]LoginTenantCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, LoginTenantCandidate{
			AccountID: row.AccountID, TenantID: row.TenantID, Name: row.Name,
			TenantCode: row.TenantCode, TenantName: row.TenantName, TenantStatus: row.TenantStatus,
		})
	}
	return out, nil
}

// loadAccountByPhone 在指定租户内按 phone_hash 读取认证账号投影。
func (r *repo) loadAccountByPhone(ctx context.Context, tenantID int64, phoneHash string) (LoginAccountSnapshot, error) {
	var acc sqlcgen.Account
	var roles []int16
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, err := q.GetAccountByPhoneHash(ctx, phoneHash)
		if err != nil {
			return apperr.ErrWrongCredentials
		}
		acc = found
		roles, err = q.ListAccountRoles(ctx, found.ID)
		return err
	}); err != nil {
		return LoginAccountSnapshot{}, err
	}
	return loginAccountSnapshot(acc, roles), nil
}

// loadAccountByNo 在指定租户内按学号或工号读取认证账号投影。
func (r *repo) loadAccountByNo(ctx context.Context, tenantID int64, no string) (LoginAccountSnapshot, error) {
	var acc sqlcgen.Account
	var roles []int16
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		profile, err := q.GetAccountProfileByNo(ctx, no)
		if err != nil {
			return apperr.ErrWrongCredentials
		}
		found, err := q.GetAccountByID(ctx, profile.AccountID)
		if err != nil {
			return apperr.ErrWrongCredentials
		}
		acc = found
		roles, err = q.ListAccountRoles(ctx, found.ID)
		return err
	}); err != nil {
		return LoginAccountSnapshot{}, err
	}
	return loginAccountSnapshot(acc, roles), nil
}

// getTenantByCode 按学校短码读取认证入口租户投影。
func (r *repo) getTenantByCode(ctx context.Context, code string) (TenantLoginSnapshot, error) {
	var out TenantLoginSnapshot
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		tenant, err := q.GetTenantByCode(ctx, code)
		if err != nil {
			return apperr.ErrTenantNotFound
		}
		out = TenantLoginSnapshot{ID: tenant.ID, Status: tenant.Status}
		return nil
	}); err != nil {
		return TenantLoginSnapshot{}, err
	}
	return out, nil
}

// getTenantByIDForLogin 按租户 ID 读取认证入口租户投影。
func (r *repo) getTenantByIDForLogin(ctx context.Context, tenantID int64) (TenantLoginSnapshot, error) {
	var out TenantLoginSnapshot
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		tenant, err := q.GetTenantByID(ctx, tenantID)
		if err != nil {
			return apperr.ErrTenantNotFound
		}
		out = TenantLoginSnapshot{ID: tenant.ID, Status: tenant.Status}
		return nil
	}); err != nil {
		return TenantLoginSnapshot{}, err
	}
	return out, nil
}

// incrementPasswordFailure 记录密码登录失败次数并按策略锁定账号。
func (r *repo) incrementPasswordFailure(ctx context.Context, tenantID, accountID int64, maxFailed int16, lockMinutes int) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.IncrAccountPwdFailed(ctx, sqlcgen.IncrAccountPwdFailedParams{
			ID:             accountID,
			PwdFailedCount: maxFailed,
			Column3:        pgtypex.Text(fmt.Sprintf("%d", lockMinutes)),
		})
		return err
	})
}

// createLoginSession 原子清理失败计数、踢掉旧会话、写新会话和登录审计。
func (r *repo) createLoginSession(ctx context.Context, acc LoginAccountSnapshot, sessionID int64, refreshHash, device, ip string, expireAt time.Time, resetFailed bool, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, acc.TenantID, func(q *sqlcgen.Queries) error {
		if resetFailed {
			if err := q.ResetAccountPwdFailed(ctx, acc.ID); err != nil {
				return err
			}
		}
		// 单端登录:吊销该账号其余有效会话。
		if err := q.RevokeAllAccountSessions(ctx, acc.ID); err != nil {
			return err
		}
		if _, err := q.CreateAuthSession(ctx, sqlcgen.CreateAuthSessionParams{
			ID:               sessionID,
			TenantID:         acc.TenantID,
			AccountID:        acc.ID,
			RefreshTokenHash: refreshHash,
			DeviceInfo:       pgtypex.Text(device),
			Ip:               pgtypex.Text(ip),
			ExpireAt:         timex.RequiredTimestamptz(expireAt),
		}); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// findSessionByTokenHash 跨租户按 Refresh Token 哈希定位会话。
func (r *repo) findSessionByTokenHash(ctx context.Context, tokenHash string) (AuthSessionSnapshot, bool, error) {
	if !r.hasPrivileged() {
		return AuthSessionSnapshot{}, false, apperr.ErrIdentityPrivilegedRequired.WithCause(errors.New("未配置特权连接,无法刷新令牌"))
	}
	var row sqlcgen.FindSessionByTokenHashRow
	found := true
	if err := r.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		session, err := q.FindSessionByTokenHash(ctx, tokenHash)
		if err != nil {
			if db.IsNoRows(err) {
				found = false
				return nil
			}
			return err
		}
		row = session
		return nil
	}); err != nil {
		return AuthSessionSnapshot{}, false, apperr.ErrAuthSessionQueryFailed.WithCause(err)
	}
	if !found {
		return AuthSessionSnapshot{}, false, nil
	}
	return AuthSessionSnapshot{
		ID: row.ID, TenantID: row.TenantID, AccountID: row.AccountID,
		Status: row.Status, ExpireAt: timex.FromTimestamptz(row.ExpireAt),
	}, true, nil
}

// loadAccountByIDForAuth 按账号 ID 读取认证账号投影。
func (r *repo) loadAccountByIDForAuth(ctx context.Context, tenantID, accountID int64) (LoginAccountSnapshot, error) {
	var acc sqlcgen.Account
	var roles []int16
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, err := q.GetAccountByID(ctx, accountID)
		if err != nil {
			return apperr.ErrRefreshInvalid
		}
		acc = found
		roles, err = q.ListAccountRoles(ctx, accountID)
		return err
	}); err != nil {
		return LoginAccountSnapshot{}, err
	}
	return loginAccountSnapshot(acc, roles), nil
}

// revokeAllAccountSessions 吊销某账号全部有效会话。
func (r *repo) revokeAllAccountSessions(ctx context.Context, tenantID, accountID int64) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		return q.RevokeAllAccountSessions(ctx, accountID)
	})
}

// revokeAuthSessionWithAudit 吊销当前租户会话并写入登出审计。
func (r *repo) revokeAuthSessionWithAudit(ctx context.Context, tenantID, sessionID int64, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if err := q.RevokeAuthSession(ctx, sessionID); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// resetPasswordWithAudit 写入找回密码的新密码、清失败计数、吊销会话并写审计。
func (r *repo) resetPasswordWithAudit(ctx context.Context, target LoginTenantCandidate, passwordHash string, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, target.TenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.GetAccountByID(ctx, target.AccountID); err != nil {
			return apperr.ErrAccountNotFound
		}
		if err := q.UpdateAccountPassword(ctx, sqlcgen.UpdateAccountPasswordParams{
			ID: target.AccountID, PasswordHash: pgtypex.Text(passwordHash), MustChangePwd: false,
		}); err != nil {
			return err
		}
		if err := q.ResetAccountPwdFailed(ctx, target.AccountID); err != nil {
			return err
		}
		// 找回密码后吊销全部会话(强制重新登录),并在同一事务内记录账号安全变更审计。
		if err := q.RevokeAllAccountSessions(ctx, target.AccountID); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// loginAccountSnapshot 转换账号行和角色为认证投影。
func loginAccountSnapshot(row sqlcgen.Account, roles []int16) LoginAccountSnapshot {
	return LoginAccountSnapshot{
		ID: row.ID, TenantID: row.TenantID, PasswordHash: textVal(row.PasswordHash),
		HasPassword: row.PasswordHash.Valid, Name: row.Name, BaseIdentity: row.BaseIdentity,
		Status: row.Status, MustChangePwd: row.MustChangePwd,
		LockedUntil: timex.FromTimestamptz(row.LockedUntil), HasLockedUntil: row.LockedUntil.Valid,
		Roles: roleCodesOf(roles),
	}
}
