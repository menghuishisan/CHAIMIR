// M1 contracts 实现测试。
package identity

import (
	"context"
	"errors"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/tenant"
)

// TestCollectBatchAccountsReturnsErrorOnMissingAccount 确认批量跨模块查询不能静默丢弃失败项。
func TestCollectBatchAccountsReturnsErrorOnMissingAccount(t *testing.T) {
	expected := errors.New("account missing")
	_, err := collectBatchAccounts(context.Background(), []int64{1, 2}, func(_ context.Context, id int64) (contracts.AccountInfo, error) {
		if id == 2 {
			return contracts.AccountInfo{}, expected
		}
		return contracts.AccountInfo{AccountID: id}, nil
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected original lookup error, got %v", err)
	}
}

// TestAuditQueryScopeUsesPlatformPath 确认平台管理员审计查询使用平台级范围。
func TestAuditQueryScopeUsesPlatformPath(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), tenant.Identity{
		AccountID:  9001,
		IsPlatform: true,
	})

	scope, err := auditQueryScopeFromContext(ctx)
	if err != nil {
		t.Fatalf("platform audit scope: %v", err)
	}
	if !scope.platform {
		t.Fatalf("platform admin audit query must use platform-scoped path")
	}
}

// TestAuditQueryScopeRequiresTenantIdentity 确认审计查询必须有服务端鉴权身份。
func TestAuditQueryScopeRequiresTenantIdentity(t *testing.T) {
	if _, err := auditQueryScopeFromContext(context.Background()); err == nil {
		t.Fatalf("expected missing identity to fail")
	}
}
