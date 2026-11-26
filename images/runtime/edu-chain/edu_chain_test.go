// 本文件覆盖教学链原生适配器的 HTTP 边界,确保平台动作不会绕过节点 API。
package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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

type responseTransport func(*http.Request) (*http.Response, error)

// RoundTrip 在内存中返回节点响应，不监听端口或访问网络。
func (f responseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestAdapterJSONRequestRequiresJSONObject 验证非对象响应不能成为成功查询结果。
func TestAdapterJSONRequestRequiresJSONObject(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, body := range []string{`["not-an-object"]`, "null", "{}{}"} {
		t.Run(body, func(t *testing.T) {
			http.DefaultTransport = responseTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})
			if _, err := adapterJSONRequest(http.MethodGet, "http://fixture.invalid", nil, time.Second); err == nil {
				t.Fatal("非对象响应未被拒绝")
			}
		})
	}
}
