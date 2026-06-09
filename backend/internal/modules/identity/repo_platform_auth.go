// M1 平台认证数据访问:集中处理平台管理员账号读取、平台会话写入和平台会话吊销。
package identity

import (
	"context"
	"errors"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// getPlatformAdminByUsername 按用户名读取平台管理员认证投影。
func (r *repo) getPlatformAdminByUsername(ctx context.Context, username string) (PlatformAdminSnapshot, error) {
	var row sqlcgen.PlatformAdmin
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		admin, err := q.GetPlatformAdminByUsername(ctx, username)
		if err != nil {
			return apperr.ErrWrongCredentials
		}
		row = admin
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return PlatformAdminSnapshot{}, ae
		}
		return PlatformAdminSnapshot{}, err
	}
	return platformAdminSnapshot(row), nil
}

// getPlatformAdminByID 按 ID 读取平台管理员认证投影。
func (r *repo) getPlatformAdminByID(ctx context.Context, id int64) (PlatformAdminSnapshot, error) {
	var row sqlcgen.PlatformAdmin
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		admin, err := q.GetPlatformAdminByID(ctx, id)
		if err != nil {
			return apperr.ErrRefreshInvalid
		}
		row = admin
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return PlatformAdminSnapshot{}, ae
		}
		return PlatformAdminSnapshot{}, err
	}
	return platformAdminSnapshot(row), nil
}

// createPlatformLoginSession 原子吊销旧平台会话、写入新会话和平台级审计。
func (r *repo) createPlatformLoginSession(ctx context.Context, adminID, sessionID int64, refreshHash, device, ip string, expireAt time.Time, auditLog AuditLogCreate) error {
	if !r.hasPrivileged() {
		return apperr.ErrIdentityPrivilegedRequired.WithCause(errors.New("平台登录审计写入需要特权连接"))
	}
	return r.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		// 平台管理员采用单点在线策略,新会话创建前撤销旧 refresh token。
		if err := q.RevokeAllPlatformAdminSessions(ctx, adminID); err != nil {
			return err
		}
		if _, err := q.CreatePlatformAuthSession(ctx, sqlcgen.CreatePlatformAuthSessionParams{
			ID:               sessionID,
			PlatformAdminID:  adminID,
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

// revokePlatformSession 吊销平台管理员当前会话。
func (r *repo) revokePlatformSession(ctx context.Context, sessionID int64) error {
	return r.inApp(ctx, func(q *sqlcgen.Queries) error {
		return q.RevokePlatformAuthSession(ctx, sessionID)
	})
}

// revokeAllPlatformAdminSessions 吊销平台管理员所有有效会话。
func (r *repo) revokeAllPlatformAdminSessions(ctx context.Context, adminID int64) error {
	return r.inApp(ctx, func(q *sqlcgen.Queries) error {
		return q.RevokeAllPlatformAdminSessions(ctx, adminID)
	})
}

// findPlatformSessionByTokenHash 按 refresh token 哈希定位平台管理员会话。
func (r *repo) findPlatformSessionByTokenHash(ctx context.Context, tokenHash string) (PlatformSessionSnapshot, bool, error) {
	var row sqlcgen.FindPlatformSessionByTokenHashRow
	found := true
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		session, err := q.FindPlatformSessionByTokenHash(ctx, tokenHash)
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
		return PlatformSessionSnapshot{}, false, apperr.ErrAuthSessionQueryFailed.WithCause(err)
	}
	if !found {
		return PlatformSessionSnapshot{}, false, nil
	}
	return PlatformSessionSnapshot{
		ID:              row.ID,
		PlatformAdminID: row.PlatformAdminID,
		Status:          row.Status,
		ExpireAt:        timex.FromTimestamptz(row.ExpireAt),
	}, true, nil
}

// platformAdminSnapshot 转换平台管理员行到认证投影。
func platformAdminSnapshot(row sqlcgen.PlatformAdmin) PlatformAdminSnapshot {
	return PlatformAdminSnapshot{
		ID:           row.ID,
		PasswordHash: row.PasswordHash,
		Name:         row.Name,
		Status:       row.Status,
	}
}
