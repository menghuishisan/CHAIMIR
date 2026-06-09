// identity service_archive_test 文件守护按学年归档账号与班级的文档口径。
package identity

import (
	"os"
	"strings"
	"testing"
)

// TestBatchArchiveRouteUsesEnrollmentYearRequest 验证批量归档接口按文档绑定入学年份而不是账号 ID 列表。
func TestBatchArchiveRouteUsesEnrollmentYearRequest(t *testing.T) {
	raw, err := os.ReadFile("api_account.go")
	if err != nil {
		t.Fatalf("读取账号 API 文件失败: %v", err)
	}
	source := string(raw)
	start := strings.Index(source, "func (a accountAPI) batchArchive")
	if start < 0 {
		t.Fatalf("未找到 batchArchive handler")
	}
	end := strings.Index(source[start:], "func (a accountAPI) batchRestore")
	if end < 0 {
		t.Fatalf("未找到 batchArchive handler 边界")
	}
	body := source[start : start+end]

	if !strings.Contains(body, "ArchiveClassesRequest") || !strings.Contains(body, "httpx.BindJSON") {
		t.Fatalf("批量归档应绑定 enrollment_year 请求体,当前未使用 ArchiveClassesRequest")
	}
	if strings.Contains(body, "batchStatus") || strings.Contains(body, "BatchAccountIDsRequest") {
		t.Fatalf("批量归档不应复用账号 ID 列表状态流转路径")
	}
}

// TestArchiveClassesByAdminAlsoArchivesStudentAccounts 验证班级归档流程同步归档该学年正常学生账号。
func TestArchiveClassesByAdminAlsoArchivesStudentAccounts(t *testing.T) {
	raw, err := os.ReadFile("service_org.go")
	if err != nil {
		t.Fatalf("读取组织 service 文件失败: %v", err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Service) ArchiveClassesByAdmin")
	if start < 0 {
		t.Fatalf("未找到 ArchiveClassesByAdmin")
	}
	end := strings.Index(source[start:], "func (s *Service) parseOrgImportFile")
	if end < 0 {
		t.Fatalf("未找到 ArchiveClassesByAdmin 函数边界")
	}
	body := source[start : start+end]

	if !strings.Contains(body, "ArchiveStudentAccountsByEnrollmentYear") {
		t.Fatalf("班级归档必须同步归档同入学年份的正常学生账号")
	}
	if !strings.Contains(body, "TraceID:") {
		t.Fatalf("班级归档审计必须透传 trace_id")
	}
}

// TestIdentitySQLDefinesStudentArchiveByEnrollmentYear 验证 repo 只能通过 sqlc 查询归档学生账号。
func TestIdentitySQLDefinesStudentArchiveByEnrollmentYear(t *testing.T) {
	raw, err := os.ReadFile("../../../db/queries/identity.sql")
	if err != nil {
		t.Fatalf("读取 identity SQL 文件失败: %v", err)
	}
	source := string(raw)
	if !strings.Contains(source, "-- name: ArchiveStudentAccountsByEnrollmentYear :exec") {
		t.Fatalf("缺少按入学年份归档学生账号的 sqlc 查询")
	}
	if !strings.Contains(source, "base_identity = 1") || !strings.Contains(source, "status = 2") || !strings.Contains(source, "status = 4") {
		t.Fatalf("学生账号归档 SQL 必须限定学生、正常状态并更新为归档状态")
	}
}

// TestAuditWriterPersistsTraceID 验证统一审计写入器不会丢失请求链路编号。
func TestAuditWriterPersistsTraceID(t *testing.T) {
	raw, err := os.ReadFile("audit.go")
	if err != nil {
		t.Fatalf("读取审计写入器失败: %v", err)
	}
	if !strings.Contains(string(raw), "TraceID:    e.TraceID") {
		t.Fatalf("platform/audit.Entry 的 trace_id 必须写入 audit_log")
	}
}
