// identity service 文件定义服务依赖注入和通用业务辅助,不接收数据库连接。
package identity

import (
	"encoding/base64"
	"fmt"
	"time"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"
)

// Service 承载 identity 模块业务编排,依赖 repo 接口和平台横切能力。
type Service struct {
	store       Store
	auth        *auth.Manager
	redis       *redis.Client
	ids         snowflake.Generator
	cipher      *crypto.Cipher
	hmacKey     []byte
	cfg         config.IdentityConfig
	uploadCfg   config.UploadConfig
	deploy      config.DeployConfig
	authCfg     config.AuthConfig
	sms         SMSSender
	auditWriter *AuditWriter
}

// NewService 构造 identity 服务,不接收数据库连接,由装配层传入 Store。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("identity service 缺少 store")
	}
	if deps.Auth == nil {
		return nil, fmt.Errorf("identity service 缺少 auth manager")
	}
	if deps.IDs == nil {
		return nil, fmt.Errorf("identity service 缺少 ID 生成器")
	}
	key, err := base64.StdEncoding.DecodeString(deps.AuthConfig.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("解析 APP_ENCRYPTION_KEY 失败: %w", err)
	}
	cipher, err := crypto.NewCipher(key)
	if err != nil {
		return nil, err
	}
	s := &Service{
		store:     deps.Store,
		auth:      deps.Auth,
		redis:     deps.Redis,
		ids:       deps.IDs,
		cipher:    cipher,
		hmacKey:   []byte(deps.AuthConfig.HMACKey),
		cfg:       deps.IdentityConfig,
		uploadCfg: deps.UploadConfig,
		deploy:    deps.DeployConfig,
		authCfg:   deps.AuthConfig,
		sms:       deps.SMSSender,
	}
	if s.sms == nil {
		return nil, fmt.Errorf("identity service 缺少短信发送器")
	}
	s.auditWriter = &AuditWriter{store: deps.Store, ids: deps.IDs}
	return s, nil
}

// ServiceDeps 是 identity service 的装配依赖集合。
type ServiceDeps struct {
	Store          Store
	Auth           *auth.Manager
	Redis          *redis.Client
	IDs            snowflake.Generator
	AuthConfig     config.AuthConfig
	IdentityConfig config.IdentityConfig
	UploadConfig   config.UploadConfig
	DeployConfig   config.DeployConfig
	SMSSender      SMSSender
}

// AuditWriter 返回写入全平台唯一 audit_log 的审计实现。
func (s *Service) AuditWriter() *AuditWriter {
	return s.auditWriter
}

// refreshExpireAt 计算 Refresh Token 过期时间。
func (s *Service) refreshExpireAt() time.Time {
	return timex.Now().Add(time.Duration(s.authCfg.RefreshTTLDay) * 24 * time.Hour)
}

// importMaxBytes 返回统一上传配置中的导入文件大小上限,配置缺失应在启动装配阶段失败。
func (s *Service) importMaxBytes() int64 {
	return s.uploadCfg.ImportMaxBytes
}

// hashSecret 使用统一 HMAC 密钥哈希不透明凭证。
func (s *Service) hashSecret(value string) (string, error) {
	return crypto.HMACHash(s.hmacKey, value)
}

// phoneHash 计算手机号查询哈希。
func (s *Service) phoneHash(phone string) (string, error) {
	return crypto.HMACHash(s.hmacKey, phone)
}

// encryptPhone 加密手机号明文。
func (s *Service) encryptPhone(phone string) ([]byte, error) {
	return s.cipher.Encrypt([]byte(phone))
}

// decryptPhone 解密手机号密文,解密失败时向上返回错误供日志记录。
func (s *Service) decryptPhone(ciphertext []byte) (string, error) {
	plain, err := s.cipher.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
