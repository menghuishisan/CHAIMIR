// M1 密码策略测试:覆盖强度校验与弱口令拦截。
package identity

import "testing"

// TestValidPasswordRejectsCommonWeakPasswords 确认常见弱口令不会因满足长度/字母/数字而通过。
func TestValidPasswordRejectsCommonWeakPasswords(t *testing.T) {
	for _, password := range []string{"Password123", "Qwerty123", "Admin123456", "Chaimir123"} {
		if validPassword(password) {
			t.Fatalf("weak password %q must be rejected", password)
		}
	}
}

// TestValidPasswordAcceptsNonDictionaryStrongPassword 确认非字典强密码仍可使用。
func TestValidPasswordAcceptsNonDictionaryStrongPassword(t *testing.T) {
	if !validPassword("CmirLab7294") {
		t.Fatalf("expected non-dictionary strong password to be accepted")
	}
}
