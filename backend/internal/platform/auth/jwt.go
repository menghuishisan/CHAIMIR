// auth 实现 JWT access token 签发与校验,并承载内部服务 HMAC 鉴权所需配置。
package auth

import (
	"errors"
	"fmt"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/timex"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType 区分 access 与 refresh。
type TokenType string

const (
	// AccessToken 表示短期 JWT access token。
	AccessToken TokenType = "access"
	// RefreshToken 预留给需要 JWT 形态时的 token 类型声明;当前刷新令牌是随机串,不由本包签发。
	RefreshToken TokenType = "refresh"
)

// Claims 是 access token 的受控载荷。
type Claims struct {
	TenantID   int64     `json:"tid"`
	AccountID  int64     `json:"aid"`
	SessionID  int64     `json:"sid"`
	IsPlatform bool      `json:"plat"`
	Type       TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// Manager 负责 JWT 签发校验和服务签名时间窗口配置。
type Manager struct {
	signingKey     []byte
	hmacKey        []byte
	accessTTL      time.Duration
	issuer         string
	serviceMaxSkew time.Duration
}

// NewManager 根据统一鉴权配置构造鉴权管理器。
func NewManager(cfg config.AuthConfig) *Manager {
	return &Manager{
		signingKey:     []byte(cfg.JWTSigningKey),
		hmacKey:        []byte(cfg.HMACKey),
		accessTTL:      time.Duration(cfg.AccessTTLMin) * time.Minute,
		issuer:         cfg.JWTIssuer,
		serviceMaxSkew: time.Duration(cfg.ServiceAuthMaxSkewSeconds) * time.Second,
	}
}

// IssueAccess 签发 access token。
func (m *Manager) IssueAccess(tenantID, accountID, sessionID int64, isPlatform bool) (string, error) {
	now := timex.Now()
	claims := Claims{
		TenantID:   tenantID,
		AccountID:  accountID,
		SessionID:  sessionID,
		IsPlatform: isPlatform,
		Type:       AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.signingKey)
	if err != nil {
		return "", fmt.Errorf("签发 JWT 失败: %w", err)
	}
	return signed, nil
}

// VerifyAccess 校验 access token 的签名、时效和最小身份边界。
func (m *Manager) VerifyAccess(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期签名算法: %v", token.Header["alg"])
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
