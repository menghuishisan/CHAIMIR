// identity audit 文件实现 platform/audit.Writer,统一写入 identity.audit_log。
package identity

import (
	"context"
	"fmt"

	platformaudit "chaimir/internal/platform/audit"
	"chaimir/pkg/snowflake"
)

// AuditWriter 把平台审计条目写入 identity 模块共享 audit_log 表。
type AuditWriter struct {
	store Store
	ids   snowflake.Generator
}

// Write 写入一条审计记录,所有模块都应经 platform/audit.Writer 调用该实现。
func (w *AuditWriter) Write(ctx context.Context, e platformaudit.Entry) error {
	if w == nil || w.store == nil || w.ids == nil {
		return fmt.Errorf("identity audit writer 未初始化")
	}
	write := func(ctx context.Context, tx TxStore) error {
		return tx.WriteAudit(ctx, WriteAuditInput{
			ID:         w.ids.Generate(),
			TenantID:   e.TenantID,
			ActorID:    e.ActorID,
			ActorRole:  e.ActorRole,
			Action:     e.Action,
			TargetType: e.TargetType,
			TargetID:   e.TargetID,
			Detail:     []byte(e.Detail),
			IP:         e.IP,
			TraceID:    e.TraceID,
		})
	}
	if e.TenantID > 0 {
		return w.store.TenantTx(ctx, e.TenantID, write)
	}
	return w.store.PlatformTx(ctx, write)
}
