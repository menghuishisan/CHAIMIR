// M9 审计测试:确保管理后台高权限操作在缺少审计 writer 时不会静默成功。
package admin

import (
	"context"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestWriteAuditRequiresConfiguredWriter 确认 M9 缺少审计 writer 时显式失败。
func TestWriteAuditRequiresConfiguredWriter(t *testing.T) {
	svc := &Service{}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	err := svc.writeAudit(ctx, 1001, 2001, "admin.config.update", "system_config", 3001, map[string]any{"key": "quota.warn"})
	if err == nil {
		t.Fatalf("expected missing auditor to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrAdminAuditWriteFailed.Code {
		t.Fatalf("expected admin audit write error, got %v", err)
	}
}

// TestAuditCSVRowFormatsCreatedAtAsUTC 确认 CSV 导出边界统一输出 UTC/RFC3339。
func TestAuditCSVRowFormatsCreatedAtAsUTC(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	row := contracts.AuditRecord{
		ID:         1,
		TenantID:   1001,
		ActorID:    2001,
		Action:     "admin.audit.export",
		TargetType: "audit_log",
		TargetID:   3001,
		TraceID:    "trace-admin-audit",
		CreatedAt:  time.Date(2026, 6, 6, 18, 30, 0, 0, shanghai),
	}

	fields := auditCSVRow(row)
	if got, want := fields[7], "2026-06-06T10:30:00Z"; got != want {
		t.Fatalf("expected created_at exported as UTC, got %q want %q", got, want)
	}
}
