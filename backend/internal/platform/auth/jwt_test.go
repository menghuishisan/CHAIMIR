// Package auth 测试 JWT 鉴权时间边界和载荷约束。
package auth

import (
	"os"
	"strings"
	"testing"

	"chaimir/internal/platform/config"

	"github.com/golang-jwt/jwt/v5"
)

// TestIssueAccessDoesNotReadLocalCurrentTime 确认 JWT 签发时间通过 platform/timex 入口取得。
func TestIssueAccessDoesNotReadLocalCurrentTime(t *testing.T) {
	data, err := os.ReadFile("jwt.go")
	if err != nil {
		t.Fatalf("read jwt.go: %v", err)
	}
	if strings.Contains(string(data), "time.Now()") {
		t.Fatal("IssueAccess must use platform/timex instead of direct local current time")
	}
}

// TestIssueAccessKeepsTokenSemantics 确认时间入口收敛后 JWT 仍可签发和校验。
func TestIssueAccessKeepsTokenSemantics(t *testing.T) {
	mgr := NewManager(config.AuthConfig{
		JWTSigningKey: "test-signing-key",
		HMACKey:       "test-hmac-key",
		AccessTTLMin:  15,
		JWTIssuer:     "chaimir-test",
	})

	token, err := mgr.IssueAccess(1001, 2001, 3001, false)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	claims, err := mgr.VerifyAccess(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.TenantID != 1001 || claims.AccountID != 2001 || claims.SessionID != 3001 || claims.Type != AccessToken {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

// TestVerifyAccessRejectsTokenWithoutExpiration 确认 access token 必须携带 exp,避免永久有效令牌被接受。
func TestVerifyAccessRejectsTokenWithoutExpiration(t *testing.T) {
	mgr := NewManager(config.AuthConfig{
		JWTSigningKey: "test-signing-key",
		HMACKey:       "test-hmac-key",
		AccessTTLMin:  15,
		JWTIssuer:     "chaimir-test",
	})
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		TenantID: 1001, AccountID: 2001, SessionID: 3001, Type: AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "chaimir-test"},
	}).SignedString([]byte("test-signing-key"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := mgr.VerifyAccess(token); err == nil {
		t.Fatalf("token without exp must be rejected")
	}
}
