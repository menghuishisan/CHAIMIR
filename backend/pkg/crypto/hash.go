// crypto 提供密码哈希、对称加密、HMAC 哈希。
package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// HashPassword 用 argon2id 生成密码哈希。
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成 salt 失败: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		encodeRawStd(salt),
		encodeRawStd(hash),
	), nil
}

// VerifyPassword 恒定时间比较明文与存储哈希。
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("密码哈希格式非法")
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, fmt.Errorf("解析 argon2 参数失败: %w", err)
	}
	salt, err := decodeRawStd(parts[4])
	if err != nil {
		return false, fmt.Errorf("解析 salt 失败: %w", err)
	}
	want, err := decodeRawStd(parts[5])
	if err != nil {
		return false, fmt.Errorf("解析 hash 失败: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return EqualBytes(got, want), nil
}

// SHA256Hex 计算普通内容摘要的十六进制 SHA-256,用于版本、归档等非密钥完整性场景。
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// HMACSHA256Hex 计算十六进制 HMAC-SHA256。
func HMACSHA256Hex(key []byte, message string) (string, error) {
	if len(key) == 0 {
		return "", fmt.Errorf("HMAC 密钥不能为空")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// HMACHash 生成确定性哈希,用于 phone_hash / code_hash / refresh_token_hash。
func HMACHash(key []byte, value string) (string, error) {
	return HMACSHA256Hex(key, value)
}

// EqualHMAC 使用常量时间比较两个 HMAC 字符串。
func EqualHMAC(a string, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// EqualHexHMAC 解码十六进制 HMAC 后做常量时间比较,用于服务间签名等外部输入。
func EqualHexHMAC(a string, b string) bool {
	left, err := hex.DecodeString(a)
	if err != nil {
		return false
	}
	right, err := hex.DecodeString(b)
	if err != nil {
		return false
	}
	return len(left) == len(right) && hmac.Equal(left, right)
}

// EqualBytes 对两段字节做常量时间比较。
func EqualBytes(a []byte, b []byte) bool {
	return hmac.Equal(a, b)
}

// encodeRawStd 用 RawStdEncoding 编码二进制片段,保持密码哈希字符串紧凑稳定。
func encodeRawStd(data []byte) string {
	return base64.RawStdEncoding.EncodeToString(data)
}

// decodeRawStd 解码 RawStdEncoding 编码片段。
func decodeRawStd(value string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(value)
}
