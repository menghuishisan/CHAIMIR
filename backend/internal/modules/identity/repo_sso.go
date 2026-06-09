// M1 SSO 数据访问:集中处理启用配置读取和 SSO 首次匹配激活事务。
package identity

import (
	"context"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/pkg/apperr"
)

// getEnabledSsoConfig 读取租户当前启用的 SSO 配置。
func (r *repo) getEnabledSsoConfig(ctx context.Context, tenantID int64) (SsoConfigSnapshot, error) {
	var row sqlcgen.SsoConfig
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		cfg, err := q.GetSsoConfig(ctx, tenantID)
		if err != nil {
			if db.IsNoRows(err) {
				return apperr.ErrSsoUnavailable
			}
			return err
		}
		row = cfg
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return SsoConfigSnapshot{}, ae
		}
		return SsoConfigSnapshot{}, apperr.ErrSsoConfigReadFailed.WithCause(err)
	}
	return SsoConfigSnapshot{Type: row.Type, Config: row.Config, MatchField: row.MatchField}, nil
}

// activateSsoAccountWithAudit 激活 SSO 匹配到的待激活账号并写审计。
func (r *repo) activateSsoAccountWithAudit(ctx context.Context, acc LoginAccountSnapshot, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, acc.TenantID, func(q *sqlcgen.Queries) error {
		if err := q.SetAccountActivated(ctx, acc.ID); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}
