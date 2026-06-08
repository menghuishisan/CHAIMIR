// M1 服务配置测试:确认运行边界由装配配置注入,不在模块内硬编码。
package identity

import (
	"testing"
	"time"

	"chaimir/internal/platform/config"
)

// TestNewServiceKeepsOperationalConfig 确认激活码有效期与 SSO 网络超时来自统一配置。
func TestNewServiceKeepsOperationalConfig(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, nil, config.DeployConfig{}, config.IdentityConfig{
		ActivationCodeTTLHours:   48,
		SSONetworkTimeoutSeconds: 7,
		SSOAllowedServiceOrigins: []string{"https://chaimir.example.edu"},
		PasswordMaxFailedCount:   3,
		PasswordLockMinutes:      9,
		SMSResendSeconds:         45,
		SMSDailyLimit:            6,
		SMSCodeTTLMinutes:        4,
		SMSVerifyMaxAttempts:     3,
		ImportMaxRows:            1200,
		ImportPreviewTTLHours:    36,
	}, 24*time.Hour)

	if svc.activationCodeTTL != 48*time.Hour {
		t.Fatalf("activation ttl was not injected: %s", svc.activationCodeTTL)
	}
	if svc.ssoNetworkTimeout != 7*time.Second {
		t.Fatalf("sso timeout was not injected: %s", svc.ssoNetworkTimeout)
	}
	if len(svc.ssoAllowedServiceOrigins) != 1 || svc.ssoAllowedServiceOrigins[0] != "https://chaimir.example.edu" {
		t.Fatalf("sso service origins were not injected: %#v", svc.ssoAllowedServiceOrigins)
	}
	if svc.passwordMaxFailedCount != 3 {
		t.Fatalf("password max failed count was not injected: %d", svc.passwordMaxFailedCount)
	}
	if svc.passwordLockMinutes != 9 {
		t.Fatalf("password lock minutes was not injected: %d", svc.passwordLockMinutes)
	}
	if svc.smsResendInterval != 45*time.Second {
		t.Fatalf("sms resend interval was not injected: %s", svc.smsResendInterval)
	}
	if svc.smsDailyLimit != 6 {
		t.Fatalf("sms daily limit was not injected: %d", svc.smsDailyLimit)
	}
	if svc.smsCodeTTL != 4*time.Minute {
		t.Fatalf("sms code ttl was not injected: %s", svc.smsCodeTTL)
	}
	if svc.smsVerifyMaxAttempts != 3 {
		t.Fatalf("sms verify max attempts was not injected: %d", svc.smsVerifyMaxAttempts)
	}
	if svc.importMaxRows != 1200 {
		t.Fatalf("import max rows was not injected: %d", svc.importMaxRows)
	}
	if svc.importPreviewTTL != 36*time.Hour {
		t.Fatalf("import preview ttl was not injected: %s", svc.importPreviewTTL)
	}
}
