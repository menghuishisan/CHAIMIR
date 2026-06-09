// 上传配置测试:确保文件大小与归档展开限制来自环境变量。
package config

import (
	"strings"
	"testing"
)

// TestLoadReadsUploadLimits 确认上传边界不硬编码在业务模块里。
func TestLoadReadsUploadLimits(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("UPLOAD_IMPORT_MAX_BYTES", "1024")
	t.Setenv("UPLOAD_SIM_BUNDLE_MAX_BYTES", "2048")
	t.Setenv("UPLOAD_SIM_BUNDLE_MAX_FILES", "9")
	t.Setenv("UPLOAD_SIM_BUNDLE_MAX_UNPACKED_BYTES", "4096")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upload.ImportMaxBytes != 1024 ||
		cfg.Upload.SimBundleMaxBytes != 2048 ||
		cfg.Upload.SimBundleMaxFiles != 9 ||
		cfg.Upload.SimBundleMaxUnpackedBytes != 4096 {
		t.Fatalf("unexpected upload config: %#v", cfg.Upload)
	}
}

// TestLoadReadsContestLimits 确认竞赛外部同步边界由 M8 独立配置承载。
func TestLoadReadsContestLimits(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CONTEST_VULN_SOURCE_MAX_RESPONSE_BYTES", "8192")
	t.Setenv("CONTEST_VULN_SOURCE_TIMEOUT_SECONDS", "11")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Contest.VulnSourceMaxResponseBytes != 8192 || cfg.Contest.VulnSourceTimeoutSeconds != 11 {
		t.Fatalf("unexpected contest config: %#v", cfg.Contest)
	}
}

// TestLoadRejectsInvalidContestTimeout 确认漏洞源默认超时不能绕过启动边界。
func TestLoadRejectsInvalidContestTimeout(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CONTEST_VULN_SOURCE_TIMEOUT_SECONDS", "0")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "CONTEST_VULN_SOURCE_TIMEOUT_SECONDS") {
		t.Fatalf("invalid contest timeout must fail config loading, got %v", err)
	}
}

// TestLoadReadsWebSocketAllowedOrigins 确认 WebSocket Origin 白名单由服务配置统一承载。
func TestLoadReadsWebSocketAllowedOrigins(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("WS_ALLOWED_ORIGINS", "https://chaimir.example.edu, https://admin.example.edu")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Server.WSAllowedOrigins) != 2 ||
		cfg.Server.WSAllowedOrigins[0] != "https://chaimir.example.edu" ||
		cfg.Server.WSAllowedOrigins[1] != "https://admin.example.edu" {
		t.Fatalf("unexpected ws origins: %#v", cfg.Server.WSAllowedOrigins)
	}
}

// TestLoadReadsInfrastructureTimeouts 确认基础设施探测和重连阈值全部来自环境变量。
func TestLoadReadsInfrastructureTimeouts(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("HEALTH_CHECK_TIMEOUT_SECONDS", "4")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT_SECONDS", "12")
	t.Setenv("REDIS_PING_TIMEOUT_SECONDS", "6")
	t.Setenv("NATS_RECONNECT_WAIT_SECONDS", "3")
	t.Setenv("MINIO_PING_TIMEOUT_SECONDS", "7")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.HealthTimeoutSeconds != 4 ||
		cfg.Server.ShutdownTimeoutSeconds != 12 ||
		cfg.Redis.PingTimeoutSeconds != 6 ||
		cfg.NATS.ReconnectWaitSeconds != 3 ||
		cfg.MinIO.PingTimeoutSeconds != 7 {
		t.Fatalf("unexpected infrastructure timeouts: server=%#v redis=%#v nats=%#v minio=%#v", cfg.Server, cfg.Redis, cfg.NATS, cfg.MinIO)
	}
}

