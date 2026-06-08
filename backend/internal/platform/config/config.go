// Package config 统一从环境变量加载配置(禁硬编码,总-技术选型 §2.4)。
// 启动时一次性读取并校验,缺失必填项即 fail-fast(边界处校验)。
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Config 是后端运行所需的全部配置(分组对应 .env.example)。
type Config struct {
	Deploy     DeployConfig
	Server     ServerConfig
	Postgres   PostgresConfig
	Redis      RedisConfig
	NATS       NATSConfig
	MinIO      MinIOConfig
	Auth       AuthConfig
	Bootstrap  BootstrapConfig
	Identity   IdentityConfig
	SMS        SMSConfig
	Upload     UploadConfig
	Contest    ContestConfig
	Notify     NotifyConfig
	Teaching   TeachingConfig
	Grade      GradeConfig
	Sandbox    SandboxConfig
	Judge      JudgeConfig
	Monitoring MonitoringConfig
	Snowflake  SnowflakeConfig
}

// DeployConfig 部署形态。
type DeployConfig struct {
	Mode            string // saas / school
	PlatformEnabled bool
	SchoolTenantID  int64
}

// ServerConfig 服务监听与日志。
type ServerConfig struct {
	Addr                   string
	Port                   int
	WSPath                 string
	WSAllowedOrigins       []string
	LogLevel               string
	LogFormat              string
	AppEnv                 string
	HealthTimeoutSeconds   int
	ShutdownTimeoutSeconds int
}

// PostgresConfig 数据库连接。含特权连接(绕 RLS,仅登录前跨租户定位)。
type PostgresConfig struct {
	Host     string
	Port     int
	Database string
	User     string // 应用连接用户(chaimir_app,受 RLS)。
	Password string
	SSLMode  string
	MaxConns int
	MinConns int
	// 特权连接(属主,绕 RLS);为空表示不启用。
	PrivUser     string
	PrivPassword string
}

// RedisConfig 缓存/会话。
type RedisConfig struct {
	Host               string
	Port               int
	Password           string
	DB                 int
	PingTimeoutSeconds int
}

// NATSConfig 事件总线。
type NATSConfig struct {
	URL                  string
	Token                string
	ReconnectWaitSeconds int
}

// MinIOConfig 对象存储。
type MinIOConfig struct {
	Endpoint           string
	UseSSL             bool
	Region             string
	AccessKey          string
	SecretKey          string
	BucketCode         string
	BucketAttach       string
	BucketReport       string
	BucketBackup       string
	PingTimeoutSeconds int
}

// AuthConfig 鉴权与加密密钥。
type AuthConfig struct {
	JWTSigningKey             string
	AccessTTLMin              int
	RefreshTTLDay             int
	JWTIssuer                 string
	EncryptionKey             string
	HMACKey                   string
	ServiceAuthMaxSkewSeconds int
}

// BootstrapConfig 定义迁移/初始化命令所需的首个管理员参数。
// SaaS 使用平台管理员字段;私有化使用学校租户与首个学校管理员字段。
type BootstrapConfig struct {
	SchoolTenantID        int64
	SchoolTenantCode      string
	SchoolName            string
	SchoolType            int16
	AdminPhone            string
	AdminName             string
	AdminPassword         string
	PlatformAdminUser     string
	PlatformAdminName     string
	PlatformAdminPassword string
}

// IdentityConfig 定义 M1 账号开通与 SSO 协议的运行边界。
type IdentityConfig struct {
	ActivationCodeTTLHours   int
	SSONetworkTimeoutSeconds int
	SSOAllowedServiceOrigins []string
	PasswordMaxFailedCount   int
	PasswordLockMinutes      int
	SMSResendSeconds         int
	SMSDailyLimit            int
	SMSCodeTTLMinutes        int
	SMSVerifyMaxAttempts     int
	ImportMaxRows            int
	ImportPreviewTTLHours    int
}

// SMSConfig 短信验证码网关配置。
type SMSConfig struct {
	Provider       string // log(仅非生产) / http
	Endpoint       string
	Token          string
	LoginTemplate  string
	ResetTemplate  string
	ChangeTemplate string
	TimeoutSeconds int
}

