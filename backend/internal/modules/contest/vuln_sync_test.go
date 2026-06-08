// M8 漏洞源同步测试:覆盖配置驱动拉取、字段映射、草稿生成与错误边界。
package contest

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"chaimir/pkg/apperr"
)

// roundTripFunc 让测试用函数模拟 HTTP 客户端。
type roundTripFunc func(*http.Request) (*http.Response, error)

// Do 执行测试 HTTP 请求。
func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

// sequenceIDGen 为测试按顺序返回多个雪花 ID。
type sequenceIDGen struct {
	ids []int64
	idx int
}

// Generate 返回下一个固定 ID。
func (g *sequenceIDGen) Generate() int64 {
	if g.idx >= len(g.ids) {
		return 0
	}
	id := g.ids[g.idx]
	g.idx++
	return id
}

// TestSyncVulnSourceImportsMappedCases 确认同步会按配置拉取外部案例并生成漏洞题草稿。
func TestSyncVulnSourceImportsMappedCases(t *testing.T) {
	store := &fakeContestStore{
		vulnSource: VulnSourceDTO{
			ID: "8501", TenantID: "100", Type: 1, Name: "source", DefaultLevel: VulnLevelB, Enabled: true,
			Config: map[string]any{
				"endpoint":        "https://vuln.example.edu/feed",
				"method":          "POST",
				"timeout_seconds": float64(5),
				"headers":         map[string]any{"Authorization": "Bearer test-token"},
				"body":            map[string]any{"scope": "evm"},
				"cases_path":      "items",
				"mapping": map[string]any{
					"external_ref": "id",
					"title":        "title",
					"level":        "level",
					"runtime_mode": "mode",
					"draft_body":   "body",
				},
			},
		},
	}
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", req.Method)
		}
		if req.URL.String() != "https://vuln.example.edu/feed" {
			t.Fatalf("unexpected endpoint: %s", req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization header not applied")
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if !strings.Contains(string(body), `"scope":"evm"`) {
			t.Fatalf("request body not encoded from config: %s", string(body))
		}
		return jsonResponse(http.StatusOK, `{
			"items": [
				{"id": "SWC-107", "title": "Reentrancy", "level": "A", "mode": "isolated", "body": {"summary": "attack path"}},
				{"id": "CVE-2026-0001", "title": "Oracle Drift", "mode": "forked", "body": {"chain": "mainnet"}}
			]
		}`), nil
	})
	svc := &Service{store: store, idgen: &sequenceIDGen{ids: []int64{9001, 9002}}, httpClient: client, vulnSourceMaxResponseBytes: 1 << 20, vulnSourceTimeoutSeconds: 10}

	out, err := svc.SyncVulnSource(testTenantContext(), 8501)
	if err != nil {
		t.Fatalf("sync rejected: %v", err)
	}
	if out.ImportedCount != 2 || len(out.Problems) != 2 {
		t.Fatalf("unexpected sync result: %#v", out)
	}
	if !store.sourceSynced {
		t.Fatalf("source last_sync_at should be marked after import")
	}
	first := store.createdVulnProblems[0]
	if first.ExternalRef != "SWC-107" || first.Level != VulnLevelA || first.RuntimeMode != VulnRuntimeIsolated {
		t.Fatalf("first case mapped incorrectly: %#v", first)
	}
	second := store.createdVulnProblems[1]
	if second.Level != VulnLevelB || second.RuntimeMode != VulnRuntimeForked {
		t.Fatalf("defaults and forked mode mapped incorrectly: %#v", second)
	}
}

// TestSyncVulnSourceRejectsDisabledSource 确认禁用漏洞源不会发起同步。
func TestSyncVulnSourceRejectsDisabledSource(t *testing.T) {
	store := &fakeContestStore{vulnSource: VulnSourceDTO{ID: "8501", Enabled: false, DefaultLevel: VulnLevelB, Config: validSyncConfig()}}
	svc := &Service{store: store, vulnSourceMaxResponseBytes: 1 << 20, vulnSourceTimeoutSeconds: 10, httpClient: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatalf("disabled source must not call remote endpoint")
		return nil, nil
	})}
	if _, err := svc.SyncVulnSource(testTenantContext(), 8501); !hasAppCode(err, apperr.ErrContestVulnSourceInvalid.Code) {
		t.Fatalf("expected vuln source invalid, got %v", err)
	}
}

// TestSyncVulnSourceRejectsInvalidConfig 确认缺少端点的配置会被拒绝。
func TestSyncVulnSourceRejectsInvalidConfig(t *testing.T) {
	store := &fakeContestStore{vulnSource: VulnSourceDTO{ID: "8501", Enabled: true, DefaultLevel: VulnLevelB, Config: map[string]any{"method": "GET"}}}
	svc := &Service{store: store}
	if _, err := svc.SyncVulnSource(testTenantContext(), 8501); !hasAppCode(err, apperr.ErrContestVulnSourceInvalid.Code) {
		t.Fatalf("expected vuln source invalid, got %v", err)
	}
}

