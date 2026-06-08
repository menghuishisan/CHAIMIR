// M5 审计辅助:集中定义题库模块动作码,并经 platform/audit.Writer 写入 M1 audit_log。
package content

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

const (
	auditTargetItem     = "content.item"
	auditTargetCategory = "content.category"
	auditTargetPaper    = "content.paper"

	auditActionItemCreate    = "content.item.create"
	auditActionItemUpdate    = "content.item.update"
	auditActionItemPublish   = "content.item.publish"
	auditActionItemDeprecate = "content.item.deprecate"
	auditActionItemDelete    = "content.item.delete"
	auditActionItemClone     = "content.item.clone"
	auditActionItemShare     = "content.item.share"
	auditActionItemUnshare   = "content.item.unshare"
	auditActionItemUsage     = "content.item.usage"
	auditActionCategorySave  = "content.category.save"
	auditActionPaperSave     = "content.paper.save"
)

// writeAudit 写关键操作审计;缺少审计 writer 时显式失败,避免绕过 audit_log。
func (s *Service) writeAudit(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	if s.auditor == nil {
		return apperr.ErrInternal
	}
	actorID, actorRole, err := audit.ResolveActor(ctx, s.identity)
	if err != nil {
		return err
	}
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return err
	}
	return s.auditor.Write(ctx, entry)
}
