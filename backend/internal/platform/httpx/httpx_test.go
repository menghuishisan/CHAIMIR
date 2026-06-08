// Package httpx 测试 HTTP 层通用参数解析与响应 helper。
package httpx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// TestPathIDRejectsInvalidParam 确认路径 ID 解析失败时写统一错误响应。
func TestPathIDRejectsInvalidParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/test/bad", nil)
	c.Params = gin.Params{{Key: "id", Value: "bad"}}

	id, ok := PathID(c, "id")
	if ok || id != 0 {
		t.Fatalf("expected invalid path id, got id=%d ok=%v", id, ok)
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"11010"`) {
		t.Fatalf("expected unified bad request, status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// TestHTTPXDoesNotOwnIDParsing 防止 HTTP helper 包保留第二套雪花 ID 解析入口。
func TestHTTPXDoesNotOwnIDParsing(t *testing.T) {
	data, err := os.ReadFile("httpx.go")
	if err != nil {
		t.Fatalf("read httpx.go: %v", err)
	}
	if strings.Contains(string(data), "func ParseID(") {
		t.Fatalf("httpx must use platform/ids for ID parsing instead of defining ParseID")
	}
}

// TestWritePageUsesUnifiedEnvelope 确认分页响应走统一响应结构。
func TestWritePageUsesUnifiedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	WritePage(c, []string{"a"}, 1, 1, 20, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("expected page response, status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// TestBindJSONWithErrorUsesModuleError 确认模块可复用统一 JSON 绑定并保留本模块错误码。
func TestBindJSONWithErrorUsesModuleError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("{"))
	c.Request.Header.Set("Content-Type", "application/json")

	var req struct {
		Name string `json:"name"`
	}
	if BindJSONWithError(c, &req, apperr.ErrContentRequestInvalid) {
		t.Fatalf("expected malformed JSON to fail")
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"51007"`) {
		t.Fatalf("expected module error response, status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "unexpected EOF") {
		t.Fatalf("response body must not expose JSON parser internals: %s", rec.Body.String())
	}
}

// TestBindJSONWithErrorKeepsCause 确认 JSON 绑定错误仍写入内部错误链而不进响应体。
func TestBindJSONWithErrorKeepsCause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("{"))
	c.Request.Header.Set("Content-Type", "application/json")

	var req struct {
		Name string `json:"name"`
	}
	if BindJSONWithError(c, &req, apperr.ErrRequestBodyInvalid) {
		t.Fatalf("expected malformed JSON to fail")
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"11011"`) {
		t.Fatalf("expected request body error response, status=%d body=%s", rec.Code, body)
	}
	if strings.Contains(body, "unexpected EOF") {
		t.Fatalf("response body must not expose JSON parser internals: %s", body)
	}
}
