// storage_test 用聚焦测试守住统一文件服务唯一入口,避免模块重新拼装第二套上传下载链路。
package storage

import (
	"strings"
	"testing"
	"time"

	"chaimir/internal/platform/upload"
)

// TestServicePlanUploadBuildsTenantScopedObjectRef 确认统一文件服务会把上传请求归一为租户作用域对象引用。
func TestServicePlanUploadBuildsTenantScopedObjectRef(t *testing.T) {
	service := Service{
		DownloadGrantTTL: 5 * time.Minute,
	}
	plan, err := service.PlanUpload(PlanUploadRequest{
		TenantID:        42,
		AccountID:       1001,
		Module:          "grade",
		ResourceType:    "transcript",
		ResourceID:      "2026-final",
		FileName:        "report.pdf",
		ContentType:     "application/pdf",
		Size:            128,
		MaxBytes:        1024,
		ExpectedBucket:  "chaimir-report",
		AllowedFileName: true,
	})
	if err != nil {
		t.Fatalf("plan upload: %v", err)
	}
	if plan.ObjectRef != "minio://chaimir-report/42/grade/transcript/2026-final/report.pdf" {
		t.Fatalf("object ref = %q", plan.ObjectRef)
	}
	if plan.DownloadExpiresAt.IsZero() {
		t.Fatalf("download expiry should be populated")
	}
}

// TestServicePlanUploadRejectsUnsafeFileName 确认统一文件服务会拒绝不安全文件名而不是把路径规则分散给模块处理。
func TestServicePlanUploadRejectsUnsafeFileName(t *testing.T) {
	service := Service{
		DownloadGrantTTL: 5 * time.Minute,
	}
	_, err := service.PlanUpload(PlanUploadRequest{
		TenantID:        42,
		AccountID:       1001,
		Module:          "grade",
		ResourceType:    "transcript",
		ResourceID:      "2026-final",
		FileName:        "../report.pdf",
		ContentType:     "application/pdf",
		Size:            128,
		MaxBytes:        1024,
		ExpectedBucket:  "chaimir-report",
		AllowedFileName: true,
	})
	if err == nil {
		t.Fatalf("unsafe file name must fail")
	}
}

// TestServicePlanUploadRunsSharedValidators 确认统一文件服务会收敛 upload 原语层的大小、类型和病毒扫描校验。
func TestServicePlanUploadRunsSharedValidators(t *testing.T) {
	service := Service{
		DownloadGrantTTL: 5 * time.Minute,
		Scanner:          stubScanner{result: upload.ScanResult{Verdict: upload.VerdictClean}},
	}
	_, err := service.PlanUpload(PlanUploadRequest{
		TenantID:        42,
		AccountID:       1001,
		Module:          "identity",
		ResourceType:    "import",
		ResourceID:      "job-1",
		FileName:        "accounts.csv",
		ContentType:     "text/csv",
		Size:            12,
		MaxBytes:        1024,
		ExpectedBucket:  "chaimir-attachment",
		AllowedFileName: true,
		Content:         []byte("id,name\n1,a"),
		KindValidator: func(fileName, contentType string, content []byte) bool {
			return upload.CSVOrXLSXKind(fileName, contentType, content) == upload.KindCSV
		},
		ScanPolicy: upload.ScanPolicy{Required: true},
	})
	if err != nil {
		t.Fatalf("shared validation should pass: %v", err)
	}
}

// TestServiceIssueDownloadGrantUsesPlannedObjectRef 确认统一文件服务会从统一上传计划继续签发下载授权。
func TestServiceIssueDownloadGrantUsesPlannedObjectRef(t *testing.T) {
	service := Service{
		DownloadGrantTTL: 5 * time.Minute,
		SigningKey:       strings.Repeat("k", 32),
	}
	plan, err := service.PlanUpload(PlanUploadRequest{
		TenantID:        42,
		AccountID:       1001,
		Module:          "grade",
		ResourceType:    "transcript",
		ResourceID:      "2026-final",
		FileName:        "report.pdf",
		ContentType:     "application/pdf",
		Size:            128,
		MaxBytes:        1024,
		ExpectedBucket:  "chaimir-report",
		AllowedFileName: true,
	})
	if err != nil {
		t.Fatalf("plan upload: %v", err)
	}
	token, grant, err := service.IssueDownloadGrant(IssueDownloadGrantRequest{
		TenantID:     42,
		AccountID:    1001,
		ObjectRef:    plan.ObjectRef,
		Module:       "grade",
		ResourceType: "transcript",
		ResourceID:   "2026-final",
	})
	if err != nil {
		t.Fatalf("issue download grant: %v", err)
	}
	if strings.TrimSpace(token) == "" {
		t.Fatalf("token should not be empty")
	}
	if grant.Object.Key != "42/grade/transcript/2026-final/report.pdf" {
		t.Fatalf("grant key = %q", grant.Object.Key)
	}
}

type stubScanner struct {
	result upload.ScanResult
	err    error
}

// Scan 返回预设结果,供统一文件服务测试验证共享扫描边界。
func (s stubScanner) Scan(req upload.ScanRequest) (upload.ScanResult, error) {
	return s.result, s.err
}
