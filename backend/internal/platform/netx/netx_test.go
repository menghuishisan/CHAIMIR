// Package netx 测试出站网络安全边界。
package netx

import "testing"

// TestValidatePublicHTTPURLRejectsSSRFTargets 确认外部 HTTP 配置不能指向本机、私网或非 HTTP 协议。
func TestValidatePublicHTTPURLRejectsSSRFTargets(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:8080/a",
		"http://localhost/a",
		"http://10.0.0.1/a",
		"http://172.16.0.1/a",
		"http://192.168.1.1/a",
		"http://169.254.169.254/latest/meta-data",
		"https://user:pass@example.com/api",
		"file:///etc/passwd",
		"http://[::1]/a",
	} {
		if _, err := ValidatePublicHTTPURL(raw); err == nil {
			t.Fatalf("unsafe outbound URL should fail: %s", raw)
		}
	}
}

// TestValidatePublicHTTPURLAcceptsDomainEndpoint 确认正常公网域名端点可以通过并保留规范化 URL。
func TestValidatePublicHTTPURLAcceptsDomainEndpoint(t *testing.T) {
	got, err := ValidatePublicHTTPURL("https://example.com/api")
	if err != nil {
		t.Fatalf("public URL should pass: %v", err)
	}
	if got != "https://example.com/api" {
		t.Fatalf("unexpected normalized URL: %s", got)
	}
}

// TestValidatePublicLDAPSURLRequiresLDAPS 确认目录服务外联只接受公网 LDAPS 端点。
func TestValidatePublicLDAPSURLRequiresLDAPS(t *testing.T) {
	if _, err := ValidatePublicLDAPSURL("ldap://ldap.example.edu:389"); err == nil {
		t.Fatalf("plain LDAP endpoint should fail")
	}
	if _, err := ValidatePublicLDAPSURL("ldaps://10.0.0.10:636"); err == nil {
		t.Fatalf("private LDAPS endpoint should fail")
	}
	if _, err := ValidatePublicLDAPSURL("ldaps://ldap.example.edu:636"); err != nil {
		t.Fatalf("public LDAPS endpoint should pass: %v", err)
	}
}

// TestPublicResolvedURLUsesDefaultPort 确认非 HTTP 协议未显式端口时可由调用方提供协议默认端口。
func TestPublicResolvedURLUsesDefaultPort(t *testing.T) {
	resolved, serverName, err := PublicResolvedURL(t.Context(), "ldaps://8.8.8.8", "636")
	if err != nil {
		t.Fatalf("resolved URL with default port should pass: %v", err)
	}
	if resolved != "ldaps://8.8.8.8:636" || serverName != "8.8.8.8" {
		t.Fatalf("unexpected resolved URL or server name: %s %s", resolved, serverName)
	}
}

// TestPublicHTTPTransportDisablesEnvironmentProxy 确认公网出站 client 不经代理绕过目标地址校验。
func TestPublicHTTPTransportDisablesEnvironmentProxy(t *testing.T) {
	transport := PublicHTTPTransport(nil)
	if transport.Proxy != nil {
		t.Fatalf("public HTTP transport must not use environment proxy")
	}
}

// TestPublicDialAddressRejectsPrivateLiteral 确认请求拨号前会再次拒绝内网 IP。
func TestPublicDialAddressRejectsPrivateLiteral(t *testing.T) {
	if _, err := publicDialAddress(t.Context(), "10.0.0.1", "80"); err == nil {
		t.Fatalf("private dial address should fail")
	}
}

// TestPublicResolvedURLRejectsPrivateLiteral 确认非 HTTP 协议拨号前也会把私网地址挡住。
func TestPublicResolvedURLRejectsPrivateLiteral(t *testing.T) {
	if _, _, err := PublicResolvedURL(t.Context(), "ldaps://10.0.0.10:636", "636"); err == nil {
		t.Fatalf("private resolved URL should fail")
	}
}