// TestLoadReadsNotifyEventRetryConfig 确认通知事件消费重试策略由环境配置统一承载。
func TestLoadReadsNotifyEventRetryConfig(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("NOTIFY_EVENT_RETRY_MAX", "5")
	t.Setenv("NOTIFY_EVENT_RETRY_DELAY_MS", "250")
	t.Setenv("NOTIFY_UNREAD_TTL_HOURS", "72")
	t.Setenv("NOTIFY_SEND_RATE_WINDOW_SECONDS", "60")
	t.Setenv("NOTIFY_SEND_RATE_MAX", "30")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Notify.EventRetryMax != 5 || cfg.Notify.EventRetryDelayMs != 250 ||
		cfg.Notify.UnreadTTLHours != 72 || cfg.Notify.SendRateWindowSeconds != 60 || cfg.Notify.SendRateMax != 30 {
		t.Fatalf("unexpected notify config: %#v", cfg.Notify)
	}
}

// TestLoadReadsServiceAuthReplayWindow 确认内部服务签名重放窗口来自环境配置。
func TestLoadReadsServiceAuthReplayWindow(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SERVICE_AUTH_MAX_SKEW_SECONDS", "120")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Auth.ServiceAuthMaxSkewSeconds != 120 {
		t.Fatalf("unexpected auth config: %#v", cfg.Auth)
	}
}

// TestLoadRejectsMissingOperationalLimit 确认运行期阈值缺失时启动失败,不使用代码默认值。
func TestLoadRejectsMissingOperationalLimit(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("UPLOAD_IMPORT_MAX_BYTES", "")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "UPLOAD_IMPORT_MAX_BYTES") {
		t.Fatalf("missing operational limit must fail config loading, got %v", err)
	}
}

// TestLoadReadsJudgeSandboxReadyPollInterval 确认判题等待沙箱就绪的轮询间隔来自统一配置。
func TestLoadReadsJudgeSandboxReadyPollInterval(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JUDGE_SANDBOX_READY_POLL_INTERVAL_MS", "150")
	t.Setenv("JUDGE_RESULT_DETAILS_MAX_BYTES", "32768")
	t.Setenv("JUDGE_INPUT_INJECT_TIMEOUT_SECONDS", "45")
	t.Setenv("JUDGE_INPUT_ARCHIVE_MAX_FILES", "21")
	t.Setenv("JUDGE_INPUT_ARCHIVE_MAX_UNPACKED_BYTES", "4096")
	t.Setenv("JUDGE_SIMILARITY_DEFAULT_THRESHOLD", "0.72")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Judge.SandboxReadyPollIntervalMs != 150 ||
		cfg.Judge.ResultDetailsMaxBytes != 32768 ||
		cfg.Judge.InputInjectTimeoutSeconds != 45 ||
		cfg.Judge.InputArchiveMaxFiles != 21 ||
		cfg.Judge.InputArchiveMaxUnpackedBytes != 4096 ||
		cfg.Judge.SimilarityDefaultThreshold != 0.72 {
		t.Fatalf("unexpected judge config: %#v", cfg.Judge)
	}
}

// TestLoadReadsSandboxPollIntervals 确认 M2 K8s 编排轮询间隔来自统一配置。
func TestLoadReadsSandboxPollIntervals(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SANDBOX_PREPULL_POLL_INTERVAL_SECONDS", "3")
	t.Setenv("SANDBOX_READY_POLL_INTERVAL_SECONDS", "2")
	t.Setenv("SANDBOX_PREPULL_NAMESPACE", "chaimir-prepull-prod")
	t.Setenv("SANDBOX_INIT_ARCHIVE_MAX_FILES", "19")
	t.Setenv("SANDBOX_INIT_ARCHIVE_MAX_UNPACKED_BYTES", "8192")
	t.Setenv("SANDBOX_PROBE_DEFAULT_PERIOD_SECONDS", "4")
	t.Setenv("SANDBOX_PROBE_DEFAULT_FAILURE_THRESHOLD", "12")
	t.Setenv("SANDBOX_RECYCLE_POLL_INTERVAL_SECONDS", "7")
	t.Setenv("SANDBOX_RECYCLE_BATCH_SIZE", "13")
	t.Setenv("SANDBOX_READY_IDLE_TIMEOUT_SECONDS", "600")
	t.Setenv("SANDBOX_SELFTEST_RECYCLE_TIMEOUT_SECONDS", "17")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Sandbox.PrepullPollIntervalSeconds != 3 || cfg.Sandbox.ReadyPollIntervalSeconds != 2 {
		t.Fatalf("unexpected sandbox config: %#v", cfg.Sandbox)
	}
	if cfg.Sandbox.InitArchiveMaxFiles != 19 || cfg.Sandbox.InitArchiveMaxUnpackedBytes != 8192 {
		t.Fatalf("unexpected sandbox init archive config: %#v", cfg.Sandbox)
	}
	if cfg.Sandbox.ProbeDefaultPeriodSeconds != 4 || cfg.Sandbox.ProbeDefaultFailureThreshold != 12 {
		t.Fatalf("unexpected sandbox probe defaults: %#v", cfg.Sandbox)
	}
	if cfg.Sandbox.RecyclePollIntervalSeconds != 7 ||
		cfg.Sandbox.RecycleBatchSize != 13 ||
		cfg.Sandbox.ReadyIdleTimeoutSeconds != 600 ||
		cfg.Sandbox.SelftestRecycleTimeoutSeconds != 17 {
		t.Fatalf("unexpected sandbox recycle scheduler config: %#v", cfg.Sandbox)
	}
	if cfg.Sandbox.PrepullNamespace != "chaimir-prepull-prod" {
		t.Fatalf("unexpected prepull namespace: %s", cfg.Sandbox.PrepullNamespace)
	}
}