// UploadConfig 定义用户上传和归档展开的统一服务端边界。
type UploadConfig struct {
	ImportMaxBytes            int64
	SimBundleMaxBytes         int64
	SimBundleMaxFiles         int
	SimBundleMaxUnpackedBytes int64
}

// ContestConfig 定义竞赛模块访问外部系统时的服务端边界。
type ContestConfig struct {
	VulnSourceMaxResponseBytes int64
	VulnSourceTimeoutSeconds   int
}

// NotifyConfig 定义通知事件消费的重试边界,由事件入口统一使用。
type NotifyConfig struct {
	EventRetryMax     int
	EventRetryDelayMs int
	UnreadTTLHours    int
}

// TeachingConfig 定义 M6 对外/跨模块读取边界。
type TeachingConfig struct {
	CourseGradesMaxRows       int
	JudgeOutboxBatchSize      int
	JudgeOutboxPollIntervalMs int
}

// GradeConfig 定义 M11 审核、申诉和成绩单流程的运行边界。
type GradeConfig struct {
	AppealWindowDays     int
	TranscriptSigningKey string
}

// SandboxConfig K8s 沙箱编排。
type SandboxConfig struct {
	KubeconfigPath               string
	NSPrefixStudent              string
	NSPrefixJudge                string
	NSPrefixBattle               string
	PrepullNamespace             string
	ImageRegistry                string
	ImageAttestations            []SandboxImageAttestation
	DefaultCPU                   string
	DefaultMemory                string
	DefaultReqCPU                string
	DefaultReqMemory             string
	MaxCPU                       string
	MaxMemory                    string
	MaxPods                      string
	WorkspaceStorage             string
	PrepullTimeoutSeconds        int
	ReadyTimeoutSeconds          int
	PrepullPollIntervalSeconds   int
	ReadyPollIntervalSeconds     int
	PrepullRequestCPU            string
	PrepullRequestMemory         string
	PrepullLimitCPU              string
	PrepullLimitMemory           string
	ChainRPCTimeoutSeconds       int
	InitArchiveMaxFiles          int
	InitArchiveMaxUnpackedBytes  int64
	ProbeDefaultPeriodSeconds    int32
	ProbeDefaultFailureThreshold int32
	RecyclePollIntervalSeconds   int
	RecycleBatchSize             int
	ReadyIdleTimeoutSeconds      int
	ControlNamespace             string
	ControlPodLabelKey           string
	ControlPodLabelValue         string
}

// SandboxImageAttestation 是 CI/Harbor 产出的受控镜像安全证明。
type SandboxImageAttestation struct {
	ImageURL       string `json:"image_url"`
	Digest         string `json:"digest"`
	CosignVerified bool   `json:"cosign_verified"`
	TrivyStatus    string `json:"trivy_status"`
}

// JudgeConfig M3 判题队列与限频配置。
type JudgeConfig struct {
	QueuePollIntervalMs        int
	WorkerBatchSize            int
	SubmitRateLimitSec         int
	DefaultMaxRetries          int
	SandboxReadyPollIntervalMs int
	ResultDetailsMaxBytes      int
	InputInjectTimeoutSeconds  int
}

// MonitoringConfig 是 M9 外接监控面板嵌入配置。
type MonitoringConfig struct {
	PanelsJSON string
}

// SnowflakeConfig 雪花 ID 节点。
type SnowflakeConfig struct {
	NodeID int64
}

