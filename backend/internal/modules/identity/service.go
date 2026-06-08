// M1 业务逻辑层(service)—— 服务结构与共享依赖/辅助。
// 职责:认证/账号/组织/导入/租户审核的业务规则与状态机;
//
//	经 repo 访问数据(全 sqlc),经 platform 取基础设施。
//
// 错误显式处理、分层暴露(对外 apperr 友好文案,内部链入日志)。
package identity

import (
	"regexp"
	"strings"
	"time"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/redis"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"

	"github.com/jackc/pgx/v5/pgtype"
)

// Service 是 M1 的业务服务,聚合依赖。
type Service struct {
	repo                     *repo
	auth                     *auth.Manager
	bus                      eventbus.Bus
	redis                    *redis.Client
	idgen                    *snowflake.Node
	cipher                   *crypto.Cipher
	sms                      SmsSender
	hmacKey                  []byte
	cfg                      config.DeployConfig
	activationCodeTTL        time.Duration
	ssoNetworkTimeout        time.Duration
	ssoAllowedServiceOrigins []string
	refreshTTL               time.Duration
	passwordMaxFailedCount   int16
	passwordLockMinutes      int
	smsResendInterval        time.Duration
	smsDailyLimit            int64
	smsCodeTTL               time.Duration
	smsVerifyMaxAttempts     int64
	importMaxRows            int
	importPreviewTTL         time.Duration
}

// NewService 构造 M1 服务;cipher 用 APP_ENCRYPTION_KEY 初始化(手机号加密);
// sms 由装配按 APP_ENV 注入(dev=LogSmsSender,生产=真实网关)。
func NewService(
	database *db.DB,
	authMgr *auth.Manager,
	bus eventbus.Bus,
	rc *redis.Client,
	idgen *snowflake.Node,
	cipher *crypto.Cipher,
	sms SmsSender,
	hmacKey []byte,
	deployCfg config.DeployConfig,
	identityCfg config.IdentityConfig,
	refreshTTL time.Duration,
) *Service {
	return &Service{
		repo:                     newRepo(database),
		auth:                     authMgr,
		bus:                      bus,
		redis:                    rc,
		idgen:                    idgen,
		cipher:                   cipher,
		sms:                      sms,
		hmacKey:                  hmacKey,
		cfg:                      deployCfg,
		activationCodeTTL:        time.Duration(identityCfg.ActivationCodeTTLHours) * time.Hour,
		ssoNetworkTimeout:        time.Duration(identityCfg.SSONetworkTimeoutSeconds) * time.Second,
		ssoAllowedServiceOrigins: identityCfg.SSOAllowedServiceOrigins,
		refreshTTL:               refreshTTL,
		passwordMaxFailedCount:   int16(identityCfg.PasswordMaxFailedCount),
		passwordLockMinutes:      identityCfg.PasswordLockMinutes,
		smsResendInterval:        time.Duration(identityCfg.SMSResendSeconds) * time.Second,
		smsDailyLimit:            int64(identityCfg.SMSDailyLimit),
		smsCodeTTL:               time.Duration(identityCfg.SMSCodeTTLMinutes) * time.Minute,
		smsVerifyMaxAttempts:     int64(identityCfg.SMSVerifyMaxAttempts),
		importMaxRows:            identityCfg.ImportMaxRows,
		importPreviewTTL:         time.Duration(identityCfg.ImportPreviewTTLHours) * time.Hour,
	}
}

// phoneHash 计算手机号的 HMAC(唯一约束与查询用)。
func (s *Service) phoneHash(phone string) string {
	return crypto.HMACHash(s.hmacKey, phone)
}

// encryptPhone 加密手机号明文(落库 phone_enc)。
func (s *Service) encryptPhone(phone string) ([]byte, error) {
	return s.cipher.Encrypt([]byte(phone))
}

// decryptPhone 解密 phone_enc 为明文(展示前脱敏)。
func (s *Service) decryptPhone(enc []byte) (string, error) {
	b, err := s.cipher.Decrypt(enc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ---- 小工具 ----

// maskPhone 手机号脱敏:138****1234。
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

var weakPasswordDictionary = map[string]struct{}{
	"password":      {},
	"password123":   {},
	"qwerty123":     {},
	"admin123":      {},
	"admin123456":   {},
	"chaimir123":    {},
	"12345678a":     {},
	"abc123456":     {},
	"letmein123":    {},
	"welcome123":    {},
	"changeme123":   {},
	"iloveyou123":   {},
	"student123":    {},
	"teacher123":    {},
	"blockchain123": {},
}

// validPassword 校验密码强度,并拒绝常见弱口令字典命中项。
func validPassword(pw string) bool {
	if len(pw) < 8 {
		return false
	}
	if _, weak := weakPasswordDictionary[strings.ToLower(strings.TrimSpace(pw))]; weak {
		return false
	}
	var hasLetter, hasDigit bool
	for _, c := range pw {
		switch {
		case c >= '0' && c <= '9':
			hasDigit = true
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
			hasLetter = true
		}
	}
	return hasLetter && hasDigit
}

var cnMobilePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

// validCNPhone 校验中国大陆手机号格式。
func validCNPhone(phone string) bool {
	return cnMobilePattern.MatchString(phone)
}

// genTempPassword 生成系统开通账号用临时密码,随机主体来自 CSPRNG。
func (s *Service) genTempPassword() (string, error) {
	raw, err := crypto.RandomToken(10)
	if err != nil {
		return "", err
	}
	return "Cm" + raw + "9", nil // 前缀字母 + 随机主体 + 数字,满足强度。
}

// pgText 将空字符串映射为 SQL NULL,用于可选文本字段。
func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// pgInt2 构造可选 smallint 字段,valid 由业务校验结果决定。
func pgInt2(v int16, valid bool) pgtype.Int2 {
	return pgtype.Int2{Int16: v, Valid: valid}
}

// pgInt8 构造可选 bigint 字段,用于外键或创建人等可空 ID。
func pgInt8(v int64, valid bool) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: valid}
}

// loginableStatus 校验账号状态是否可登录(仅"正常")。
func loginableStatus(status int16) error {
	switch status {
	case AccountActive:
		return nil
	case AccountPending:
		return apperr.ErrAccountInactive
	case AccountDisabled:
		return apperr.ErrAccountDisabled
	case AccountArchived:
		return apperr.ErrAccountArchived
	case AccountCancelled:
		return apperr.ErrAccountCancelled
	default:
		return apperr.ErrAccountDisabled
	}
}