// TestLoadReadsSandboxImageAttestations 确认镜像签名与扫描证明来自受控配置清单。
func TestLoadReadsSandboxImageAttestations(t *testing.T) {
	setRequiredEnv(t)
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	t.Setenv("SANDBOX_IMAGE_ATTESTATIONS_JSON", `[{"image_url":"registry/runtime/evm@`+digest+`","digest":"`+digest+`","cosign_verified":true,"trivy_status":"passed"}]`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Sandbox.ImageAttestations) != 1 {
		t.Fatalf("unexpected image attestations: %#v", cfg.Sandbox.ImageAttestations)
	}
	got := cfg.Sandbox.ImageAttestations[0]
	if got.ImageURL != "registry/runtime/evm@"+digest || got.Digest != digest || !got.CosignVerified || got.TrivyStatus != "passed" {
		t.Fatalf("unexpected image attestation: %#v", got)
	}
}

// TestLoadRejectsInvalidSandboxImageAttestations 确认坏镜像证明清单在启动边界 fail-fast。
func TestLoadRejectsInvalidSandboxImageAttestations(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SANDBOX_IMAGE_ATTESTATIONS_JSON", `{"bad":true}`)

	if _, err := Load(); err == nil {
		t.Fatalf("invalid sandbox image attestations must fail config loading")
	}
}

// TestLoadRejectsInvalidSandboxQuantities 确认 K8s 资源配置在启动时 fail-fast,不会在编排时 panic。
func TestLoadRejectsInvalidSandboxQuantities(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SANDBOX_MAX_CPU", "not-a-quantity")

	if _, err := Load(); err == nil {
		t.Fatalf("invalid sandbox resource quantity must fail config loading")
	}
}

// TestLoadReadsIdentityOperationalLimits 确认 M1 激活码与 SSO 网络边界来自统一配置。
func TestLoadReadsIdentityOperationalLimits(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("IDENTITY_ACTIVATION_CODE_TTL_HOURS", "336")
	t.Setenv("IDENTITY_SSO_NETWORK_TIMEOUT_SECONDS", "8")
	t.Setenv("IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS", "https://chaimir.example.edu, https://admin.example.edu")
	t.Setenv("IDENTITY_PASSWORD_MAX_FAILED_COUNT", "4")
	t.Setenv("IDENTITY_PASSWORD_LOCK_MINUTES", "12")
	t.Setenv("IDENTITY_SMS_RESEND_SECONDS", "50")
	t.Setenv("IDENTITY_SMS_DAILY_LIMIT", "7")
	t.Setenv("IDENTITY_SMS_CODE_TTL_MINUTES", "6")
	t.Setenv("IDENTITY_SMS_VERIFY_MAX_ATTEMPTS", "3")
	t.Setenv("IDENTITY_IMPORT_MAX_ROWS", "1200")
	t.Setenv("IDENTITY_IMPORT_PREVIEW_TTL_HOURS", "36")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Identity.ActivationCodeTTLHours != 336 || cfg.Identity.SSONetworkTimeoutSeconds != 8 {
		t.Fatalf("unexpected identity config: %#v", cfg.Identity)
	}
	if len(cfg.Identity.SSOAllowedServiceOrigins) != 2 ||
		cfg.Identity.SSOAllowedServiceOrigins[0] != "https://chaimir.example.edu" ||
		cfg.Identity.SSOAllowedServiceOrigins[1] != "https://admin.example.edu" {
		t.Fatalf("unexpected sso service origins: %#v", cfg.Identity.SSOAllowedServiceOrigins)
	}
	if cfg.Identity.PasswordMaxFailedCount != 4 || cfg.Identity.PasswordLockMinutes != 12 ||
		cfg.Identity.SMSResendSeconds != 50 || cfg.Identity.SMSDailyLimit != 7 ||
		cfg.Identity.SMSCodeTTLMinutes != 6 || cfg.Identity.SMSVerifyMaxAttempts != 3 ||
		cfg.Identity.ImportMaxRows != 1200 ||
		cfg.Identity.ImportPreviewTTLHours != 36 {
		t.Fatalf("unexpected identity security limits: %#v", cfg.Identity)
	}
}

