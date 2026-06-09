// identity rules_test 文件校验身份模块输入规则和状态机规则。
package identity

import "testing"

func TestValidatePhoneRejectsInvalidChineseMobile(t *testing.T) {
	if err := ValidatePhone("123456"); err == nil {
		t.Fatalf("期望非法手机号被拒绝")
	}
	if err := ValidatePhone("13800138000"); err != nil {
		t.Fatalf("期望合法手机号通过: %v", err)
	}
}

func TestValidatePasswordRequiresLetterAndDigit(t *testing.T) {
	cases := []string{"12345678", "abcdefgh", "a1"}
	for _, tc := range cases {
		if err := ValidatePassword(tc); err == nil {
			t.Fatalf("期望弱密码 %q 被拒绝", tc)
		}
	}
	if err := ValidatePassword("abc12345"); err != nil {
		t.Fatalf("期望满足规则的密码通过: %v", err)
	}
}
