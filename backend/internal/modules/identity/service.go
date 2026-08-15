// identity service 文件定义服务依赖注入和通用业务辅助,不接收数据库连接。
package identity

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"
)

const (
	// identityModuleName 与 tenantLogo* 共同构成校徽对象在统一文件服务里的受控前缀。
	identityModuleName     = "identity"
	tenantLogoResourceType = "logo"
	// tenantLogoResourceID 固定为 current:一个租户只有一枚在用校徽,不需要 id 维度。
	tenantLogoResourceID = "current"
)

// baseRoleNumber 将 identity 基础身份映射为跨模块 RBAC 数字契约,只在 service 边界适配。
func baseRoleNumber(baseIdentity int16) (int16, error) {
	if err := ValidateBaseIdentity(baseIdentity); err != nil {
		return 0, err
	}
	switch baseIdentity {
	case BaseIdentityTeacher:
		return contracts.RoleNumTeacher, nil
	case BaseIdentityStudent:
		return contracts.RoleNumStudent, nil
	default:
		return 0, apperr.ErrIdentityBaseRoleInvalid
	}
}

// objectStorage 描述 M1 写入与读取校徽对象所需的共享对象存储能力。
type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, bucket, key string) error
	BucketAttach() string
}

// fileService 描述 M1 复用统一文件服务规划受控对象路径所需能力。
type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
}

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
	scanner     upload.Scanner
	objects     objectStorage
	files       fileService
	deploy      config.DeployConfig
	authCfg     config.AuthConfig
	sms         SMSSender
	auditWriter *AuditWriter
	bus         eventbus.Bus
}

// filterLoginCandidatesForDeployment 将私有化部署的预认证候选收敛到固定学校租户。
// 私有化实例只对外提供一个租户边界，不能把共享数据库中的同手机号账号暴露给登录选择器。
func filterLoginCandidatesForDeployment(candidates []LoginCandidate, deploy config.DeployConfig) []LoginCandidate {
	if !deploy.IsSchool() {
		return candidates
	}
	filtered := make([]LoginCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.TenantID == deploy.SchoolTenantID {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
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
	if deps.Objects == nil {
		return nil, fmt.Errorf("identity service 缺少统一对象存储")
	}
	if deps.FileService == nil {
		return nil, fmt.Errorf("identity service 缺少统一文件服务")
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
		scanner:   deps.Scanner,
		objects:   deps.Objects,
		files:     deps.FileService,
		deploy:    deps.DeployConfig,
		authCfg:   deps.AuthConfig,
		sms:       deps.SMSSender,
		bus:       deps.EventBus,
	}
	if s.sms == nil {
		return nil, fmt.Errorf("identity service 缺少短信发送器")
	}
	s.auditWriter = &AuditWriter{store: deps.Store, ids: deps.IDs}
	deps.Auth.SetSessionValidator(s)
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
	Scanner        upload.Scanner
	Objects        objectStorage
	FileService    fileService
	DeployConfig   config.DeployConfig
	SMSSender      SMSSender
	EventBus       eventbus.Bus
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

// protectPhone 生成手机号存储所需的密文与检索哈希,统一走 crypto.ProtectPhone,与验收种子共享唯一实现。
func (s *Service) protectPhone(phone string) ([]byte, string, error) {
	return crypto.ProtectPhone(s.cipher, s.hmacKey, phone)
}

// decryptPhone 解密手机号密文,解密失败时向上返回错误供日志记录。
func (s *Service) decryptPhone(ciphertext []byte) (string, error) {
	plain, err := s.cipher.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
