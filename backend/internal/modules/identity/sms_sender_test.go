// 短信发送器测试。
package identity

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHTTPSmsSenderPostsVerificationCode 确认 HTTP 网关收到脱离日志的真实发送请求。
func TestHTTPSmsSenderPostsVerificationCode(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization header = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	sender := &HTTPSmsSender{
		cfg: HTTPSmsConfig{
			Endpoint:       server.URL,
			Token:          "test-token",
			LoginTemplate:  "login-template",
			ResetTemplate:  "reset-template",
			ChangeTemplate: "change-template",
			Timeout:        time.Second,
		},
		client: server.Client(),
	}

	if err := sender.Send(context.Background(), "13800000000", "123456", SmsSceneLogin); err != nil {
		t.Fatalf("send sms: %v", err)
	}
	if got["phone"] != "13800000000" || got["code"] != "123456" || got["template"] != "login-template" {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

// TestHTTPSmsSenderRejectsGatewayFailure 确认网关失败会显式返回。
func TestHTTPSmsSenderRejectsGatewayFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	sender := &HTTPSmsSender{
		cfg: HTTPSmsConfig{
			Endpoint:       server.URL,
			Token:          "test-token",
			LoginTemplate:  "login-template",
			ResetTemplate:  "reset-template",
			ChangeTemplate: "change-template",
			Timeout:        time.Second,
		},
		client: server.Client(),
	}

	if err := sender.Send(context.Background(), "13800000000", "123456", SmsSceneLogin); err == nil {
		t.Fatalf("expected gateway failure")
	}
}

// TestHTTPSmsSenderReturnsBodyCloseFailure 确认网关响应体关闭失败不会被静默吞掉。
func TestHTTPSmsSenderReturnsBodyCloseFailure(t *testing.T) {
	closeFailure := errors.New("close failed")
	sender, err := NewHTTPSmsSender(HTTPSmsConfig{
		Endpoint:      "https://sms.example.test/send",
		LoginTemplate: "login-template",
		Timeout:       time.Second,
	})
	if err != nil {
		t.Fatalf("new sms sender: %v", err)
	}
	sender.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       closeFailBody{Reader: strings.NewReader(""), err: closeFailure},
		}, nil
	})}

	err = sender.Send(context.Background(), "13800000000", "123456", SmsSceneLogin)
	if !errors.Is(err, closeFailure) {
		t.Fatalf("expected close failure in error chain, got %v", err)
	}
}

// TestHTTPSmsSenderRejectsMissingTimeout 确认真实短信网关超时必须由配置显式注入。
func TestHTTPSmsSenderRejectsMissingTimeout(t *testing.T) {
	_, err := NewHTTPSmsSender(HTTPSmsConfig{
		Endpoint:      "https://sms.example.test/send",
		LoginTemplate: "login-template",
	})
	if err == nil {
		t.Fatalf("expected missing timeout to fail")
	}
}

// TestHTTPSmsSenderRejectsLocalEndpoint 确认生产构造器拒绝本机地址,避免短信配置形成 SSRF。
func TestHTTPSmsSenderRejectsLocalEndpoint(t *testing.T) {
	_, err := NewHTTPSmsSender(HTTPSmsConfig{
		Endpoint:      "http://127.0.0.1/send",
		LoginTemplate: "login-template",
		Timeout:       time.Second,
	})
	if err == nil {
		t.Fatalf("expected local endpoint to fail")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type closeFailBody struct {
	io.Reader
	err error
}

func (b closeFailBody) Close() error { return b.err }
