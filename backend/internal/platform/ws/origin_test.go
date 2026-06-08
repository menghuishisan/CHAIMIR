// Package ws 测试 WebSocket Origin 白名单策略。
package ws

import (
	"net/http"
	"testing"
)

// TestOriginPolicyRejectsUnlistedOrigin 确认 WebSocket 不再默认放行任意跨站 Origin。
func TestOriginPolicyRejectsUnlistedOrigin(t *testing.T) {
	policy := NewOriginPolicy([]string{"https://chaimir.example.edu"})
	req, err := http.NewRequest(http.MethodGet, "https://api.example.edu/ws", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example.net")

	if policy.Check(req) {
		t.Fatalf("unexpectedly allowed unlisted origin")
	}
}

// TestOriginPolicyAllowsSameHostAndConfiguredOrigin 确认同源与配置白名单可建立 WS。
func TestOriginPolicyAllowsSameHostAndConfiguredOrigin(t *testing.T) {
	policy := NewOriginPolicy([]string{"https://chaimir.example.edu"})
	sameHost, err := http.NewRequest(http.MethodGet, "https://api.example.edu/ws", nil)
	if err != nil {
		t.Fatalf("create same host request: %v", err)
	}
	sameHost.Header.Set("Origin", "https://api.example.edu")
	if !policy.Check(sameHost) {
		t.Fatalf("same host origin should be allowed")
	}

	listed, err := http.NewRequest(http.MethodGet, "https://api.example.edu/ws", nil)
	if err != nil {
		t.Fatalf("create listed origin request: %v", err)
	}
	listed.Header.Set("Origin", "https://chaimir.example.edu")
	if !policy.Check(listed) {
		t.Fatalf("configured origin should be allowed")
	}
}

// TestOriginPolicyRejectsMalformedOrigin 确认非空但格式非法的 Origin 不会被当作无 Origin 放行。
func TestOriginPolicyRejectsMalformedOrigin(t *testing.T) {
	policy := NewOriginPolicy([]string{"https://chaimir.example.edu"})
	req, err := http.NewRequest(http.MethodGet, "https://api.example.edu/ws", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Origin", "://bad-origin")
	if policy.Check(req) {
		t.Fatalf("malformed origin must be rejected")
	}
}