// TestLoadRejectsInvalidSSOServiceOrigin 确认 CAS 回调白名单格式错误时启动失败。
func TestLoadRejectsInvalidSSOServiceOrigin(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS", "not-a-url")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS") {
		t.Fatalf("invalid sso service origin must fail config loading, got %v", err)
	}
}

// TestLoadReadsTeachingCourseGradeLimit 确认 M6 跨模块成绩读取批量边界来自统一配置。
func TestLoadReadsTeachingCourseGradeLimit(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("TEACHING_COURSE_GRADES_MAX_ROWS", "3000")
	t.Setenv("TEACHING_JUDGE_OUTBOX_BATCH_SIZE", "37")
	t.Setenv("TEACHING_JUDGE_OUTBOX_POLL_INTERVAL_MS", "2500")
	t.Setenv("TEACHING_GRADE_EXPORT_BATCH_SIZE", "88")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Teaching.CourseGradesMaxRows != 3000 ||
		cfg.Teaching.JudgeOutboxBatchSize != 37 ||
		cfg.Teaching.JudgeOutboxPollIntervalMs != 2500 ||
		cfg.Teaching.GradeExportBatchSize != 88 {
		t.Fatalf("unexpected teaching config: %#v", cfg.Teaching)
	}
}

// TestLoadRejectsInvalidTeachingOperationalConfig 确认 M6 后台 worker 与聚合读取阈值在启动边界 fail-fast。
func TestLoadRejectsInvalidTeachingOperationalConfig(t *testing.T) {
	cases := []string{
		"TEACHING_COURSE_GRADES_MAX_ROWS",
		"TEACHING_JUDGE_OUTBOX_BATCH_SIZE",
		"TEACHING_JUDGE_OUTBOX_POLL_INTERVAL_MS",
		"TEACHING_GRADE_EXPORT_BATCH_SIZE",
	}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv(key, "0")

			if _, err := Load(); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("invalid %s must fail config loading, got %v", key, err)
			}
		})
	}
}