// Load 从环境变量装载并校验配置;任何必填项缺失/格式错即返回错误。
func Load() (*Config, error) {
	c := &Config{}
	var errs []string
	// 第一步:定义只读环境变量的解析器,所有缺失和格式错误都收集后一次性返回。
	req := func(key string) string {
		v := os.Getenv(key)
		if strings.TrimSpace(v) == "" {
			errs = append(errs, "缺少必填环境变量: "+key)
		}
		return v
	}
	reqInt := func(key string) int {
		v := os.Getenv(key)
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			errs = append(errs, fmt.Sprintf("环境变量 %s 需为整数,实际=%q", key, v))
		}
		return n
	}
	reqInt64 := func(key string) int64 {
		v := os.Getenv(key)
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			errs = append(errs, fmt.Sprintf("环境变量 %s 需为 int64,实际=%q", key, v))
		}
		return n
	}
	optInt64 := func(key string) int64 {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			return 0
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Sprintf("环境变量 %s 需为 int64,实际=%q", key, v))
		}
		return n
	}
	reqBool := func(key string) bool {
		v := os.Getenv(key)
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y", "on":
			return true
		case "false", "0", "no", "n", "off":
			return false
		default:
			errs = append(errs, fmt.Sprintf("环境变量 %s 需为布尔值,实际=%q", key, v))
			return false
		}
	}

	// 第二步:按 .env.example 分组装载配置,避免业务代码再从环境变量旁路读取。
	c.Deploy = DeployConfig{
		Mode:            req("DEPLOY_MODE"),
		PlatformEnabled: reqBool("PLATFORM_LAYER_ENABLED"),
		SchoolTenantID:  optInt64("SCHOOL_TENANT_ID"),
	}
	c.Server = ServerConfig{
		Addr:                   req("HTTP_ADDR"),
		Port:                   reqInt("HTTP_PORT"),
		WSPath:                 req("WS_PATH"),
		WSAllowedOrigins:       getCSV("WS_ALLOWED_ORIGINS"),
		LogLevel:               req("LOG_LEVEL"),
		LogFormat:              req("LOG_FORMAT"),
		AppEnv:                 req("APP_ENV"),
		HealthTimeoutSeconds:   reqInt("HEALTH_CHECK_TIMEOUT_SECONDS"),
		ShutdownTimeoutSeconds: reqInt("HTTP_SHUTDOWN_TIMEOUT_SECONDS"),
	}
	c.Postgres = PostgresConfig{
		Host:         req("PG_HOST"),
		Port:         reqInt("PG_PORT"),
		Database:     req("PG_DATABASE"),
		User:         req("PG_USER"),
		Password:     req("PG_PASSWORD"),
		SSLMode:      req("PG_SSLMODE"),
		MaxConns:     reqInt("PG_MAX_CONNS"),
		MinConns:     reqInt("PG_MIN_CONNS"),
		PrivUser:     os.Getenv("PG_PRIV_USER"),
		PrivPassword: os.Getenv("PG_PRIV_PASSWORD"),
	}
	c.Redis = RedisConfig{
		Host:               req("REDIS_HOST"),
		Port:               reqInt("REDIS_PORT"),
		Password:           os.Getenv("REDIS_PASSWORD"),
		DB:                 reqInt("REDIS_DB"),
		PingTimeoutSeconds: reqInt("REDIS_PING_TIMEOUT_SECONDS"),
	}
	c.NATS = NATSConfig{
		URL:                  req("NATS_URL"),
		Token:                os.Getenv("NATS_TOKEN"),
		ReconnectWaitSeconds: reqInt("NATS_RECONNECT_WAIT_SECONDS"),
	}
	c.MinIO = MinIOConfig{
		Endpoint:           req("MINIO_ENDPOINT"),
		UseSSL:             reqBool("MINIO_USE_SSL"),
		Region:             req("MINIO_REGION"),
		AccessKey:          req("MINIO_ACCESS_KEY"),
		SecretKey:          req("MINIO_SECRET_KEY"),
		BucketCode:         req("MINIO_BUCKET_CODE"),
		BucketAttach:       req("MINIO_BUCKET_ATTACHMENT"),
		BucketReport:       req("MINIO_BUCKET_REPORT"),
		BucketBackup:       req("MINIO_BUCKET_BACKUP"),
		PingTimeoutSeconds: reqInt("MINIO_PING_TIMEOUT_SECONDS"),
	}
	c.Auth = AuthConfig{
		JWTSigningKey:             req("JWT_SIGNING_KEY"),
		AccessTTLMin:              reqInt("JWT_ACCESS_TTL_MIN"),
		RefreshTTLDay:             reqInt("JWT_REFRESH_TTL_DAY"),
		JWTIssuer:                 req("JWT_ISSUER"),
		EncryptionKey:             req("APP_ENCRYPTION_KEY"),
		HMACKey:                   req("APP_HMAC_KEY"),
		ServiceAuthMaxSkewSeconds: reqInt("SERVICE_AUTH_MAX_SKEW_SECONDS"),
	}
	c.Bootstrap = BootstrapConfig{
		SchoolTenantID:        c.Deploy.SchoolTenantID,
		SchoolTenantCode:      os.Getenv("BOOTSTRAP_SCHOOL_TENANT_CODE"),
		SchoolName:            os.Getenv("BOOTSTRAP_SCHOOL_NAME"),
		SchoolType:            int16(optInt64("BOOTSTRAP_SCHOOL_TYPE")),
		AdminPhone:            os.Getenv("BOOTSTRAP_ADMIN_PHONE"),
		AdminName:             os.Getenv("BOOTSTRAP_ADMIN_NAME"),
		AdminPassword:         os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		PlatformAdminUser:     os.Getenv("BOOTSTRAP_PLATFORM_ADMIN_USERNAME"),
		PlatformAdminName:     os.Getenv("BOOTSTRAP_PLATFORM_ADMIN_NAME"),
		PlatformAdminPassword: os.Getenv("BOOTSTRAP_PLATFORM_ADMIN_PASSWORD"),
	}
	c.Identity = IdentityConfig{
		ActivationCodeTTLHours:   reqInt("IDENTITY_ACTIVATION_CODE_TTL_HOURS"),
		SSONetworkTimeoutSeconds: reqInt("IDENTITY_SSO_NETWORK_TIMEOUT_SECONDS"),
		SSOAllowedServiceOrigins: getCSV("IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS"),
		PasswordMaxFailedCount:   reqInt("IDENTITY_PASSWORD_MAX_FAILED_COUNT"),
		PasswordLockMinutes:      reqInt("IDENTITY_PASSWORD_LOCK_MINUTES"),
		SMSResendSeconds:         reqInt("IDENTITY_SMS_RESEND_SECONDS"),
		SMSDailyLimit:            reqInt("IDENTITY_SMS_DAILY_LIMIT"),
		SMSCodeTTLMinutes:        reqInt("IDENTITY_SMS_CODE_TTL_MINUTES"),
		SMSVerifyMaxAttempts:     reqInt("IDENTITY_SMS_VERIFY_MAX_ATTEMPTS"),
		ImportMaxRows:            reqInt("IDENTITY_IMPORT_MAX_ROWS"),
		ImportPreviewTTLHours:    reqInt("IDENTITY_IMPORT_PREVIEW_TTL_HOURS"),
	}
	c.SMS = SMSConfig{
		Provider:       req("SMS_PROVIDER"),
		Endpoint:       os.Getenv("SMS_HTTP_ENDPOINT"),
		Token:          os.Getenv("SMS_HTTP_TOKEN"),
		LoginTemplate:  os.Getenv("SMS_TEMPLATE_LOGIN"),
		ResetTemplate:  os.Getenv("SMS_TEMPLATE_RESET"),
		ChangeTemplate: os.Getenv("SMS_TEMPLATE_CHANGE_PHONE"),
		TimeoutSeconds: reqInt("SMS_TIMEOUT_SECONDS"),
	}
	c.Upload = UploadConfig{
		ImportMaxBytes:            reqInt64("UPLOAD_IMPORT_MAX_BYTES"),
		SimBundleMaxBytes:         reqInt64("UPLOAD_SIM_BUNDLE_MAX_BYTES"),
		SimBundleMaxFiles:         reqInt("UPLOAD_SIM_BUNDLE_MAX_FILES"),
		SimBundleMaxUnpackedBytes: reqInt64("UPLOAD_SIM_BUNDLE_MAX_UNPACKED_BYTES"),
	}
	c.Contest = ContestConfig{
		VulnSourceMaxResponseBytes: reqInt64("CONTEST_VULN_SOURCE_MAX_RESPONSE_BYTES"),
		VulnSourceTimeoutSeconds:   reqInt("CONTEST_VULN_SOURCE_TIMEOUT_SECONDS"),
	}
	c.Notify = NotifyConfig{
		EventRetryMax:     reqInt("NOTIFY_EVENT_RETRY_MAX"),
		EventRetryDelayMs: reqInt("NOTIFY_EVENT_RETRY_DELAY_MS"),
		UnreadTTLHours:    reqInt("NOTIFY_UNREAD_TTL_HOURS"),
	}
	c.Teaching = TeachingConfig{
		CourseGradesMaxRows:       reqInt("TEACHING_COURSE_GRADES_MAX_ROWS"),
		JudgeOutboxBatchSize:      reqInt("TEACHING_JUDGE_OUTBOX_BATCH_SIZE"),
		JudgeOutboxPollIntervalMs: reqInt("TEACHING_JUDGE_OUTBOX_POLL_INTERVAL_MS"),
	}
	c.Grade = GradeConfig{
		AppealWindowDays:     reqInt("GRADE_APPEAL_WINDOW_DAYS"),
		TranscriptSigningKey: c.Auth.HMACKey,
	}
	c.Sandbox = SandboxConfig{
		KubeconfigPath:               os.Getenv("KUBECONFIG_PATH"),
		NSPrefixStudent:              req("SANDBOX_NS_PREFIX_STUDENT"),
		NSPrefixJudge:                req("SANDBOX_NS_PREFIX_JUDGE"),
		NSPrefixBattle:               req("SANDBOX_NS_PREFIX_BATTLE"),
		PrepullNamespace:             req("SANDBOX_PREPULL_NAMESPACE"),
		ImageRegistry:                req("IMAGE_REGISTRY"),
		ImageAttestations:            readSandboxImageAttestations("SANDBOX_IMAGE_ATTESTATIONS_JSON", &errs),
		DefaultCPU:                   req("SANDBOX_DEFAULT_CPU"),
		DefaultMemory:                req("SANDBOX_DEFAULT_MEMORY"),
		DefaultReqCPU:                req("SANDBOX_DEFAULT_REQUEST_CPU"),
		DefaultReqMemory:             req("SANDBOX_DEFAULT_REQUEST_MEMORY"),
		MaxCPU:                       req("SANDBOX_MAX_CPU"),
		MaxMemory:                    req("SANDBOX_MAX_MEMORY"),
		MaxPods:                      req("SANDBOX_MAX_PODS"),
		WorkspaceStorage:             req("SANDBOX_WORKSPACE_STORAGE"),
		PrepullTimeoutSeconds:        reqInt("SANDBOX_PREPULL_TIMEOUT_SECONDS"),
		ReadyTimeoutSeconds:          reqInt("SANDBOX_READY_TIMEOUT_SECONDS"),
		PrepullPollIntervalSeconds:   reqInt("SANDBOX_PREPULL_POLL_INTERVAL_SECONDS"),
		ReadyPollIntervalSeconds:     reqInt("SANDBOX_READY_POLL_INTERVAL_SECONDS"),
		PrepullRequestCPU:            req("SANDBOX_PREPULL_REQUEST_CPU"),
		PrepullRequestMemory:         req("SANDBOX_PREPULL_REQUEST_MEMORY"),
		PrepullLimitCPU:              req("SANDBOX_PREPULL_LIMIT_CPU"),
		PrepullLimitMemory:           req("SANDBOX_PREPULL_LIMIT_MEMORY"),
		ChainRPCTimeoutSeconds:       reqInt("SANDBOX_CHAIN_RPC_TIMEOUT_SECONDS"),
		InitArchiveMaxFiles:          reqInt("SANDBOX_INIT_ARCHIVE_MAX_FILES"),
		InitArchiveMaxUnpackedBytes:  reqInt64("SANDBOX_INIT_ARCHIVE_MAX_UNPACKED_BYTES"),
		ProbeDefaultPeriodSeconds:    int32(reqInt("SANDBOX_PROBE_DEFAULT_PERIOD_SECONDS")),
		ProbeDefaultFailureThreshold: int32(reqInt("SANDBOX_PROBE_DEFAULT_FAILURE_THRESHOLD")),
		RecyclePollIntervalSeconds:   reqInt("SANDBOX_RECYCLE_POLL_INTERVAL_SECONDS"),
		RecycleBatchSize:             reqInt("SANDBOX_RECYCLE_BATCH_SIZE"),
		ReadyIdleTimeoutSeconds:      reqInt("SANDBOX_READY_IDLE_TIMEOUT_SECONDS"),
		ControlNamespace:             req("SANDBOX_CONTROL_NAMESPACE"),
		ControlPodLabelKey:           req("SANDBOX_CONTROL_POD_LABEL_KEY"),
		ControlPodLabelValue:         req("SANDBOX_CONTROL_POD_LABEL_VALUE"),
	}
	errs = append(errs, validateSandboxQuantities(c.Sandbox)...)
	c.Judge = JudgeConfig{
		QueuePollIntervalMs:        reqInt("JUDGE_QUEUE_POLL_INTERVAL_MS"),
		WorkerBatchSize:            reqInt("JUDGE_WORKER_BATCH_SIZE"),
		SubmitRateLimitSec:         reqInt("JUDGE_SUBMIT_RATE_LIMIT_SECONDS"),
		DefaultMaxRetries:          reqInt("JUDGE_DEFAULT_MAX_RETRIES"),
		SandboxReadyPollIntervalMs: reqInt("JUDGE_SANDBOX_READY_POLL_INTERVAL_MS"),
		ResultDetailsMaxBytes:      reqInt("JUDGE_RESULT_DETAILS_MAX_BYTES"),
		InputInjectTimeoutSeconds:  reqInt("JUDGE_INPUT_INJECT_TIMEOUT_SECONDS"),
	}
	c.Monitoring = MonitoringConfig{
		PanelsJSON: req("MONITORING_PANELS_JSON"),
	}
	c.Snowflake = SnowflakeConfig{NodeID: reqInt64("SNOWFLAKE_NODE_ID")}

	// 第三步:校验跨字段约束和运行边界,把不安全的部署配置挡在启动阶段。
	// school 形态必须显式给固定租户 ID。
	if c.Deploy.Mode == "school" && c.Deploy.SchoolTenantID == 0 {
		errs = append(errs, "DEPLOY_MODE=school 时必须设置 SCHOOL_TENANT_ID")
	}
	if c.Auth.ServiceAuthMaxSkewSeconds <= 0 {
		errs = append(errs, "SERVICE_AUTH_MAX_SKEW_SECONDS 必须大于 0")
	}
	if c.Server.HealthTimeoutSeconds <= 0 {
		errs = append(errs, "HEALTH_CHECK_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Server.ShutdownTimeoutSeconds <= 0 {
		errs = append(errs, "HTTP_SHUTDOWN_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Redis.PingTimeoutSeconds <= 0 {
		errs = append(errs, "REDIS_PING_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.NATS.ReconnectWaitSeconds <= 0 {
		errs = append(errs, "NATS_RECONNECT_WAIT_SECONDS 必须大于 0")
	}
	if c.MinIO.PingTimeoutSeconds <= 0 {
		errs = append(errs, "MINIO_PING_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Contest.VulnSourceTimeoutSeconds < 1 || c.Contest.VulnSourceTimeoutSeconds > 60 {
		errs = append(errs, "CONTEST_VULN_SOURCE_TIMEOUT_SECONDS 必须在 1 到 60 秒之间")
	}
	if c.Judge.ResultDetailsMaxBytes <= 0 {
		errs = append(errs, "JUDGE_RESULT_DETAILS_MAX_BYTES 必须大于 0")
	}
	if c.Judge.InputInjectTimeoutSeconds <= 0 {
		errs = append(errs, "JUDGE_INPUT_INJECT_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Sandbox.InitArchiveMaxFiles <= 0 {
		errs = append(errs, "SANDBOX_INIT_ARCHIVE_MAX_FILES 必须大于 0")
	}
	if c.Sandbox.InitArchiveMaxUnpackedBytes <= 0 {
		errs = append(errs, "SANDBOX_INIT_ARCHIVE_MAX_UNPACKED_BYTES 必须大于 0")
	}
	if c.Sandbox.ProbeDefaultPeriodSeconds <= 0 {
		errs = append(errs, "SANDBOX_PROBE_DEFAULT_PERIOD_SECONDS 必须大于 0")
	}
	if c.Sandbox.ProbeDefaultFailureThreshold <= 0 {
		errs = append(errs, "SANDBOX_PROBE_DEFAULT_FAILURE_THRESHOLD 必须大于 0")
	}
	if c.Sandbox.RecyclePollIntervalSeconds <= 0 {
		errs = append(errs, "SANDBOX_RECYCLE_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	if c.Sandbox.RecycleBatchSize <= 0 {
		errs = append(errs, "SANDBOX_RECYCLE_BATCH_SIZE 必须大于 0")
	}
	if c.Sandbox.ReadyIdleTimeoutSeconds <= 0 {
		errs = append(errs, "SANDBOX_READY_IDLE_TIMEOUT_SECONDS 必须大于 0")
	}
	if len(c.Identity.SSOAllowedServiceOrigins) == 0 {
		errs = append(errs, "IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS 至少配置一个平台 CAS 回调 origin")
	}
	for _, origin := range c.Identity.SSOAllowedServiceOrigins {
		if !validOrigin(origin) {
			errs = append(errs, fmt.Sprintf("IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS 包含非法 origin: %s", origin))
		}
	}

	// 第四步:统一返回所有配置问题,便于部署一次修完而不是逐项重启试错。
	if len(errs) > 0 {
		return nil, fmt.Errorf("配置加载失败:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return c, nil
}

// validOrigin 校验 scheme+host 形式的 origin,避免把完整路径或坏 URL 当作安全白名单。
func validOrigin(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme != "" && u.Host != "" && u.Path == "" && u.RawQuery == "" && u.Fragment == ""
}

// readSandboxImageAttestations 解析受控镜像证明清单;空清单会使镜像登记全部失败。
func readSandboxImageAttestations(key string, errs *[]string) []SandboxImageAttestation {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		*errs = append(*errs, "缺少必填环境变量: "+key)
		return nil
	}
	var out []SandboxImageAttestation
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		*errs = append(*errs, fmt.Sprintf("环境变量 %s 需为镜像证明 JSON 数组: %v", key, err))
		return nil
	}
	if len(out) == 0 {
		*errs = append(*errs, "环境变量 "+key+" 至少包含一个镜像证明")
	}
	for i, item := range out {
		if strings.TrimSpace(item.ImageURL) == "" || strings.TrimSpace(item.Digest) == "" || strings.TrimSpace(item.TrivyStatus) == "" {
			*errs = append(*errs, fmt.Sprintf("环境变量 %s 第 %d 项镜像证明不完整", key, i))
		}
	}
	return out
}

// validateSandboxQuantities 在启动边界校验 K8s quantity,避免请求路径触发资源解析 panic。
func validateSandboxQuantities(cfg SandboxConfig) []string {
	values := map[string]string{
		"SANDBOX_DEFAULT_CPU":            cfg.DefaultCPU,
		"SANDBOX_DEFAULT_MEMORY":         cfg.DefaultMemory,
		"SANDBOX_DEFAULT_REQUEST_CPU":    cfg.DefaultReqCPU,
		"SANDBOX_DEFAULT_REQUEST_MEMORY": cfg.DefaultReqMemory,
		"SANDBOX_MAX_CPU":                cfg.MaxCPU,
		"SANDBOX_MAX_MEMORY":             cfg.MaxMemory,
		"SANDBOX_MAX_PODS":               cfg.MaxPods,
		"SANDBOX_WORKSPACE_STORAGE":      cfg.WorkspaceStorage,
		"SANDBOX_PREPULL_REQUEST_CPU":    cfg.PrepullRequestCPU,
		"SANDBOX_PREPULL_REQUEST_MEMORY": cfg.PrepullRequestMemory,
		"SANDBOX_PREPULL_LIMIT_CPU":      cfg.PrepullLimitCPU,
		"SANDBOX_PREPULL_LIMIT_MEMORY":   cfg.PrepullLimitMemory,
	}
	var errs []string
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, "环境变量 "+key+" 不能为空")
			continue
		}
		if _, err := resource.ParseQuantity(value); err != nil {
			errs = append(errs, fmt.Sprintf("环境变量 %s 需为 Kubernetes quantity,实际=%q", key, value))
		}
	}
	return errs
}
