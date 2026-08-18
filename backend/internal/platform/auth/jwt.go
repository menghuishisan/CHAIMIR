// auth 实现 JWT access token 签发与校验,并承载内部服务 HMAC 鉴权所需配置。
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/timex"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType 区分当前由 JWT 管理器签发的 token 类型。
type TokenType string

const (
	// AccessToken 表示短期 JWT access token。
	AccessToken TokenType = "access"
	// WebSocketTicket 表示只能用于指定 WebSocket 路径的短时连接票据。
	WebSocketTicket TokenType = "ws_ticket"
	// BrowserAccessTicket 表示只能用于指定浏览器工具路径前缀的短时入口票据。
	BrowserAccessTicket TokenType = "browser_ticket"
	// maxPathTicketPathLength 限制票据绑定路径长度,避免异常 URL 被签入凭证。
	maxPathTicketPathLength = 512
)

// Claims 是 access token 的受控载荷。
type Claims struct {
	TenantID   int64     `json:"tid"`
	AccountID  int64     `json:"aid"`
	SessionID  int64     `json:"sid"`
	IsPlatform bool      `json:"plat"`
	Type       TokenType `json:"typ"`
	BoundPath  string    `json:"pth,omitempty"`
	jwt.RegisteredClaims
}

// SessionIdentity 是 access token 中用于服务端会话二次校验的最小身份快照。
type SessionIdentity struct {
	TenantID   int64
	AccountID  int64
	SessionID  int64
	IsPlatform bool
	Method     string
	Path       string
}

// SessionValidator 校验 JWT 所指向的服务端会话仍处于有效状态。
type SessionValidator interface {
	ValidateAccessSession(ctx context.Context, id SessionIdentity) error
}

// Manager 负责 JWT 签发校验和服务签名时间窗口配置。
type Manager struct {
	signingKey     []byte
	hmacKey        []byte
	accessTTL      time.Duration
	pathTicketTTL  time.Duration
	refreshTTL     time.Duration
	issuer         string
	serviceMaxSkew time.Duration
	sessions       SessionValidator
}

// NewManager 根据统一鉴权配置构造鉴权管理器。
func NewManager(cfg config.AuthConfig) *Manager {
	return &Manager{
		signingKey:     []byte(cfg.JWTSigningKey),
		hmacKey:        []byte(cfg.HMACKey),
		accessTTL:      time.Duration(cfg.AccessTTLMin) * time.Minute,
		pathTicketTTL:  time.Duration(cfg.PathTicketTTLSeconds) * time.Second,
		refreshTTL:     time.Duration(cfg.RefreshTTLDay) * 24 * time.Hour,
		issuer:         cfg.JWTIssuer,
		serviceMaxSkew: time.Duration(cfg.ServiceAuthMaxSkewSeconds) * time.Second,
	}
}

