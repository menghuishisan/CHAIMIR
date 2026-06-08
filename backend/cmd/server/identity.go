// identity 模块(M1,第0层 地基)装配。
// 职责:租户/账号/认证/授权/审计表;所有模块的前置依赖。
// 本文件构造 service/repo,注入 contracts.IdentityService、audit.Writer 实现,
//
//	注册 /auth /platform /tenant /org /accounts /me /audit 路由。
package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/modules/identity"
	"chaimir/internal/platform/config"
	"chaimir/pkg/crypto"
)

// assembleIdentity 装配 identity 模块(M1)并注册 HTTP 路由与跨模块契约实现。
func assembleIdentity(d *moduleDeps) error {
	cipher, err := crypto.NewCipher([]byte(d.cfg.Auth.EncryptionKey))
	if err != nil {
		return fmt.Errorf("装配 identity 失败: 初始化加密器失败: %w", err)
	}
	smsSender, err := buildIdentitySmsSender(d.cfg)
	if err != nil {
		return fmt.Errorf("装配 identity 失败: 初始化短信发送器失败: %w", err)
	}
	svc := identity.NewService(
		d.infra.db,
		d.infra.auth,
		d.infra.bus,
		d.infra.redis,
		d.infra.idgen,
		cipher,
		smsSender,
		[]byte(d.cfg.Auth.HMACKey),
		d.cfg.Deploy,
		d.cfg.Identity,
		time.Duration(d.cfg.Auth.RefreshTTLDay)*24*time.Hour,
	)
	api := identity.NewAPI(svc, d.infra.auth, d.cfg.Deploy, d.cfg.Upload)
	api.Register(d.infra.server.apiV1())
	d.infra.audit = svc
	d.infra.identity = svc
	d.infra.identityAdmin = svc

	slog.Info("模块装配", slog.String("module", "identity"), slog.String("layer", "0-foundation"))
	return nil
}

// buildIdentitySmsSender 按运行环境选择短信发送器;生产不得使用开发日志发送器。
func buildIdentitySmsSender(cfg *config.Config) (identity.SmsSender, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.SMS.Provider))
	if provider == "log" && cfg.Server.AppEnv != "prod" {
		return identity.LogSmsSender{}, nil
	}
	if provider != "http" {
		return nil, fmt.Errorf("生产环境必须配置真实短信网关,禁止使用开发日志短信发送器")
	}
	if strings.TrimSpace(cfg.SMS.Endpoint) == "" ||
		strings.TrimSpace(cfg.SMS.LoginTemplate) == "" ||
		strings.TrimSpace(cfg.SMS.ResetTemplate) == "" ||
		strings.TrimSpace(cfg.SMS.ChangeTemplate) == "" {
		return nil, fmt.Errorf("SMS_PROVIDER=http 时必须配置 SMS_HTTP_ENDPOINT 和全部短信模板")
	}
	return identity.NewHTTPSmsSender(identity.HTTPSmsConfig{
		Endpoint:       cfg.SMS.Endpoint,
		Token:          cfg.SMS.Token,
		LoginTemplate:  cfg.SMS.LoginTemplate,
		ResetTemplate:  cfg.SMS.ResetTemplate,
		ChangeTemplate: cfg.SMS.ChangeTemplate,
		Timeout:        time.Duration(cfg.SMS.TimeoutSeconds) * time.Second,
	})
}