// TestLoadReadsGradeAppealWindow 确认 M11 申诉时效窗口来自统一配置。
func TestLoadReadsGradeAppealWindow(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GRADE_APPEAL_WINDOW_DAYS", "45")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Grade.AppealWindowDays != 45 || cfg.Grade.TranscriptSigningKey != "hmac" {
		t.Fatalf("unexpected grade config: %#v", cfg.Grade)
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	required := map[string]string{
		"DEPLOY_MODE":                              "saas",
		"PLATFORM_LAYER_ENABLED":                   "true",
		"HTTP_ADDR":                                "0.0.0.0",
		"HTTP_PORT":                                "8080",
		"WS_PATH":                                  "/ws",
		"WS_ALLOWED_ORIGINS":                       "http://localhost:5173",
		"LOG_LEVEL":                                "info",
		"LOG_FORMAT":                               "json",
		"APP_ENV":                                  "test",
		"HEALTH_CHECK_TIMEOUT_SECONDS":             "3",
		"HTTP_SHUTDOWN_TIMEOUT_SECONDS":            "10",
		"PG_HOST":                                  "postgres",
		"PG_PORT":                                  "5432",
		"PG_DATABASE":                              "chaimir",
		"PG_USER":                                  "chaimir_app",
		"PG_PASSWORD":                              "secret",
		"PG_SSLMODE":                               "disable",
		"PG_MAX_CONNS":                             "20",
		"PG_MIN_CONNS":                             "2",
		"REDIS_HOST":                               "redis",
		"REDIS_PORT":                               "6379",
		"REDIS_DB":                                 "0",
		"REDIS_PING_TIMEOUT_SECONDS":               "5",
		"NATS_URL":                                 "nats://nats:4222",
		"NATS_RECONNECT_WAIT_SECONDS":              "2",
		"MINIO_ENDPOINT":                           "minio:9000",
		"MINIO_USE_SSL":                            "false",
		"MINIO_REGION":                             "cn-local",
		"MINIO_ACCESS_KEY":                         "access",
		"MINIO_SECRET_KEY":                         "secret",
		"MINIO_BUCKET_CODE":                        "chaimir-code",
		"MINIO_BUCKET_ATTACHMENT":                  "chaimir-attachment",
		"MINIO_BUCKET_REPORT":                      "chaimir-report",
		"MINIO_BUCKET_BACKUP":                      "chaimir-backup",
		"MINIO_PING_TIMEOUT_SECONDS":               "5",
		"JWT_SIGNING_KEY":                          "jwt",
		"JWT_ACCESS_TTL_MIN":                       "15",
		"JWT_REFRESH_TTL_DAY":                      "7",
		"JWT_ISSUER":                               "chaimir-test",
		"APP_ENCRYPTION_KEY":                       "12345678901234567890123456789012",
		"APP_HMAC_KEY":                             "hmac",
		"SERVICE_AUTH_MAX_SKEW_SECONDS":            "300",
		"IDENTITY_ACTIVATION_CODE_TTL_HOURS":       "336",
		"IDENTITY_SSO_NETWORK_TIMEOUT_SECONDS":     "10",
		"IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS":     "https://chaimir.example.edu",
		"IDENTITY_PASSWORD_MAX_FAILED_COUNT":       "5",
		"IDENTITY_PASSWORD_LOCK_MINUTES":           "15",
		"IDENTITY_SMS_RESEND_SECONDS":              "60",
		"IDENTITY_SMS_DAILY_LIMIT":                 "10",
		"IDENTITY_SMS_CODE_TTL_MINUTES":            "5",
		"IDENTITY_SMS_VERIFY_MAX_ATTEMPTS":         "5",
		"IDENTITY_IMPORT_MAX_ROWS":                 "5000",
		"IDENTITY_IMPORT_PREVIEW_TTL_HOURS":        "24",
		"SMS_PROVIDER":                             "log",
		"SMS_TIMEOUT_SECONDS":                      "5",
		"UPLOAD_IMPORT_MAX_BYTES":                  "10485760",
		"UPLOAD_SIM_BUNDLE_MAX_BYTES":              "20971520",
		"UPLOAD_SIM_BUNDLE_MAX_FILES":              "200",
		"UPLOAD_SIM_BUNDLE_MAX_UNPACKED_BYTES":     "52428800",
		"CONTEST_VULN_SOURCE_MAX_RESPONSE_BYTES":   "10485760",
		"CONTEST_VULN_SOURCE_TIMEOUT_SECONDS":      "10",
		"NOTIFY_EVENT_RETRY_MAX":                   "3",
		"NOTIFY_EVENT_RETRY_DELAY_MS":              "100",
		"NOTIFY_UNREAD_TTL_HOURS":                  "4320",
		"NOTIFY_SEND_RATE_WINDOW_SECONDS":          "60",
		"NOTIFY_SEND_RATE_MAX":                     "120",
		"TEACHING_COURSE_GRADES_MAX_ROWS":          "10000",
		"TEACHING_JUDGE_OUTBOX_BATCH_SIZE":         "10",
		"TEACHING_JUDGE_OUTBOX_POLL_INTERVAL_MS":   "1000",
		"TEACHING_GRADE_EXPORT_BATCH_SIZE":         "200",
		"GRADE_APPEAL_WINDOW_DAYS":                 "30",
		"IMAGE_REGISTRY":                           "registry",
		"SANDBOX_NS_PREFIX_STUDENT":                "sbx",
		"SANDBOX_NS_PREFIX_JUDGE":                  "judge",
		"SANDBOX_NS_PREFIX_BATTLE":                 "battle",
		"SANDBOX_PREPULL_NAMESPACE":                "chaimir-prepull",
		"SANDBOX_IMAGE_ATTESTATIONS_JSON":          `[{"image_url":"registry/runtime/evm@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","cosign_verified":true,"trivy_status":"passed"}]`,
		"SANDBOX_DEFAULT_CPU":                      "1",
		"SANDBOX_DEFAULT_MEMORY":                   "1Gi",
		"SANDBOX_DEFAULT_REQUEST_CPU":              "250m",
		"SANDBOX_DEFAULT_REQUEST_MEMORY":           "256Mi",
		"SANDBOX_MAX_CPU":                          "4",
		"SANDBOX_MAX_MEMORY":                       "8Gi",
		"SANDBOX_MAX_PODS":                         "16",
		"SANDBOX_WORKSPACE_STORAGE":                "2Gi",
		"SANDBOX_PREPULL_TIMEOUT_SECONDS":          "120",
		"SANDBOX_READY_TIMEOUT_SECONDS":            "30",
		"SANDBOX_PREPULL_POLL_INTERVAL_SECONDS":    "2",
		"SANDBOX_READY_POLL_INTERVAL_SECONDS":      "1",
		"SANDBOX_PREPULL_REQUEST_CPU":              "10m",
		"SANDBOX_PREPULL_REQUEST_MEMORY":           "32Mi",
		"SANDBOX_PREPULL_LIMIT_CPU":                "100m",
		"SANDBOX_PREPULL_LIMIT_MEMORY":             "128Mi",
		"SANDBOX_CHAIN_RPC_TIMEOUT_SECONDS":        "10",
		"SANDBOX_INIT_ARCHIVE_MAX_FILES":           "200",
		"SANDBOX_INIT_ARCHIVE_MAX_UNPACKED_BYTES":  "52428800",
		"SANDBOX_PROBE_DEFAULT_PERIOD_SECONDS":     "2",
		"SANDBOX_PROBE_DEFAULT_FAILURE_THRESHOLD":  "30",
		"SANDBOX_RECYCLE_POLL_INTERVAL_SECONDS":    "60",
		"SANDBOX_RECYCLE_BATCH_SIZE":               "100",
		"SANDBOX_READY_IDLE_TIMEOUT_SECONDS":       "600",
		"SANDBOX_SELFTEST_RECYCLE_TIMEOUT_SECONDS": "30",
		"SANDBOX_CONTROL_NAMESPACE":                "chaimir-system",
		"SANDBOX_CONTROL_POD_LABEL_KEY":            "app.kubernetes.io/name",
		"SANDBOX_CONTROL_POD_LABEL_VALUE":          "chaimir-backend",
		"JUDGE_QUEUE_POLL_INTERVAL_MS":             "1000",
		"JUDGE_WORKER_BATCH_SIZE":                  "4",
		"JUDGE_SUBMIT_RATE_LIMIT_SECONDS":          "10",
		"JUDGE_DEFAULT_MAX_RETRIES":                "2",
		"JUDGE_SANDBOX_READY_POLL_INTERVAL_MS":     "500",
		"JUDGE_RESULT_DETAILS_MAX_BYTES":           "65536",
		"JUDGE_INPUT_INJECT_TIMEOUT_SECONDS":       "60",
		"JUDGE_INPUT_ARCHIVE_MAX_FILES":            "200",
		"JUDGE_INPUT_ARCHIVE_MAX_UNPACKED_BYTES":   "52428800",
		"JUDGE_SIMILARITY_DEFAULT_THRESHOLD":       "0.8",
		"MONITORING_PANELS_JSON":                   "[]",
		"SNOWFLAKE_NODE_ID":                        "1",
	}
	for key, value := range required {
		t.Setenv(key, value)
	}
}
