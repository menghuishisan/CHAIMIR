// 账号开通方式测试:临时密码与激活码两种模式不能混用。
package identity

import "testing"

// TestBuildAccountOpeningCredentialUsesActivationCodeMode 确认激活码模式不写初始密码。
func TestBuildAccountOpeningCredentialUsesActivationCodeMode(t *testing.T) {
	cred, err := buildAccountOpeningCredential(true, "TempPass123")
	if err != nil {
		t.Fatalf("build credential: %v", err)
	}
	if cred.PasswordHash.Valid {
		t.Fatalf("activation-code mode must not store initial password")
	}
	if cred.MustChangePassword {
		t.Fatalf("activation-code mode should not require first-login password change")
	}
	if !cred.NeedsActivationCode {
		t.Fatalf("activation-code mode should request activation code issuance")
	}
	if cred.InitPassword != "" {
		t.Fatalf("activation-code mode must not return temp password")
	}
}

// TestBuildAccountOpeningCredentialUsesInitialPasswordMode 确认本地开通模式生成首登改密凭据。
func TestBuildAccountOpeningCredentialUsesInitialPasswordMode(t *testing.T) {
	cred, err := buildAccountOpeningCredential(false, "TempPass123")
	if err != nil {
		t.Fatalf("build credential: %v", err)
	}
	if !cred.PasswordHash.Valid {
		t.Fatalf("initial-password mode should store password hash")
	}
	if !cred.MustChangePassword {
		t.Fatalf("initial-password mode should require first-login password change")
	}
	if cred.NeedsActivationCode {
		t.Fatalf("initial-password mode should not request activation code")
	}
	if cred.InitPassword != "TempPass123" {
		t.Fatalf("expected temp password to be returned")
	}
}
