// config 统一从环境变量加载后端与平台基础设施所需配置,启动期一次性校验并 fail-fast。
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

// Config 是后端运行所需的全部配置集合。
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
	Transfer   TransferConfig
	Contest    ContestConfig
	Notify     NotifyConfig
	Teaching   TeachingConfig
	Experiment ExperimentConfig
	Grade      GradeConfig
	Admin      AdminConfig
	Sandbox    SandboxConfig
	Judge      JudgeConfig
	Monitoring MonitoringConfig
	Snowflake  SnowflakeConfig
}

// DeployConfig 描述部署形态和平台管理员层开关。
type DeployConfig struct {
	Mode            string
	PlatformEnabled bool
	SchoolTenantID  int64
}

// ServerConfig 描述 HTTP、WebSocket 与日志运行边界。
type ServerConfig struct {
	Addr                     string
	Port                     int
	WSPath                   string
	WSAllowedOrigins         []string
	LogLevel                 string
	LogFormat                string
	AppEnv                   string
	HealthTimeoutSeconds     int
	ShutdownTimeoutSeconds   int
	ReadHeaderTimeoutSeconds int
	WSReadTimeoutSeconds     int
	WSWriteTimeoutSeconds    int
	WSPingIntervalSeconds    int
	WSReadLimitBytes         int64
}

// PostgresConfig 描述 PostgreSQL 连接池和可选特权连接。
type PostgresConfig struct {
	Host                string
	Port                int
	Database            string
	User                string
	Password            string
	SSLMode             string
	MaxConns            int
	MinConns            int
	PrivUser            string
	PrivPassword        string
	GrantTimeoutSeconds int
}

// RedisConfig 描述 Redis 连接和探测超时。
type RedisConfig struct {
	Host               string
	Port               int
	Password           string
	DB                 int
	PingTimeoutSeconds int
}

// NATSConfig 描述事件总线连接参数。
type NATSConfig struct {
	URL                  string
	Token                string
	ReconnectWaitSeconds int
	ConsumerRetryMax     int
	ConsumerRetryDelayMs int
	DeadLetterPrefix     string
}

// MinIOConfig 描述对象存储连接和桶命名。
type MinIOConfig struct {
	Endpoint                string
	UseSSL                  bool
	Region                  string
	AccessKey               string
	SecretKey               string
	BucketCode              string
	BucketAttach            string
	BucketReport            string
	BucketBackup            string
	PingTimeoutSeconds      int
	DownloadGrantTTLSeconds int
}

// AuthConfig 描述 JWT、服务签名和敏感数据加密密钥。
type AuthConfig struct {
	JWTSigningKey             string
	AccessTTLMin              int
	RefreshTTLDay             int
	JWTIssuer                 string
	EncryptionKey             string
	HMACKey                   string
	ServiceAuthMaxSkewSeconds int
}

// BootstrapConfig 描述 migrate/seed 所需的首个租户与管理员参数。
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

