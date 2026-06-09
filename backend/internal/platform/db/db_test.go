// db_test 校验 PostgreSQL 平台基础设施的统一事务和凭据边界。
package db

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"chaimir/internal/platform/config"

	"github.com/jackc/pgx/v5"
)

// TestIsNoRowsRecognizesWrappedPGXError 确认包装后的 pgx 未命中错误仍可统一识别。
func TestIsNoRowsRecognizesWrappedPGXError(t *testing.T) {
	if !IsNoRows(pgx.ErrNoRows) {
		t.Fatalf("expected direct pgx.ErrNoRows to be recognized")
	}
	if !IsNoRows(fmt.Errorf("load course: %w", pgx.ErrNoRows)) {
		t.Fatalf("expected wrapped pgx.ErrNoRows to be recognized")
	}
}

// TestPrivilegedModuleTxDocumentsMaintenanceOnlyBoundary 确认特权事务被限制在模块维护路径。
func TestPrivilegedModuleTxDocumentsMaintenanceOnlyBoundary(t *testing.T) {
	src, err := os.ReadFile("db.go")
	if err != nil {
		t.Fatalf("read db.go: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, "WithPrivilegedModuleTx") ||
		!strings.Contains(body, "仅限模块后台维护任务扫描本模块自有 RLS 表") {
		t.Fatalf("db must expose and document a narrow privileged module maintenance transaction")
	}
}

// TestPoolConfigPreservesSpecialCharacters 确认凭据通过结构化配置传递,不会破坏特殊字符。
func TestPoolConfigPreservesSpecialCharacters(t *testing.T) {
	poolCfg, err := buildPoolConfig(config.PostgresConfig{
		Host:     "postgres.local",
		Port:     5432,
		Database: "chaimir",
		SSLMode:  "require",
		MaxConns: 20,
		MinConns: 2,
	}, "app user", "p@ss word=with'quote")
	if err != nil {
		t.Fatalf("build pool config: %v", err)
	}
	if poolCfg.ConnConfig.User != "app user" || poolCfg.ConnConfig.Password != "p@ss word=with'quote" {
		t.Fatalf("credentials were not preserved: user=%q password=%q", poolCfg.ConnConfig.User, poolCfg.ConnConfig.Password)
	}
	if poolCfg.ConnConfig.TLSConfig == nil {
		t.Fatalf("sslmode=require should enable TLS config")
	}
}
