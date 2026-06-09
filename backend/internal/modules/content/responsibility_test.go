// M5 职责测试:用源码级守护防止 HTTP 层承载业务编排或审计绕过模块错误码。
package content

import (
	"os"
	"strings"
	"testing"
)

// TestBatchGetFaceDelegatesToService 确认内部批量取题面只在 service 层编排,API 层只绑定请求并写响应。
func TestBatchGetFaceDelegatesToService(t *testing.T) {
	src := mustReadContentSource(t, "api.go")
	body := extractContentFunction(t, src, "batchGetFace")
	if strings.Contains(body, "for _, ref := range req.Items") || strings.Contains(body, "svc.GetFace") {
		t.Fatalf("batchGetFace must delegate batch orchestration to service, got:\n%s", body)
	}
	if !strings.Contains(body, "svc.BatchGetFace") {
		t.Fatalf("batchGetFace should call Service.BatchGetFace, got:\n%s", body)
	}
}

// TestAuditUsesContentErrorCodes 确认 audit.go 不使用通用内部错误或裸返回底层审计错误。
func TestAuditUsesContentErrorCodes(t *testing.T) {
	src := mustReadContentSource(t, "audit.go")
	if strings.Contains(src, "apperr.ErrInternal") {
		t.Fatalf("content audit must use ErrContentAuditFailed instead of ErrInternal")
	}
	if !strings.Contains(src, "ErrContentAuditFailed") {
		t.Fatalf("content audit should map failures to ErrContentAuditFailed")
	}
}

// TestOwnAndInternalReadsDistinguishNotFoundFromQueryFailure 确认内容读取区分 no rows 与 DB 查询故障。
func TestOwnAndInternalReadsDistinguishNotFoundFromQueryFailure(t *testing.T) {
	serviceSrc := mustReadContentSource(t, "service.go")
	contractsSrc := mustReadContentSource(t, "service_contract.go")
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "getOwnContent", body: extractContentFunction(t, serviceSrc, "getOwnContent")},
		{name: "getSharedContent", body: extractContentFunction(t, serviceSrc, "getSharedContent")},
		{name: "getContentInTenant", body: extractContentFunction(t, contractsSrc, "getContentInTenant")},
	} {
		if !strings.Contains(tc.body, "db.IsNoRows") {
			t.Fatalf("%s must distinguish no rows from query failure, got:\n%s", tc.name, tc.body)
		}
		if !strings.Contains(tc.body, "ErrContentReadFailed") && !strings.Contains(tc.body, "ErrContentShareReadFailed") {
			t.Fatalf("%s must use precise read failure code for non-no-rows errors, got:\n%s", tc.name, tc.body)
		}
	}
}

func mustReadContentSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func extractContentFunction(t *testing.T, src, name string) string {
	t.Helper()
	start := strings.Index(src, "func ")
	for start >= 0 {
		remaining := src[start:]
		if strings.HasPrefix(remaining, "func "+name+"(") || strings.Contains(remaining[:minContentLen(len(remaining), 120)], ") "+name+"(") {
			brace := strings.Index(remaining, "{")
			if brace < 0 {
				t.Fatalf("function %s has no body", name)
			}
			depth := 0
			for i, r := range remaining[brace:] {
				switch r {
				case '{':
					depth++
				case '}':
					depth--
					if depth == 0 {
						return remaining[:brace+i+1]
					}
				}
			}
			t.Fatalf("function %s body not closed", name)
		}
		next := strings.Index(remaining[5:], "func ")
		if next < 0 {
			break
		}
		start += 5 + next
	}
	t.Fatalf("function %s not found", name)
	return ""
}

func minContentLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}
