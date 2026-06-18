// notify audit 文件封装 M10 公告审计 action 和共享 audit_log 写入。
package notify

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

const auditTargetAnnouncement = "notify.announcement"

// writeAudit 写入 M1 共享 audit_log,通知模块不得自建审计表。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrNotifyAnnouncementInvalid.WithCause(err)
	}
	if err := s.audit.Write(ctx, entry); err != nil {
		return apperr.ErrNotifyAnnouncementInvalid.WithCause(err)
	}
	return nil
}

// auditRoleForIdentity 把公告发布身份映射到审计角色。
func auditRoleForIdentity(id tenant.Identity) int16 {
	if id.IsPlatform {
		return contracts.RoleNumPlatformAdmin
	}
	return contracts.RoleNumSchoolAdmin
}
