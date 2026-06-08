// M2 审计辅助:集中定义沙箱模块动作码,并经 platform/audit.Writer 写入 M1 audit_log。
package sandbox

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

const (
	auditTargetRuntime      = "sandbox.runtime"
	auditTargetRuntimeImage = "sandbox.runtime_image"
	auditTargetTool         = "sandbox.tool"
	auditTargetSandbox      = "sandbox.sandbox"
	auditTargetQuota        = "sandbox.tenant_quota"

	auditActionRuntimeCreate       = "sandbox.runtime.create"
	auditActionRuntimeUpdate       = "sandbox.runtime.update"
	auditActionRuntimeSelftest     = "sandbox.runtime.selftest"
	auditActionRuntimeImageCreate  = "sandbox.runtime_image.create"
	auditActionRuntimeImagePrepull = "sandbox.runtime_image.prepull"
	auditActionToolCreate          = "sandbox.tool.create"
	auditActionSandboxCreate       = "sandbox.sandbox.create"
	auditActionSandboxPause        = "sandbox.sandbox.pause"
	auditActionSandboxResume       = "sandbox.sandbox.resume"
	auditActionSandboxRecycle      = "sandbox.sandbox.recycle"
	auditActionQuotaUpdate         = "sandbox.quota.update"
)

// writeAudit 写关键操作审计;缺少审计 writer 时显式失败,避免绕过 audit_log。
func (s *Service) writeAudit(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	if s.auditor == nil {
		return apperr.ErrSandboxAuditFail
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
