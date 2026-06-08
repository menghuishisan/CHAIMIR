// M1 凭证随机性测试:守护登录与开通凭证不能由时间或雪花 ID 派生。
package identity

import (
	"os"
	"strings"
	"testing"
)

// TestOpaqueCredentialsUseRandomTokenGenerator 确认不透明凭证统一走 CSPRNG 随机生成。
func TestOpaqueCredentialsUseRandomTokenGenerator(t *testing.T) {
	cases := []string{"service.go", "service_auth.go", "service_activation.go"}
	for _, file := range cases {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(src)
		if strings.Contains(text, "timex.Now().UnixNano()") ||
			strings.Contains(text, `fmt.Sprintf("temp:`) ||
			strings.Contains(text, `fmt.Sprintf("rt:`) ||
			strings.Contains(text, `fmt.Sprintf("activate:`) {
			t.Fatalf("%s must generate opaque credentials with crypto.RandomToken", file)
		}
	}
}
