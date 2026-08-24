// 本文件覆盖教学链原生适配器的 HTTP 边界,确保平台动作不会绕过节点 API。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAdapterQueryURLRestrictsSpecialTargets 验证查询目标只映射到教学链已声明的端点。
func TestAdapterQueryURLRestrictsSpecialTargets(t *testing.T) {
	chainURL, err := adapterQueryURL("http://runtime:8080", "chain")
	if err != nil || chainURL != "http://runtime:8080/chain" {
		t.Fatalf("chain 查询地址错误: %q, %v", chainURL, err)
	}
	objectURL, err := adapterQueryURL("http://runtime:8080", "object:abc")
	if err != nil || !strings.Contains(objectURL, "/query?target=object%3Aabc") {
		t.Fatalf("对象查询地址错误: %q, %v", objectURL, err)
	}
}

// TestAdapterJSONRequestRequiresJSONObject 验证适配器拒绝非对象响应,避免把错误页面当成链结果。
func TestAdapterJSONRequestRequiresJSONObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]string{"not-an-object"})
	}))
	defer server.Close()
	if _, err := adapterJSONRequest(http.MethodGet, server.URL, nil); err == nil {
		t.Fatal("非对象响应未被拒绝")
	}
}
