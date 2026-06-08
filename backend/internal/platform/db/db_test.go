// Package db 测试 PostgreSQL 平台辅助能力的统一边界。
package db

import (
	"fmt"
	"testing"

	"chaimir/internal/platform/config"

	"github.com/jackc/pgx/v5"
)

// TestIsNoRowsRecognizesWrappedPGXError 确认模块能统一识别直接或包装后的 pgx 未命中错误。
func TestIsNoRowsRecognizesWrappedPGXError(t *testing.T) {
	if !IsNoRows(pgx.ErrNoRows) {
		t.Fatalf("expected direct pgx.ErrNoRows to be recognized")
	}
	if !IsNoRows(fmt.Errorf("load course: %w", pgx.ErrNoRows)) {
		t.Fatalf("expected wrapped pgx.ErrNoRows to be recognized")
	}
}

// TestPoolConfigPreservesSpecialCharacters 确认数据库凭据通过结构化字段传递,不会因空格或符号破坏 DSN。
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
