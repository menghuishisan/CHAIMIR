// 短信验证码生成测试。
package identity

import (
	"context"
	"errors"
	"testing"

	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

type failingReader struct{}

// Read 固定返回错误,用于验证随机源失败不被吞掉。
func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("random failed")
}

// TestGenSmsCodeFromReaderReturnsRandomError 确认验证码随机源错误会显式返回。
func TestGenSmsCodeFromReaderReturnsRandomError(t *testing.T) {
	if _, err := genSmsCodeFromReader(failingReader{}); err == nil {
		t.Fatalf("expected random source error")
	}
}

// TestSmsTenantIDForRebindUsesAuthenticatedTenant 确认换绑验证码写入当前登录租户。
func TestSmsTenantIDForRebindUsesAuthenticatedTenant(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	tenantID, ok, err := smsTenantIDFromContext(ctx, SmsSceneRebind)

	if err != nil {
		t.Fatalf("resolve rebind sms tenant: %v", err)
	}
	if !ok || tenantID != 1001 {
		t.Fatalf("expected tenant 1001 from auth context, got tenant=%d ok=%v", tenantID, ok)
	}
}

// TestSmsTenantIDForRebindRequiresAuthenticatedTenant 确认未登录不能发送换绑验证码。
func TestSmsTenantIDForRebindRequiresAuthenticatedTenant(t *testing.T) {
	if _, _, err := smsTenantIDFromContext(context.Background(), SmsSceneRebind); err == nil {
		t.Fatalf("expected rebind sms without tenant context to fail")
	}
}

// TestResetPasswordTargetRequiresSingleTenant 确认无学校上下文的找回密码不能在一号多校手机号上误选学校。
func TestResetPasswordTargetRequiresSingleTenant(t *testing.T) {
	_, err := selectResetPasswordTarget([]LoginTenantCandidate{
		{TenantID: 1001, AccountID: 2001},
		{TenantID: 1002, AccountID: 2002},
	}, "")
	if err != apperr.ErrResetPasswordTenantAmbiguous {
		t.Fatalf("expected ambiguous reset target to be rejected, got %v", err)
	}
}

// TestResetPasswordTargetUsesRequestedTenant 确认一号多校找回密码可按用户选择的学校定位账号。
func TestResetPasswordTargetUsesRequestedTenant(t *testing.T) {
	target, err := selectResetPasswordTarget([]LoginTenantCandidate{
		{TenantID: 1001, AccountID: 2001},
		{TenantID: 1002, AccountID: 2002},
	}, "1002")
	if err != nil {
		t.Fatalf("select reset target: %v", err)
	}
	if target.TenantID != 1002 || target.AccountID != 2002 {
		t.Fatalf("unexpected reset target: %+v", target)
	}
}

// TestResetPasswordSmsVerificationUsesGlobalScope 确认找回验证码按 tenant_id=NULL 的全局路径校验。
func TestResetPasswordSmsVerificationUsesGlobalScope(t *testing.T) {
	target, err := selectResetPasswordTarget([]LoginTenantCandidate{
		{TenantID: 1001, AccountID: 2001},
	}, "")
	if err != nil {
		t.Fatalf("select reset target: %v", err)
	}
	if tenantID := resetSmsVerificationTenantID(target); tenantID != 0 {
		t.Fatalf("reset sms verification must use global NULL-tenant scope, got %d", tenantID)
	}
}

// TestLoginSmsTenantSelectionRequiresTenantForMultipleAccounts 确认一号多校短信验证码不会误写入第一个租户。
func TestLoginSmsTenantSelectionRequiresTenantForMultipleAccounts(t *testing.T) {
	_, err := selectLoginSmsTenantID([]AccountTenantCandidate{
		{TenantID: 1001},
		{TenantID: 1002},
	}, "")
	if err != apperr.ErrSmsTenantRequired {
		t.Fatalf("expected ErrSmsTenantRequired for multi-tenant sms send, got %v", err)
	}
}

// TestLoginSmsTenantSelectionUsesRequestedTenant 确认登录短信验证码写入用户选择的学校租户。
func TestLoginSmsTenantSelectionUsesRequestedTenant(t *testing.T) {
	tenantID, err := selectLoginSmsTenantID([]AccountTenantCandidate{
		{TenantID: 1001},
		{TenantID: 1002},
	}, "1002")
	if err != nil {
		t.Fatalf("select login sms tenant: %v", err)
	}
	if tenantID != 1002 {
		t.Fatalf("expected tenant 1002, got %d", tenantID)
	}
}

// TestSmsVerificationAttemptPolicyBlocksAtLimit 确认验证码校验错误达到上限后立即失效。
func TestSmsVerificationAttemptPolicyBlocksAtLimit(t *testing.T) {
	if smsVerificationAttemptsExceeded(2, 3) {
		t.Fatalf("second failed verification should still allow one more attempt")
	}
	if !smsVerificationAttemptsExceeded(3, 3) {
		t.Fatalf("third failed verification should block the code")
	}
	if smsVerificationAttemptsExceeded(100, 0) {
		t.Fatalf("zero max attempts should disable the attempt gate")
	}
}