// IdentityConfig 描述身份模块的安全和网络运行边界。
type IdentityConfig struct {
	ActivationCodeTTLHours   int
	SSONetworkTimeoutSeconds int
	SSOCASResponseMaxBytes   int64
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

// SMSConfig 描述短信网关接入边界。
type SMSConfig struct {
	Provider       string
	Endpoint       string
	Token          string
	LoginTemplate  string
	ResetTemplate  string
	ChangeTemplate string
	TimeoutSeconds int
}

// UploadConfig 描述统一上传边界。
type UploadConfig struct {
	ImportMaxBytes              int64
	ContentAttachmentMaxBytes   int64
	SimBundleMaxBytes           int64
	SimBundleMaxFiles           int
	SimBundleMaxUnpackedBytes   int64
	SimValidationReportMaxBytes int64
	VirusScanRequired           bool
	VirusScanNetwork            string
	VirusScanAddress            string
	VirusScanTimeoutSeconds     int
	VirusScanMaxBytes           int64
}

// TransferConfig 描述统一导入导出中心的任务重试与下载中心边界。
type TransferConfig struct {
	TaskMaxAttempts        int
	TaskRetryDelayMs       int
	TaskDownloadTTLSeconds int
}

// ContestConfig 描述竞赛外部源访问边界。
type ContestConfig struct {
	VulnSourceMaxResponseBytes    int64
	VulnSourceTimeoutSeconds      int
	MatchmakerPollIntervalSeconds int
	MatchmakerBatchSize           int
	SubmitRateLimitSeconds        int
	FailedCooldownSeconds         int
	BattleELOInitialScore         float64
	BattleELOKFactor              float64
}

// NotifyConfig 描述通知事件消费和限频边界。
type NotifyConfig struct {
	EventRetryMax          int
	EventRetryDelayMs      int
	UnreadTTLHours         int
	SendRateWindowSeconds  int
	SendRateMax            int
	RetentionDays          int
	CleanupIntervalSeconds int
}

// TeachingConfig 描述教学跨模块读取与后台任务批量边界。
type TeachingConfig struct {
	CourseGradesMaxRows             int
	CourseStatusPollIntervalSeconds int
	JudgeOutboxBatchSize            int
	JudgeOutboxPollIntervalMs       int
	GradeEventOutboxBatchSize       int
	GradeEventOutboxPollMs          int
	GradeEventOutboxStaleMs         int
	GradeExportBatchSize            int
}

// ExperimentConfig 描述实验实例生命周期与报告批改的后台边界。
type ExperimentConfig struct {
	RecyclePollIntervalSeconds int
	RecycleBatchSize           int
	InstanceIdleTimeoutSeconds int
	PausedTimeoutSeconds       int
	ScoreOutboxBatchSize       int
	ScoreOutboxPollMs          int
	ScoreOutboxStaleMs         int
}

// GradeConfig 描述成绩中心申诉和成绩单签名边界。
type GradeConfig struct {
	AppealWindowDays     int
	TranscriptSigningKey string
	TranscriptMaxBytes   int64
	LockOutboxBatchSize  int
	LockOutboxPollMs     int
	LockOutboxStaleMs    int
}

// AdminConfig 描述管理后台统计快照后台任务边界。
type AdminConfig struct {
	StatisticsSnapshotIntervalSeconds int
}

// SandboxConfig 描述 K8s 沙箱编排与镜像证明边界。
type SandboxConfig struct {
	KubeconfigPath                string
	NSPrefixStudent               string
	NSPrefixJudge                 string
	NSPrefixBattle                string
	PrepullNamespace              string
	SandboxNodeSelector           map[string]string
	SandboxNodeTolerations        []SandboxToleration
	ImageRegistry                 string
	ImageAttestations             []SandboxImageAttestation
	CollectorAllowedPrefixes      []string
	DefaultCPU                    string
	DefaultMemory                 string
	DefaultReqCPU                 string
	DefaultReqMemory              string
	MaxCPU                        string
	MaxMemory                     string
	MaxPods                       string
	WorkspaceStorage              string
	StorageClassName              string
	VolumeSnapshotClassName       string
	PrepullTimeoutSeconds         int
	PrepullHoldSeconds            int
	ReadyTimeoutSeconds           int
	PrepullPollIntervalSeconds    int
	ReadyPollIntervalSeconds      int
	PrepullRequestCPU             string
	PrepullRequestMemory          string
	PrepullLimitCPU               string
	PrepullLimitMemory            string
	ChainRPCTimeoutSeconds        int
	ExecTimeoutSeconds            int
	InitArchiveMaxBytes           int64
	InitArchiveMaxFiles           int
	InitArchiveMaxUnpackedBytes   int64
	FileSaveDebounceMs            int
	ProbeDefaultPeriodSeconds     int32
	ProbeDefaultFailureThreshold  int32
	RecyclePollIntervalSeconds    int
	RecycleBatchSize              int
	RecycleOutboxBatchSize        int
	RecycleOutboxPollMs           int
	RecycleOutboxStaleMs          int
	ReadyIdleTimeoutSeconds       int
	SelftestRecycleTimeoutSeconds int
	ControlNamespace              string
	ControlPodLabelKey            string
	ControlPodLabelValue          string
}

// SandboxToleration 描述沙箱工作负载允许调度到带污点节点的最小配置。
type SandboxToleration struct {
	Key               string `json:"key"`
	Operator          string `json:"operator"`
	Value             string `json:"value"`
	Effect            string `json:"effect"`
	TolerationSeconds *int64 `json:"toleration_seconds"`
}

// SandboxImageAttestation 描述一条受控镜像的签名与扫描证明。
type SandboxImageAttestation struct {
	ImageURL       string `json:"image_url"`
	Digest         string `json:"digest"`
	CosignVerified bool   `json:"cosign_verified"`
	TrivyStatus    string `json:"trivy_status"`
}

// JudgeConfig 描述判题队列、结果大小和归档边界。
type JudgeConfig struct {
	QueuePollIntervalMs          int
	WorkerBatchSize              int
	SubmitRateLimitSec           int
	DefaultMaxRetries            int
	SandboxReadyPollIntervalMs   int
	SandboxReadyGraceSeconds     int
	ResultDetailsMaxBytes        int
	InputInjectTimeoutSeconds    int
	InputArchiveMaxFiles         int
	InputArchiveMaxUnpackedBytes int64
	SimilarityDefaultThreshold   float64
}

// MonitoringConfig 描述外接监控面板入口。
type MonitoringConfig struct {
	PanelsJSON string
}

// SnowflakeConfig 描述雪花 ID 节点编号。
type SnowflakeConfig struct {
	NodeID int64
}

// Load 从环境变量装载并校验配置;任何必填项缺失或格式错误都统一返回。
func Load() (*Config, error) {
	c := &Config{}
	var errs []string

	// 第一步:统一准备必填/可选读取器,把格式错误收敛到一次启动失败里返回。
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
	reqFloat64 := func(key string) float64 {
		v := os.Getenv(key)
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			errs = append(errs, fmt.Sprintf("环境变量 %s 需为 float64,实际=%q", key, v))
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
		v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		switch v {
		case "true", "1", "yes", "y", "on":
			return true
		case "false", "0", "no", "n", "off":
			return false
		default:
			errs = append(errs, fmt.Sprintf("环境变量 %s 需为布尔值,实际=%q", key, os.Getenv(key)))
			return false
		}
	}

	// 第二步:按配置域分组装载环境变量,保持 deploy/server/db/... 的职责边界清晰。
	c.Deploy = DeployConfig{
		Mode:            req("DEPLOY_MODE"),
		PlatformEnabled: reqBool("PLATFORM_LAYER_ENABLED"),
		SchoolTenantID:  optInt64("SCHOOL_TENANT_ID"),
	}
	c.Server = ServerConfig{
		Addr:                     req("HTTP_ADDR"),
		Port:                     reqInt("HTTP_PORT"),
		WSPath:                   req("WS_PATH"),
		WSAllowedOrigins:         getCSV("WS_ALLOWED_ORIGINS"),
		LogLevel:                 req("LOG_LEVEL"),
		LogFormat:                req("LOG_FORMAT"),
		AppEnv:                   req("APP_ENV"),
		HealthTimeoutSeconds:     reqInt("HEALTH_CHECK_TIMEOUT_SECONDS"),
		ShutdownTimeoutSeconds:   reqInt("HTTP_SHUTDOWN_TIMEOUT_SECONDS"),
		ReadHeaderTimeoutSeconds: reqInt("HTTP_READ_HEADER_TIMEOUT_SECONDS"),
		WSReadTimeoutSeconds:     reqInt("WS_READ_TIMEOUT_SECONDS"),
		WSWriteTimeoutSeconds:    reqInt("WS_WRITE_TIMEOUT_SECONDS"),
		WSPingIntervalSeconds:    reqInt("WS_PING_INTERVAL_SECONDS"),
		WSReadLimitBytes:         reqInt64("WS_READ_LIMIT_BYTES"),
	}
	c.Postgres = PostgresConfig{
		Host:                req("PG_HOST"),
		Port:                reqInt("PG_PORT"),
		Database:            req("PG_DATABASE"),
		User:                req("PG_USER"),
		Password:            req("PG_PASSWORD"),
		SSLMode:             req("PG_SSLMODE"),
		MaxConns:            reqInt("PG_MAX_CONNS"),
		MinConns:            reqInt("PG_MIN_CONNS"),
		PrivUser:            os.Getenv("PG_PRIV_USER"),
		PrivPassword:        os.Getenv("PG_PRIV_PASSWORD"),
		GrantTimeoutSeconds: reqInt("PG_GRANT_TIMEOUT_SECONDS"),
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
		ConsumerRetryMax:     reqInt("NATS_CONSUMER_RETRY_MAX"),
		ConsumerRetryDelayMs: reqInt("NATS_CONSUMER_RETRY_DELAY_MS"),
		DeadLetterPrefix:     req("NATS_DEAD_LETTER_PREFIX"),
	}
	c.MinIO = MinIOConfig{
		Endpoint:                req("MINIO_ENDPOINT"),
		UseSSL:                  reqBool("MINIO_USE_SSL"),
		Region:                  req("MINIO_REGION"),
		AccessKey:               req("MINIO_ACCESS_KEY"),
		SecretKey:               req("MINIO_SECRET_KEY"),
		BucketCode:              req("MINIO_BUCKET_CODE"),
		BucketAttach:            req("MINIO_BUCKET_ATTACHMENT"),
		BucketReport:            req("MINIO_BUCKET_REPORT"),
		BucketBackup:            req("MINIO_BUCKET_BACKUP"),
		PingTimeoutSeconds:      reqInt("MINIO_PING_TIMEOUT_SECONDS"),
		DownloadGrantTTLSeconds: reqInt("STORAGE_DOWNLOAD_GRANT_TTL_SECONDS"),
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
		SSOCASResponseMaxBytes:   reqInt64("IDENTITY_SSO_CAS_RESPONSE_MAX_BYTES"),
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
		ImportMaxBytes:              reqInt64("UPLOAD_IMPORT_MAX_BYTES"),
		ContentAttachmentMaxBytes:   reqInt64("UPLOAD_CONTENT_ATTACHMENT_MAX_BYTES"),
		SimBundleMaxBytes:           reqInt64("UPLOAD_SIM_BUNDLE_MAX_BYTES"),
		SimBundleMaxFiles:           reqInt("UPLOAD_SIM_BUNDLE_MAX_FILES"),
		SimBundleMaxUnpackedBytes:   reqInt64("UPLOAD_SIM_BUNDLE_MAX_UNPACKED_BYTES"),
		SimValidationReportMaxBytes: reqInt64("UPLOAD_SIM_VALIDATION_REPORT_MAX_BYTES"),
		VirusScanRequired:           reqBool("UPLOAD_VIRUS_SCAN_REQUIRED"),
		VirusScanNetwork:            os.Getenv("UPLOAD_VIRUS_SCAN_NETWORK"),
		VirusScanAddress:            os.Getenv("UPLOAD_VIRUS_SCAN_ADDRESS"),
		VirusScanTimeoutSeconds:     reqInt("UPLOAD_VIRUS_SCAN_TIMEOUT_SECONDS"),
		VirusScanMaxBytes:           reqInt64("UPLOAD_VIRUS_SCAN_MAX_BYTES"),
	}
	c.Transfer = TransferConfig{
		TaskMaxAttempts:        reqInt("TRANSFER_TASK_MAX_ATTEMPTS"),
		TaskRetryDelayMs:       reqInt("TRANSFER_TASK_RETRY_DELAY_MS"),
		TaskDownloadTTLSeconds: reqInt("TRANSFER_TASK_DOWNLOAD_TTL_SECONDS"),
	}
	c.Contest = ContestConfig{
		VulnSourceMaxResponseBytes:    reqInt64("CONTEST_VULN_SOURCE_MAX_RESPONSE_BYTES"),
		VulnSourceTimeoutSeconds:      reqInt("CONTEST_VULN_SOURCE_TIMEOUT_SECONDS"),
		MatchmakerPollIntervalSeconds: reqInt("CONTEST_MATCHMAKER_POLL_INTERVAL_SECONDS"),
		MatchmakerBatchSize:           reqInt("CONTEST_MATCHMAKER_BATCH_SIZE"),
		SubmitRateLimitSeconds:        reqInt("CONTEST_SUBMIT_RATE_LIMIT_SECONDS"),
		FailedCooldownSeconds:         reqInt("CONTEST_FAILED_COOLDOWN_SECONDS"),
		BattleELOInitialScore:         reqFloat64("CONTEST_BATTLE_ELO_INITIAL_SCORE"),
		BattleELOKFactor:              reqFloat64("CONTEST_BATTLE_ELO_K_FACTOR"),
	}
	c.Notify = NotifyConfig{
		EventRetryMax:          reqInt("NOTIFY_EVENT_RETRY_MAX"),
		EventRetryDelayMs:      reqInt("NOTIFY_EVENT_RETRY_DELAY_MS"),
		UnreadTTLHours:         reqInt("NOTIFY_UNREAD_TTL_HOURS"),
		SendRateWindowSeconds:  reqInt("NOTIFY_SEND_RATE_WINDOW_SECONDS"),
		SendRateMax:            reqInt("NOTIFY_SEND_RATE_MAX"),
		RetentionDays:          reqInt("NOTIFY_RETENTION_DAYS"),
		CleanupIntervalSeconds: reqInt("NOTIFY_CLEANUP_INTERVAL_SECONDS"),
	}
	c.Teaching = TeachingConfig{
		CourseGradesMaxRows:             reqInt("TEACHING_COURSE_GRADES_MAX_ROWS"),
		CourseStatusPollIntervalSeconds: reqInt("TEACHING_COURSE_STATUS_POLL_INTERVAL_SECONDS"),
		JudgeOutboxBatchSize:            reqInt("TEACHING_JUDGE_OUTBOX_BATCH_SIZE"),
		JudgeOutboxPollIntervalMs:       reqInt("TEACHING_JUDGE_OUTBOX_POLL_INTERVAL_MS"),
		GradeEventOutboxBatchSize:       reqInt("TEACHING_GRADE_EVENT_OUTBOX_BATCH_SIZE"),
		GradeEventOutboxPollMs:          reqInt("TEACHING_GRADE_EVENT_OUTBOX_POLL_INTERVAL_MS"),
		GradeEventOutboxStaleMs:         reqInt("TEACHING_GRADE_EVENT_OUTBOX_STALE_INTERVAL_MS"),
		GradeExportBatchSize:            reqInt("TEACHING_GRADE_EXPORT_BATCH_SIZE"),
	}
	c.Experiment = ExperimentConfig{
		RecyclePollIntervalSeconds: reqInt("EXPERIMENT_RECYCLE_POLL_INTERVAL_SECONDS"),
		RecycleBatchSize:           reqInt("EXPERIMENT_RECYCLE_BATCH_SIZE"),
		InstanceIdleTimeoutSeconds: reqInt("EXPERIMENT_INSTANCE_IDLE_TIMEOUT_SECONDS"),
		PausedTimeoutSeconds:       reqInt("EXPERIMENT_PAUSED_TIMEOUT_SECONDS"),
		ScoreOutboxBatchSize:       reqInt("EXPERIMENT_SCORE_OUTBOX_BATCH_SIZE"),
		ScoreOutboxPollMs:          reqInt("EXPERIMENT_SCORE_OUTBOX_POLL_INTERVAL_MS"),
		ScoreOutboxStaleMs:         reqInt("EXPERIMENT_SCORE_OUTBOX_STALE_INTERVAL_MS"),
	}
	c.Grade = GradeConfig{
		AppealWindowDays:     reqInt("GRADE_APPEAL_WINDOW_DAYS"),
		TranscriptSigningKey: req("GRADE_TRANSCRIPT_SIGNING_KEY"),
		TranscriptMaxBytes:   reqInt64("GRADE_TRANSCRIPT_MAX_BYTES"),
		LockOutboxBatchSize:  reqInt("GRADE_LOCK_OUTBOX_BATCH_SIZE"),
		LockOutboxPollMs:     reqInt("GRADE_LOCK_OUTBOX_POLL_INTERVAL_MS"),
		LockOutboxStaleMs:    reqInt("GRADE_LOCK_OUTBOX_STALE_INTERVAL_MS"),
	}
	c.Admin = AdminConfig{
		StatisticsSnapshotIntervalSeconds: reqInt("ADMIN_STATISTICS_SNAPSHOT_INTERVAL_SECONDS"),
	}
	c.Sandbox = SandboxConfig{
		KubeconfigPath:                os.Getenv("KUBECONFIG_PATH"),
		NSPrefixStudent:               req("SANDBOX_NS_PREFIX_STUDENT"),
		NSPrefixJudge:                 req("SANDBOX_NS_PREFIX_JUDGE"),
		NSPrefixBattle:                req("SANDBOX_NS_PREFIX_BATTLE"),
		PrepullNamespace:              req("SANDBOX_PREPULL_NAMESPACE"),
		SandboxNodeSelector:           getKeyValueMap("SANDBOX_NODE_SELECTOR", &errs),
		SandboxNodeTolerations:        readSandboxTolerations("SANDBOX_NODE_TOLERATIONS_JSON", &errs),
		ImageRegistry:                 req("IMAGE_REGISTRY"),
		ImageAttestations:             readSandboxImageAttestations("SANDBOX_IMAGE_ATTESTATIONS_JSON", &errs),
		CollectorAllowedPrefixes:      getCSV("CHAIMIR_COLLECTOR_ALLOWED_PREFIXES"),
		DefaultCPU:                    req("SANDBOX_DEFAULT_CPU"),
		DefaultMemory:                 req("SANDBOX_DEFAULT_MEMORY"),
		DefaultReqCPU:                 req("SANDBOX_DEFAULT_REQUEST_CPU"),
		DefaultReqMemory:              req("SANDBOX_DEFAULT_REQUEST_MEMORY"),
		MaxCPU:                        req("SANDBOX_MAX_CPU"),
		MaxMemory:                     req("SANDBOX_MAX_MEMORY"),
		MaxPods:                       req("SANDBOX_MAX_PODS"),
		WorkspaceStorage:              req("SANDBOX_WORKSPACE_STORAGE"),
		StorageClassName:              os.Getenv("SANDBOX_STORAGE_CLASS_NAME"),
		VolumeSnapshotClassName:       os.Getenv("SANDBOX_VOLUME_SNAPSHOT_CLASS_NAME"),
		PrepullTimeoutSeconds:         reqInt("SANDBOX_PREPULL_TIMEOUT_SECONDS"),
		PrepullHoldSeconds:            reqInt("SANDBOX_PREPULL_HOLD_SECONDS"),
		ReadyTimeoutSeconds:           reqInt("SANDBOX_READY_TIMEOUT_SECONDS"),
		PrepullPollIntervalSeconds:    reqInt("SANDBOX_PREPULL_POLL_INTERVAL_SECONDS"),
		ReadyPollIntervalSeconds:      reqInt("SANDBOX_READY_POLL_INTERVAL_SECONDS"),
		PrepullRequestCPU:             req("SANDBOX_PREPULL_REQUEST_CPU"),
		PrepullRequestMemory:          req("SANDBOX_PREPULL_REQUEST_MEMORY"),
		PrepullLimitCPU:               req("SANDBOX_PREPULL_LIMIT_CPU"),
		PrepullLimitMemory:            req("SANDBOX_PREPULL_LIMIT_MEMORY"),
		ChainRPCTimeoutSeconds:        reqInt("SANDBOX_CHAIN_RPC_TIMEOUT_SECONDS"),
		ExecTimeoutSeconds:            reqInt("SANDBOX_EXEC_TIMEOUT_SECONDS"),
		InitArchiveMaxBytes:           reqInt64("SANDBOX_INIT_ARCHIVE_MAX_BYTES"),
		InitArchiveMaxFiles:           reqInt("SANDBOX_INIT_ARCHIVE_MAX_FILES"),
		InitArchiveMaxUnpackedBytes:   reqInt64("SANDBOX_INIT_ARCHIVE_MAX_UNPACKED_BYTES"),
		FileSaveDebounceMs:            reqInt("SANDBOX_FILE_SAVE_DEBOUNCE_MS"),
		ProbeDefaultPeriodSeconds:     int32(reqInt("SANDBOX_PROBE_DEFAULT_PERIOD_SECONDS")),
		ProbeDefaultFailureThreshold:  int32(reqInt("SANDBOX_PROBE_DEFAULT_FAILURE_THRESHOLD")),
		RecyclePollIntervalSeconds:    reqInt("SANDBOX_RECYCLE_POLL_INTERVAL_SECONDS"),
		RecycleBatchSize:              reqInt("SANDBOX_RECYCLE_BATCH_SIZE"),
		RecycleOutboxBatchSize:        reqInt("SANDBOX_RECYCLE_OUTBOX_BATCH_SIZE"),
		RecycleOutboxPollMs:           reqInt("SANDBOX_RECYCLE_OUTBOX_POLL_INTERVAL_MS"),
		RecycleOutboxStaleMs:          reqInt("SANDBOX_RECYCLE_OUTBOX_STALE_INTERVAL_MS"),
		ReadyIdleTimeoutSeconds:       reqInt("SANDBOX_READY_IDLE_TIMEOUT_SECONDS"),
		SelftestRecycleTimeoutSeconds: reqInt("SANDBOX_SELFTEST_RECYCLE_TIMEOUT_SECONDS"),
		ControlNamespace:              req("SANDBOX_CONTROL_NAMESPACE"),
		ControlPodLabelKey:            req("SANDBOX_CONTROL_POD_LABEL_KEY"),
		ControlPodLabelValue:          req("SANDBOX_CONTROL_POD_LABEL_VALUE"),
	}
	errs = append(errs, validateSandboxQuantities(c.Sandbox)...)
	c.Judge = JudgeConfig{
		QueuePollIntervalMs:          reqInt("JUDGE_QUEUE_POLL_INTERVAL_MS"),
		WorkerBatchSize:              reqInt("JUDGE_WORKER_BATCH_SIZE"),
		SubmitRateLimitSec:           reqInt("JUDGE_SUBMIT_RATE_LIMIT_SECONDS"),
		DefaultMaxRetries:            reqInt("JUDGE_DEFAULT_MAX_RETRIES"),
		SandboxReadyPollIntervalMs:   reqInt("JUDGE_SANDBOX_READY_POLL_INTERVAL_MS"),
		SandboxReadyGraceSeconds:     reqInt("JUDGE_SANDBOX_READY_GRACE_SECONDS"),
		ResultDetailsMaxBytes:        reqInt("JUDGE_RESULT_DETAILS_MAX_BYTES"),
		InputInjectTimeoutSeconds:    reqInt("JUDGE_INPUT_INJECT_TIMEOUT_SECONDS"),
		InputArchiveMaxFiles:         reqInt("JUDGE_INPUT_ARCHIVE_MAX_FILES"),
		InputArchiveMaxUnpackedBytes: reqInt64("JUDGE_INPUT_ARCHIVE_MAX_UNPACKED_BYTES"),
		SimilarityDefaultThreshold:   reqFloat64("JUDGE_SIMILARITY_DEFAULT_THRESHOLD"),
	}
	c.Monitoring = MonitoringConfig{
		PanelsJSON: req("MONITORING_PANELS_JSON"),
	}
	c.Snowflake = SnowflakeConfig{
		NodeID: reqInt64("SNOWFLAKE_NODE_ID"),
	}

	// 第三步:集中执行跨字段约束和安全边界校验,避免运行时才暴露配置问题。
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
	if c.Server.ReadHeaderTimeoutSeconds <= 0 {
		errs = append(errs, "HTTP_READ_HEADER_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Server.WSReadTimeoutSeconds <= 0 {
		errs = append(errs, "WS_READ_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Server.WSWriteTimeoutSeconds <= 0 {
		errs = append(errs, "WS_WRITE_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Server.WSPingIntervalSeconds <= 0 {
		errs = append(errs, "WS_PING_INTERVAL_SECONDS 必须大于 0")
	}
	if c.Server.WSReadLimitBytes <= 0 {
		errs = append(errs, "WS_READ_LIMIT_BYTES 必须大于 0")
	}
	if c.Redis.PingTimeoutSeconds <= 0 {
		errs = append(errs, "REDIS_PING_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Postgres.GrantTimeoutSeconds <= 0 {
		errs = append(errs, "PG_GRANT_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.NATS.ReconnectWaitSeconds <= 0 {
		errs = append(errs, "NATS_RECONNECT_WAIT_SECONDS 必须大于 0")
	}
	if c.NATS.ConsumerRetryMax <= 0 {
		errs = append(errs, "NATS_CONSUMER_RETRY_MAX 必须大于 0")
	}
	if c.NATS.ConsumerRetryDelayMs <= 0 {
		errs = append(errs, "NATS_CONSUMER_RETRY_DELAY_MS 必须大于 0")
	}
	if strings.TrimSpace(c.NATS.DeadLetterPrefix) == "" {
		errs = append(errs, "NATS_DEAD_LETTER_PREFIX 不能为空")
	}
	if c.MinIO.PingTimeoutSeconds <= 0 {
		errs = append(errs, "MINIO_PING_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.MinIO.DownloadGrantTTLSeconds <= 0 {
		errs = append(errs, "STORAGE_DOWNLOAD_GRANT_TTL_SECONDS 必须大于 0")
	}
	if c.Upload.VirusScanTimeoutSeconds <= 0 {
		errs = append(errs, "UPLOAD_VIRUS_SCAN_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Upload.VirusScanMaxBytes <= 0 {
		errs = append(errs, "UPLOAD_VIRUS_SCAN_MAX_BYTES 必须大于 0")
	}
	if c.Upload.ContentAttachmentMaxBytes <= 0 {
		errs = append(errs, "UPLOAD_CONTENT_ATTACHMENT_MAX_BYTES 必须大于 0")
	}
	if c.Upload.SimBundleMaxBytes <= 0 {
		errs = append(errs, "UPLOAD_SIM_BUNDLE_MAX_BYTES 必须大于 0")
	}
	if c.Upload.SimBundleMaxFiles <= 0 {
		errs = append(errs, "UPLOAD_SIM_BUNDLE_MAX_FILES 必须大于 0")
	}
	if c.Upload.SimBundleMaxUnpackedBytes <= 0 {
		errs = append(errs, "UPLOAD_SIM_BUNDLE_MAX_UNPACKED_BYTES 必须大于 0")
	}
	if c.Upload.SimValidationReportMaxBytes <= 0 {
		errs = append(errs, "UPLOAD_SIM_VALIDATION_REPORT_MAX_BYTES 必须大于 0")
	}
	if c.Upload.VirusScanRequired && strings.TrimSpace(c.Upload.VirusScanAddress) == "" {
		errs = append(errs, "UPLOAD_VIRUS_SCAN_REQUIRED=true 时必须设置 UPLOAD_VIRUS_SCAN_ADDRESS")
	}
	if c.Transfer.TaskMaxAttempts <= 0 {
		errs = append(errs, "TRANSFER_TASK_MAX_ATTEMPTS 必须大于 0")
	}
	if c.Transfer.TaskRetryDelayMs <= 0 {
		errs = append(errs, "TRANSFER_TASK_RETRY_DELAY_MS 必须大于 0")
	}
	if c.Transfer.TaskDownloadTTLSeconds <= 0 {
		errs = append(errs, "TRANSFER_TASK_DOWNLOAD_TTL_SECONDS 必须大于 0")
	}
	if c.Contest.VulnSourceTimeoutSeconds < 1 || c.Contest.VulnSourceTimeoutSeconds > 60 {
		errs = append(errs, "CONTEST_VULN_SOURCE_TIMEOUT_SECONDS 必须在 1 到 60 秒之间")
	}
	if c.Contest.MatchmakerPollIntervalSeconds <= 0 {
		errs = append(errs, "CONTEST_MATCHMAKER_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	if c.Contest.MatchmakerBatchSize <= 0 {
		errs = append(errs, "CONTEST_MATCHMAKER_BATCH_SIZE 必须大于 0")
	}
	if c.Contest.SubmitRateLimitSeconds <= 0 {
		errs = append(errs, "CONTEST_SUBMIT_RATE_LIMIT_SECONDS 必须大于 0")
	}
	if c.Contest.FailedCooldownSeconds <= 0 {
		errs = append(errs, "CONTEST_FAILED_COOLDOWN_SECONDS 必须大于 0")
	}
	if c.Contest.BattleELOInitialScore <= 0 {
		errs = append(errs, "CONTEST_BATTLE_ELO_INITIAL_SCORE 必须大于 0")
	}
	if c.Contest.BattleELOKFactor <= 0 {
		errs = append(errs, "CONTEST_BATTLE_ELO_K_FACTOR 必须大于 0")
	}
	if c.Notify.EventRetryMax <= 0 {
		errs = append(errs, "NOTIFY_EVENT_RETRY_MAX 必须大于 0")
	}
	if c.Notify.EventRetryDelayMs <= 0 {
		errs = append(errs, "NOTIFY_EVENT_RETRY_DELAY_MS 必须大于 0")
	}
	if c.Notify.UnreadTTLHours <= 0 {
		errs = append(errs, "NOTIFY_UNREAD_TTL_HOURS 必须大于 0")
	}
	if c.Notify.SendRateWindowSeconds <= 0 {
		errs = append(errs, "NOTIFY_SEND_RATE_WINDOW_SECONDS 必须大于 0")
	}
	if c.Notify.SendRateMax <= 0 {
		errs = append(errs, "NOTIFY_SEND_RATE_MAX 必须大于 0")
	}
	if c.Notify.RetentionDays <= 0 {
		errs = append(errs, "NOTIFY_RETENTION_DAYS 必须大于 0")
	}
	if c.Notify.CleanupIntervalSeconds <= 0 {
		errs = append(errs, "NOTIFY_CLEANUP_INTERVAL_SECONDS 必须大于 0")
	}
	if c.Teaching.CourseGradesMaxRows <= 0 {
		errs = append(errs, "TEACHING_COURSE_GRADES_MAX_ROWS 必须大于 0")
	}
	if c.Teaching.CourseStatusPollIntervalSeconds <= 0 {
		errs = append(errs, "TEACHING_COURSE_STATUS_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	if c.Teaching.JudgeOutboxBatchSize <= 0 {
		errs = append(errs, "TEACHING_JUDGE_OUTBOX_BATCH_SIZE 必须大于 0")
	}
	if c.Teaching.JudgeOutboxPollIntervalMs <= 0 {
		errs = append(errs, "TEACHING_JUDGE_OUTBOX_POLL_INTERVAL_MS 必须大于 0")
	}
	if c.Teaching.GradeEventOutboxBatchSize <= 0 {
		errs = append(errs, "TEACHING_GRADE_EVENT_OUTBOX_BATCH_SIZE 必须大于 0")
	}
	if c.Teaching.GradeEventOutboxPollMs <= 0 {
		errs = append(errs, "TEACHING_GRADE_EVENT_OUTBOX_POLL_INTERVAL_MS 必须大于 0")
	}
	if c.Teaching.GradeEventOutboxStaleMs <= 0 {
		errs = append(errs, "TEACHING_GRADE_EVENT_OUTBOX_STALE_INTERVAL_MS 必须大于 0")
	}
	if c.Teaching.GradeExportBatchSize <= 0 {
		errs = append(errs, "TEACHING_GRADE_EXPORT_BATCH_SIZE 必须大于 0")
	}
	if c.Experiment.RecyclePollIntervalSeconds <= 0 {
		errs = append(errs, "EXPERIMENT_RECYCLE_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	if c.Experiment.RecycleBatchSize <= 0 {
		errs = append(errs, "EXPERIMENT_RECYCLE_BATCH_SIZE 必须大于 0")
	}
	if c.Experiment.InstanceIdleTimeoutSeconds <= 0 {
		errs = append(errs, "EXPERIMENT_INSTANCE_IDLE_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Experiment.PausedTimeoutSeconds <= 0 {
		errs = append(errs, "EXPERIMENT_PAUSED_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Experiment.ScoreOutboxBatchSize <= 0 {
		errs = append(errs, "EXPERIMENT_SCORE_OUTBOX_BATCH_SIZE 必须大于 0")
	}
	if c.Experiment.ScoreOutboxPollMs <= 0 {
		errs = append(errs, "EXPERIMENT_SCORE_OUTBOX_POLL_INTERVAL_MS 必须大于 0")
	}
	if c.Experiment.ScoreOutboxStaleMs <= 0 {
		errs = append(errs, "EXPERIMENT_SCORE_OUTBOX_STALE_INTERVAL_MS 必须大于 0")
	}
	if c.Grade.TranscriptMaxBytes <= 0 {
		errs = append(errs, "GRADE_TRANSCRIPT_MAX_BYTES 必须大于 0")
	}
	if c.Grade.LockOutboxBatchSize <= 0 {
		errs = append(errs, "GRADE_LOCK_OUTBOX_BATCH_SIZE 必须大于 0")
	}
	if c.Grade.LockOutboxPollMs <= 0 {
		errs = append(errs, "GRADE_LOCK_OUTBOX_POLL_INTERVAL_MS 必须大于 0")
	}
	if c.Grade.LockOutboxStaleMs <= 0 {
		errs = append(errs, "GRADE_LOCK_OUTBOX_STALE_INTERVAL_MS 必须大于 0")
	}
	if c.Judge.QueuePollIntervalMs <= 0 {
		errs = append(errs, "JUDGE_QUEUE_POLL_INTERVAL_MS 必须大于 0")
	}
	if c.Judge.WorkerBatchSize <= 0 {
		errs = append(errs, "JUDGE_WORKER_BATCH_SIZE 必须大于 0")
	}
	if c.Judge.SubmitRateLimitSec <= 0 {
		errs = append(errs, "JUDGE_SUBMIT_RATE_LIMIT_SECONDS 必须大于 0")
	}
	if c.Judge.DefaultMaxRetries < 0 {
		errs = append(errs, "JUDGE_DEFAULT_MAX_RETRIES 不能小于 0")
	}
	if c.Judge.SandboxReadyPollIntervalMs <= 0 {
		errs = append(errs, "JUDGE_SANDBOX_READY_POLL_INTERVAL_MS 必须大于 0")
	}
	if c.Judge.SandboxReadyGraceSeconds <= 0 {
		errs = append(errs, "JUDGE_SANDBOX_READY_GRACE_SECONDS 必须大于 0")
	}
	if c.Judge.ResultDetailsMaxBytes <= 0 {
		errs = append(errs, "JUDGE_RESULT_DETAILS_MAX_BYTES 必须大于 0")
	}
	if c.Judge.InputInjectTimeoutSeconds <= 0 {
		errs = append(errs, "JUDGE_INPUT_INJECT_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Judge.InputArchiveMaxFiles <= 0 {
		errs = append(errs, "JUDGE_INPUT_ARCHIVE_MAX_FILES 必须大于 0")
	}
	if c.Judge.InputArchiveMaxUnpackedBytes <= 0 {
		errs = append(errs, "JUDGE_INPUT_ARCHIVE_MAX_UNPACKED_BYTES 必须大于 0")
	}
	if c.Judge.SimilarityDefaultThreshold <= 0 || c.Judge.SimilarityDefaultThreshold >= 1 {
		errs = append(errs, "JUDGE_SIMILARITY_DEFAULT_THRESHOLD 必须大于 0 且小于 1")
	}
	if c.Admin.StatisticsSnapshotIntervalSeconds <= 0 {
		errs = append(errs, "ADMIN_STATISTICS_SNAPSHOT_INTERVAL_SECONDS 必须大于 0")
	}
	if c.Snowflake.NodeID < 0 || c.Snowflake.NodeID > 1023 {
		errs = append(errs, "SNOWFLAKE_NODE_ID 必须在 0 到 1023 之间,且同一部署内每个后端副本必须唯一")
	}
	if c.Sandbox.PrepullTimeoutSeconds <= 0 {
		errs = append(errs, "SANDBOX_PREPULL_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Sandbox.PrepullHoldSeconds <= 0 {
		errs = append(errs, "SANDBOX_PREPULL_HOLD_SECONDS 必须大于 0")
	}
	if c.Sandbox.ReadyTimeoutSeconds <= 0 {
		errs = append(errs, "SANDBOX_READY_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Sandbox.PrepullPollIntervalSeconds <= 0 {
		errs = append(errs, "SANDBOX_PREPULL_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	if c.Sandbox.ReadyPollIntervalSeconds <= 0 {
		errs = append(errs, "SANDBOX_READY_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	if c.Sandbox.ChainRPCTimeoutSeconds <= 0 {
		errs = append(errs, "SANDBOX_CHAIN_RPC_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Sandbox.ExecTimeoutSeconds <= 0 {
		errs = append(errs, "SANDBOX_EXEC_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Sandbox.InitArchiveMaxBytes <= 0 {
		errs = append(errs, "SANDBOX_INIT_ARCHIVE_MAX_BYTES 必须大于 0")
	}
	if c.Sandbox.InitArchiveMaxFiles <= 0 {
		errs = append(errs, "SANDBOX_INIT_ARCHIVE_MAX_FILES 必须大于 0")
	}
	if c.Sandbox.InitArchiveMaxUnpackedBytes <= 0 {
		errs = append(errs, "SANDBOX_INIT_ARCHIVE_MAX_UNPACKED_BYTES 必须大于 0")
	}
	if c.Sandbox.FileSaveDebounceMs <= 0 {
		errs = append(errs, "SANDBOX_FILE_SAVE_DEBOUNCE_MS 必须大于 0")
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
	if c.Sandbox.RecycleOutboxBatchSize <= 0 {
		errs = append(errs, "SANDBOX_RECYCLE_OUTBOX_BATCH_SIZE 必须大于 0")
	}
	if c.Sandbox.RecycleOutboxPollMs <= 0 {
		errs = append(errs, "SANDBOX_RECYCLE_OUTBOX_POLL_INTERVAL_MS 必须大于 0")
	}
	if c.Sandbox.RecycleOutboxStaleMs <= 0 {
		errs = append(errs, "SANDBOX_RECYCLE_OUTBOX_STALE_INTERVAL_MS 必须大于 0")
	}
	if c.Sandbox.ReadyIdleTimeoutSeconds <= 0 {
		errs = append(errs, "SANDBOX_READY_IDLE_TIMEOUT_SECONDS 必须大于 0")
	}
	if c.Sandbox.SelftestRecycleTimeoutSeconds <= 0 {
		errs = append(errs, "SANDBOX_SELFTEST_RECYCLE_TIMEOUT_SECONDS 必须大于 0")
	}
	if strings.TrimSpace(c.Sandbox.VolumeSnapshotClassName) != "" && strings.TrimSpace(c.Sandbox.StorageClassName) == "" {
		errs = append(errs, "SANDBOX_VOLUME_SNAPSHOT_CLASS_NAME 已配置时必须同时配置 SANDBOX_STORAGE_CLASS_NAME")
	}
	for i, toleration := range c.Sandbox.SandboxNodeTolerations {
		if strings.TrimSpace(toleration.Operator) != "" && toleration.Operator != "Exists" && toleration.Operator != "Equal" {
			errs = append(errs, fmt.Sprintf("SANDBOX_NODE_TOLERATIONS_JSON 第 %d 项 operator 只能为 Exists 或 Equal", i))
		}
		if strings.TrimSpace(toleration.Effect) != "" && toleration.Effect != "NoSchedule" && toleration.Effect != "PreferNoSchedule" && toleration.Effect != "NoExecute" {
			errs = append(errs, fmt.Sprintf("SANDBOX_NODE_TOLERATIONS_JSON 第 %d 项 effect 非法", i))
		}
	}
	for _, prefix := range c.Sandbox.CollectorAllowedPrefixes {
		if !strings.HasPrefix(prefix, "http://") && !strings.HasPrefix(prefix, "https://") {
			errs = append(errs, fmt.Sprintf("CHAIMIR_COLLECTOR_ALLOWED_PREFIXES 包含非法前缀: %s", prefix))
		}
	}
	if len(c.Identity.SSOAllowedServiceOrigins) == 0 {
		errs = append(errs, "IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS 至少配置一个平台 CAS 回调 origin")
	}
	if c.Identity.SSOCASResponseMaxBytes <= 0 {
		errs = append(errs, "IDENTITY_SSO_CAS_RESPONSE_MAX_BYTES 必须大于 0")
	}
	if c.SMS.TimeoutSeconds <= 0 {
		errs = append(errs, "SMS_TIMEOUT_SECONDS 必须大于 0")
	}
	switch strings.ToLower(strings.TrimSpace(c.SMS.Provider)) {
	case "http":
		if strings.TrimSpace(c.SMS.Endpoint) == "" || strings.TrimSpace(c.SMS.Token) == "" {
			errs = append(errs, "SMS_PROVIDER=http 时必须设置 SMS_HTTP_ENDPOINT 和 SMS_HTTP_TOKEN")
		}
		if strings.TrimSpace(c.SMS.LoginTemplate) == "" || strings.TrimSpace(c.SMS.ResetTemplate) == "" || strings.TrimSpace(c.SMS.ChangeTemplate) == "" {
			errs = append(errs, "SMS_PROVIDER=http 时必须配置全部短信模板")
		}
		if u, err := url.Parse(strings.TrimSpace(c.SMS.Endpoint)); err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			errs = append(errs, "SMS_HTTP_ENDPOINT 必须是不含凭据的 HTTP(S) URL")
		}
	case "log":
		if strings.EqualFold(strings.TrimSpace(c.Server.AppEnv), "prod") || strings.EqualFold(strings.TrimSpace(c.Server.AppEnv), "production") {
			errs = append(errs, "生产环境不能使用 SMS_PROVIDER=log")
		}
	default:
		errs = append(errs, "SMS_PROVIDER 只能为 http 或 log")
	}
	for _, origin := range c.Identity.SSOAllowedServiceOrigins {
		if !validOrigin(origin) {
			errs = append(errs, fmt.Sprintf("IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS 包含非法 origin: %s", origin))
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("配置加载失败:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return c, nil
}

// validOrigin 校验 origin 只包含 scheme 和 host,避免把路径误纳入安全白名单。
func validOrigin(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme != "" && u.Host != "" && u.Path == "" && u.RawQuery == "" && u.Fragment == ""
}

// readSandboxImageAttestations 解析镜像证明 JSON 数组,并校验证明字段完整性。
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

// readSandboxTolerations 解析沙箱节点容忍配置;空值表示不声明特殊调度约束。
func readSandboxTolerations(key string, errs *[]string) []SandboxToleration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	var out []SandboxToleration
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		*errs = append(*errs, fmt.Sprintf("环境变量 %s 需为 Kubernetes toleration JSON 数组: %v", key, err))
		return nil
	}
	return out
}

// validateSandboxQuantities 在启动边界校验 Kubernetes quantity,避免编排路径才失败。
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
		// 这里统一在启动期校验 quantity,避免不同编排分支各自报错且口径不一致。
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
