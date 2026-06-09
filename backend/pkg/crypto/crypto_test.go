// crypto 测试:验证密码哈希、对称加密、HMAC 与随机凭证的安全边界。
package crypto

import (
	"errors"
	"strings"
	"testing"
)

type failingTokenReader struct{}

// Read 固定返回错误,用于验证随机凭证生成失败会向上暴露。
func (failingTokenReader) Read([]byte) (int, error) {
	return 0, errors.New("random token failed")
}

type fixedTokenReader struct {
	next byte
}

// Read 生成确定字节流,用于稳定验证随机凭证长度与字符集。
func (r *fixedTokenReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.next
		r.next++
	}
	return len(p), nil
}

// TestHashPasswordAndVerify 确认密码使用 argon2id 哈希并可校验。
func TestHashPasswordAndVerify(t *testing.T) {
	hash, err := HashPassword("Passw0rd!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	ok, err := VerifyPassword("Passw0rd!", hash)
	if err != nil || !ok {
		t.Fatalf("verify password failed: %v %v", ok, err)
	}
}

// TestCipherRequiresAES256Key 确认项目加密器只接受 32 字节 AES-256 密钥。
func TestCipherRequiresAES256Key(t *testing.T) {
	if _, err := NewCipher([]byte("short key length")); err == nil {
		t.Fatalf("short key must be rejected")
	}
	if _, err := NewCipher([]byte("12345678901234567890123456789012")); err != nil {
		t.Fatalf("32-byte key should be accepted: %v", err)
	}
}

// TestCipherRoundTrip 确认敏感字段加密后可解密且密文不等于明文。
func TestCipherRoundTrip(t *testing.T) {
	c, err := NewCipher([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	ciphertext, err := c.EncryptString("13800138000")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ciphertext == "13800138000" {
		t.Fatalf("ciphertext must not equal plaintext")
	}
	plaintext, err := c.DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plaintext != "13800138000" {
		t.Fatalf("plaintext = %q", plaintext)
	}
}

// TestHMACRejectsEmptyKey 确认签名和可查询哈希不能使用空密钥。
func TestHMACRejectsEmptyKey(t *testing.T) {
	if _, err := HMACSHA256Hex(nil, "message"); err == nil {
		t.Fatalf("empty key must be rejected")
	}
}

// TestHMACHashRejectsEmptyKey 确认可查询哈希接口也不能静默吞掉空密钥错误。
func TestHMACHashRejectsEmptyKey(t *testing.T) {
	if _, err := HMACHash(nil, "message"); err == nil {
		t.Fatalf("empty key must be rejected by HMACHash")
	} else if containsASCIIErrorOnly(err.Error()) {
		t.Fatalf("error message must be Chinese, got %q", err.Error())
	}
}

// TestCipherErrorMessagesUseChinese 确认加密器边界错误对外统一使用中文提示。
func TestCipherErrorMessagesUseChinese(t *testing.T) {
	c := &Cipher{}
	if _, err := c.Encrypt([]byte("abc")); err == nil {
		t.Fatalf("expected encrypt init error")
	} else if containsASCIIErrorOnly(err.Error()) {
		t.Fatalf("encrypt error message must be Chinese, got %q", err.Error())
	}
	if _, err := c.Decrypt([]byte("abc")); err == nil {
		t.Fatalf("expected decrypt init error")
	} else if containsASCIIErrorOnly(err.Error()) {
		t.Fatalf("decrypt error message must be Chinese, got %q", err.Error())
	}
}

// containsASCIIErrorOnly 判断错误文本是否仍然是纯英文实现提示。
func containsASCIIErrorOnly(text string) bool {
	hasChinese := false
	for _, r := range text {
		if r >= '\u4e00' && r <= '\u9fff' {
			hasChinese = true
			break
		}
	}
	return !hasChinese
}

// TestHMACIsStable 确认相同密钥和消息得到稳定 HMAC。
func TestHMACIsStable(t *testing.T) {
	key := []byte("secret")
	first, err := HMACSHA256Hex(key, "message")
	if err != nil {
		t.Fatalf("hmac first: %v", err)
	}
	second, err := HMACSHA256Hex(key, "message")
	if err != nil {
		t.Fatalf("hmac second: %v", err)
	}
	if first != second || !EqualHMAC(first, second) {
		t.Fatalf("hmac should be stable")
	}
}

// TestRandomTokenFromReaderUsesConfiguredAlphabet 确认随机凭证只包含易读安全字符。
func TestRandomTokenFromReaderUsesConfiguredAlphabet(t *testing.T) {
	token, err := RandomTokenFromReader(&fixedTokenReader{}, 32)
	if err != nil {
		t.Fatalf("random token: %v", err)
	}
	if len(token) != 32 {
		t.Fatalf("expected token length 32, got %d", len(token))
	}
	for _, ch := range token {
		if !strings.ContainsRune(randomTokenAlphabet, ch) {
			t.Fatalf("token contains character outside alphabet: %q", ch)
		}
	}
}

// TestRandomTokenFromReaderReturnsRandomError 确认随机源错误不会被静默吞掉。
func TestRandomTokenFromReaderReturnsRandomError(t *testing.T) {
	if _, err := RandomTokenFromReader(failingTokenReader{}, 16); err == nil {
		t.Fatalf("expected random token source error")
	}
}
