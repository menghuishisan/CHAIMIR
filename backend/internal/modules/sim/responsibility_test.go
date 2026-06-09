// M4 职责边界测试:守护 sim 模块文件职责,避免服务、转换、规则和 WS 代码再次混杂。
package sim

import (
	"os"
	"strings"
	"testing"
)

// TestSimShareMigrationKeepsPublicShareIndexOutsideRLS 确认公开分享码索引不进入 tenant RLS loop。
func TestSimShareMigrationKeepsPublicShareIndexOutsideRLS(t *testing.T) {
	data, err := os.ReadFile("../../../db/migrations/0005_sim.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "tenant_tables TEXT[] := ARRAY[")
	end := strings.Index(body[start:], "];")
	if start < 0 || end < 0 {
		t.Fatalf("tenant_tables RLS block not found")
	}
	block := body[start : start+end]
	if strings.Contains(block, "'sim_share'") {
		t.Fatalf("sim_share is a public share-code index and must not be in tenant RLS loop")
	}
}

// TestSimWebSocketFileDoesNotOwnDBLoading 确认 api_websocket.go 只处理 WS 接入与事件流。
func TestSimWebSocketFileDoesNotOwnDBLoading(t *testing.T) {
	data, err := os.ReadFile("api_websocket.go")
	if err != nil {
		t.Fatalf("read api_websocket.go: %v", err)
	}
	body := string(data)
	for _, forbidden := range []string{`internal/sqlcgen`, `platform/db`, "func (s *Service) loadBackendSession("} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("api_websocket.go must not own DB/session loading responsibility, found %s", forbidden)
		}
	}
}

// TestSimServiceFileDoesNotOwnRowConversions 确认 service.go 不承载 DTO/row 转换职责。
func TestSimServiceFileDoesNotOwnRowConversions(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	for _, forbidden := range []string{
		"func packageToDTO(",
		"func reviewToDTO(",
		"func sessionToDTO(",
		"func actionToDTO(",
		"func replayFromRows(",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("service.go must not own row/DTO conversion, found %s", forbidden)
		}
	}
}

// TestSimServiceFileDoesNotOwnValidationReportRules 确认审核报告门禁规则留在 validation.go。
func TestSimServiceFileDoesNotOwnValidationReportRules(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	for _, forbidden := range []string{
		"func mergeValidationReport(",
		"func previewReportPassed(",
		"var protectedValidationReportKeys",
		"var dynamicValidationReportKeys",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("service.go must not own validation-report rules, found %s", forbidden)
		}
	}
}

// TestSimServiceDoesNotWrapRepositoryFailuresAsDomainAbsence 确认底层故障不会误报成不存在或分享码失效。
func TestSimServiceDoesNotWrapRepositoryFailuresAsDomainAbsence(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	for _, forbidden := range []string{
		"ErrSimPackageNotFound.WithCause(err)",
		"ErrSimPackageUnavailable.WithCause(err)",
		"ErrSimReviewNotFound.WithCause(err)",
		"ErrSimSessionNotFound.WithCause(err)",
		"ErrSimShareInvalid.WithCause(err)",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("repository failure must not be wrapped as domain absence: %s", forbidden)
		}
	}
}

// TestSimPackageLifecycleUsesStateGuard 确认包下架/重新上架必须带前置状态条件。
func TestSimPackageLifecycleUsesStateGuard(t *testing.T) {
	sqlData, err := os.ReadFile("../../../db/queries/sim.sql")
	if err != nil {
		t.Fatalf("read sim sql: %v", err)
	}
	sql := string(sqlData)
	if !strings.Contains(sql, "WHERE id = $1 AND status = $2") {
		t.Fatalf("package lifecycle transition must guard the current status")
	}

	serviceData, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	service := string(serviceData)
	for _, required := range []string{
		"transitionPackageStatus(ctx, packageID, PackageStatusPublished, PackageStatusArchived)",
		"transitionPackageStatus(ctx, packageID, PackageStatusArchived, PackageStatusPublished)",
	} {
		if !strings.Contains(service, required) {
			t.Fatalf("package lifecycle must use guarded transition: missing %s", required)
		}
	}
}