// SetSessionValidator 注入服务端会话校验器,由 identity 模块实现具体表校验。
func (m *Manager) SetSessionValidator(validator SessionValidator) {
	m.sessions = validator
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

// IssueWebSocketTicket 签发短时、路径绑定的 WebSocket 连接票据。
func (m *Manager) IssueWebSocketTicket(id SessionIdentity) (string, time.Time, error) {
	return m.issuePathTicket(id, WebSocketTicket, "WebSocket")
}

// IssueBrowserAccessTicket 签发短时、路径前缀绑定的浏览器工具入口票据。
func (m *Manager) IssueBrowserAccessTicket(id SessionIdentity) (string, time.Time, error) {
	return m.issuePathTicket(id, BrowserAccessTicket, "浏览器工具")
}

// VerifyAccess 校验 access token 的签名、时效和最小身份边界。
func (m *Manager) VerifyAccess(tokenString string) (*Claims, error) {
	claims, err := m.parseClaims(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Type != AccessToken {
		return nil, errors.New("token 类型不匹配")
	}
	return claims, nil
}

// VerifyWebSocketTicket 校验连接票据签名、时效、身份载荷和路径绑定。
func (m *Manager) VerifyWebSocketTicket(tokenString, requestPath string) (*Claims, error) {
	claims, claimPath, expectedPath, err := m.verifyPathTicket(tokenString, requestPath, WebSocketTicket, "WebSocket")
	if err != nil {
		return nil, err
	}
	if claimPath != expectedPath {
		return nil, errors.New("WebSocket 票据路径不匹配")
	}
	return claims, nil
}

// VerifyBrowserAccessTicket 校验浏览器工具票据签名、时效、身份载荷和路径前缀。
func (m *Manager) VerifyBrowserAccessTicket(tokenString, requestPath string) (*Claims, error) {
	claims, claimPath, expectedPath, err := m.verifyPathTicket(tokenString, requestPath, BrowserAccessTicket, "浏览器工具")
	if err != nil {
		return nil, err
	}
	if expectedPath != claimPath && !strings.HasPrefix(expectedPath, claimPath+"/") {
		return nil, errors.New("浏览器工具票据路径不匹配")
	}
	return claims, nil
}

// issuePathTicket 统一签发短时路径票据,由 token 类型区分 WS 精确路径与工具路径前缀语义。
func (m *Manager) issuePathTicket(id SessionIdentity, tokenType TokenType, label string) (string, time.Time, error) {
	boundPath, err := normalizePathTicketPath(id.Path)
	if err != nil {
		return "", time.Time{}, err
	}
	if id.AccountID <= 0 || id.SessionID <= 0 {
		return "", time.Time{}, fmt.Errorf("%s票据身份载荷不完整", label)
	}
	if !id.IsPlatform && id.TenantID <= 0 {
		return "", time.Time{}, fmt.Errorf("%s票据缺少租户边界", label)
	}
	now := timex.Now()
	expiresAt := now.Add(m.pathTicketTTL)
	claims := Claims{
		TenantID:   id.TenantID,
		AccountID:  id.AccountID,
		SessionID:  id.SessionID,
		IsPlatform: id.IsPlatform,
		Type:       tokenType,
		BoundPath:  boundPath,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.signingKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("签发%s票据失败: %w", label, err)
	}
	return signed, expiresAt, nil
}

// verifyPathTicket 统一校验路径票据的公共签名、类型和规范化路径。
func (m *Manager) verifyPathTicket(tokenString, requestPath string, tokenType TokenType, label string) (*Claims, string, string, error) {
	expectedPath, err := normalizePathTicketPath(requestPath)
	if err != nil {
		return nil, "", "", err
	}
	claims, err := m.parseClaims(tokenString)
	if err != nil {
		return nil, "", "", err
	}
	if claims.Type != tokenType {
		return nil, "", "", fmt.Errorf("%s票据类型不匹配", label)
	}
	claimPath, err := normalizePathTicketPath(claims.BoundPath)
	if err != nil {
		return nil, "", "", err
	}
	return claims, claimPath, expectedPath, nil
}

// parseClaims 校验 JWT 签名、签发方、时效和共享身份边界。
func (m *Manager) parseClaims(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("非预期签名算法: %v", token.Header["alg"])
		}
		return m.signingKey, nil
	}, jwt.WithIssuer(m.issuer))
	if err != nil {
		return nil, fmt.Errorf("JWT 校验失败: %w", err)
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return nil, errors.New("token 缺少有效期声明")
	}
	if claims.AccountID <= 0 || claims.SessionID <= 0 {
		return nil, errors.New("token 身份载荷不完整")
	}
	if !claims.IsPlatform && claims.TenantID <= 0 {
		return nil, errors.New("租户 Token 缺少租户边界")
	}
	return claims, nil
}

// normalizePathTicketPath 校验并规范化票据绑定路径,不接受查询串、绝对 URL 或逃逸片段。
func normalizePathTicketPath(raw string) (string, error) {
	path := strings.TrimSuffix(strings.TrimSpace(raw), "/")
	if path == "" || len(path) > maxPathTicketPathLength {
		return "", errors.New("票据路径无效")
	}
	if !strings.HasPrefix(path, "/api/") {
		return "", errors.New("票据路径缺少 API 边界")
	}
	if strings.Contains(path, "?") || strings.Contains(path, "#") || strings.Contains(path, "//") {
		return "", errors.New("票据路径包含无效片段")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("票据路径包含逃逸片段")
		}
	}
	return path, nil
}
