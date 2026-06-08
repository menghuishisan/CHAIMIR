// Package auth 实现 JWT 双 Token 的签发与校验,以及鉴权中间件。
// 依据 docs/总-API接口总览.md §3:Authorization: Bearer <access>;Access 15min、Refresh 7d;
//
//	角色从服务端会话取,不入 Token、不接受客户端传参(CLAUDE.md §7)。
package auth

import (
	"errors"
	"fmt"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/timex"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType 区分 access / refresh。
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// Claims 是 JWT 载荷;角色不入 Token(从服务端取),仅放身份与会话引用。
type Claims struct {
	TenantID   int64     `json:"tid"`
	AccountID  int64     `json:"aid"`
	SessionID  int64     `json:"sid"`
	IsPlatform bool      `json:"plat"`
	Type       TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// Manager 负责签发与校验。
type Manager struct {
	signingKey     []byte
	hmacKey        []byte
	accessTTL      time.Duration
	issuer         string
	serviceMaxSkew time.Duration
}

// NewManager 用鉴权配置构造。
func NewManager(cfg config.AuthConfig) *Manager {
	return &Manager{
		signingKey:     []byte(cfg.JWTSigningKey),
		hmacKey:        []byte(cfg.HMACKey),
		accessTTL:      time.Duration(cfg.AccessTTLMin) * time.Minute,
		issuer:         cfg.JWTIssuer,
		serviceMaxSkew: time.Duration(cfg.ServiceAuthMaxSkewSeconds) * time.Second,
	}
}

// IssueAccess 签发 access Token(refresh 为不透明随机串,由 identity 管理,不在此签 JWT)。
func (m *Manager) IssueAccess(tenantID, accountID, sessionID int64, isPlatform bool) (string, error) {
	now := timex.Now()
	claims := Claims{
		TenantID: tenantID, AccountID: accountID, SessionID: sessionID,
		IsPlatform: isPlatform, Type: AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.signingKey)
	if err != nil {
		return "", fmt.Errorf("签发 JWT 失败: %w", err)
	}
	return s, nil
}

// VerifyAccess 校验 access Token 签名、有效期和最小身份载荷。
func (m *Manager) VerifyAccess(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期签名算法: %v", t.Header["alg"])
		}
		return m.signingKey, nil
	}, jwt.WithIssuer(m.issuer))
	if err != nil {
		return nil, fmt.Errorf("JWT 校验失败: %w", err)
	}
	if claims.Type != AccessToken {
		return nil, errors.New("Token 类型不匹配")
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return nil, errors.New("Token 缺少有效期声明")
	}
	if claims.AccountID <= 0 || claims.SessionID <= 0 {
		return nil, errors.New("Token 身份载荷不完整")
	}
	if !claims.IsPlatform && claims.TenantID <= 0 {
		return nil, errors.New("租户 Token 缺少租户边界")
	}
	return claims, nil
}