// TestSyncVulnSourceRejectsMissingResponseLimit 确认 M8 装配必须显式提供漏洞源响应边界。
func TestSyncVulnSourceRejectsMissingResponseLimit(t *testing.T) {
	store := &fakeContestStore{vulnSource: VulnSourceDTO{ID: "8501", Enabled: true, DefaultLevel: VulnLevelB, Config: validSyncConfig()}}
	svc := &Service{store: store, vulnSourceTimeoutSeconds: 10, httpClient: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `[]`), nil
	})}
	if _, err := svc.SyncVulnSource(testTenantContext(), 8501); !hasAppCode(err, apperr.ErrContestVulnSourceInvalid.Code) {
		t.Fatalf("expected vuln source invalid, got %v", err)
	}
}

// TestSyncVulnSourceUsesConfiguredDefaultTimeout 确认缺省漏洞源超时来自启动配置而非代码常量。
func TestSyncVulnSourceUsesConfiguredDefaultTimeout(t *testing.T) {
	cfg := validSyncConfig()
	delete(cfg, "timeout_seconds")
	store := &fakeContestStore{vulnSource: VulnSourceDTO{ID: "8501", Enabled: true, DefaultLevel: VulnLevelB, Config: cfg}}
	svc := &Service{
		store:                      store,
		vulnSourceTimeoutSeconds:   7,
		vulnSourceMaxResponseBytes: 1 << 20,
		httpClient: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatalf("vuln source request must carry a timeout deadline")
			}
			remaining := time.Until(deadline)
			if remaining <= 6*time.Second || remaining > 7*time.Second {
				t.Fatalf("expected configured timeout near 7s, got %s", remaining)
			}
			return jsonResponse(http.StatusOK, `[]`), nil
		}),
	}
	if _, err := svc.SyncVulnSource(testTenantContext(), 8501); err != nil {
		t.Fatalf("sync rejected: %v", err)
	}
}

// TestSyncVulnSourceRejectsRemoteFailure 确认外部源非 2xx 响应不会生成草稿。
func TestSyncVulnSourceRejectsRemoteFailure(t *testing.T) {
	store := &fakeContestStore{vulnSource: VulnSourceDTO{ID: "8501", Enabled: true, DefaultLevel: VulnLevelB, Config: validSyncConfig()}}
	svc := &Service{store: store, vulnSourceMaxResponseBytes: 1 << 20, vulnSourceTimeoutSeconds: 10, httpClient: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadGateway, `{"message":"bad gateway"}`), nil
	})}
	if _, err := svc.SyncVulnSource(testTenantContext(), 8501); !hasAppCode(err, apperr.ErrContestVulnSourceInvalid.Code) {
		t.Fatalf("expected vuln source invalid, got %v", err)
	}
	if len(store.createdVulnProblems) != 0 {
		t.Fatalf("remote failure must not import problems")
	}
}

// TestSyncVulnSourceRejectsOversizedRemoteResponse 确认漏洞源响应体超过 M8 配置边界时直接拒绝。
func TestSyncVulnSourceRejectsOversizedRemoteResponse(t *testing.T) {
	store := &fakeContestStore{vulnSource: VulnSourceDTO{ID: "8501", Enabled: true, DefaultLevel: VulnLevelB, Config: validSyncConfig()}}
	svc := &Service{
		store:                      store,
		vulnSourceMaxResponseBytes: 32,
		vulnSourceTimeoutSeconds:   10,
		httpClient: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, strings.Repeat(" ", 33)), nil
		}),
	}
	if _, err := svc.SyncVulnSource(testTenantContext(), 8501); !hasAppCode(err, apperr.ErrContestVulnSourceTooLarge.Code) {
		t.Fatalf("expected vuln source response too large, got %v", err)
	}
	if len(store.createdVulnProblems) != 0 {
		t.Fatalf("oversized response must not import problems")
	}
}

// validSyncConfig 返回可用的最小同步配置。
func validSyncConfig() map[string]any {
	return map[string]any{
		"endpoint": "https://vuln.example.edu/feed",
		"method":   "GET",
		"mapping":  map[string]any{"external_ref": "id", "title": "title", "draft_body": "body"},
	}
}

// jsonResponse 构造测试 HTTP JSON 响应。
func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

// hasAppCode 判断错误是否为指定应用错误码。
func hasAppCode(err error, code string) bool {
	appErr, ok := apperr.As(err)
	return ok && appErr.Code == code
}
