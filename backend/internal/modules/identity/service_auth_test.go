// M1 认证服务测试:覆盖登录后状态决策与安全边界。
package identity

import (
	"testing"

	"chaimir/internal/modules/identity/internal/sqlcgen"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestVerifiedPasswordWithMustChangeStillIssuesTokens 确认首登改密账号校验密码后仍获得受限会话。
func TestVerifiedPasswordWithMustChangeStillIssuesTokens(t *testing.T) {
	if err := passwordLoginPostVerifyError(true); err != nil {
		t.Fatalf("must_change_pwd should be returned in login result, not block token issuance: %v", err)
	}
}

// TestInitialPasswordPendingAccountCanEnterFirstPasswordChange 确认初始密码开通账号可先登录进入首登改密流程。
func TestInitialPasswordPendingAccountCanEnterFirstPasswordChange(t *testing.T) {
	acc := sqlcgen.Account{
		Status:        AccountPending,
		MustChangePwd: true,
		PasswordHash:  pgtype.Text{String: "hash", Valid: true},
	}
	if err := passwordLoginableStatus(acc); err != nil {
		t.Fatalf("pending account with initial password should enter first password change: %v", err)
	}

	acc.PasswordHash = pgtype.Text{}
	if err := passwordLoginableStatus(acc); err == nil {
		t.Fatalf("pending account without initial password must not use password login")
	}
}
