// Package crypto 提供密码哈希、对称加密和安全随机凭证。
package crypto

import (
	"crypto/hmac"
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

var argonParams = fmt.Sprintf("m=%d,t=%d,p=%d", argonMemory, argonTime, argonThreads)

// HashPassword 用 argon2id 生成密码哈希。
func HashPassword(password string) (string, error) {
	salt, err := RandomBytes(argonSaltLen)
	if err != nil {
		return "", fmt.Errorf("生成 salt 失败: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$%s$%s$%s",
		argonParams,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword 恒定时间比较明文与存储哈希。
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, fmt.Errorf("密码哈希格式非法")
	}
	if parts[3] != argonParams {
		return false, fmt.Errorf("argon2 参数不符合平台安全配置")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("解析 salt 失败: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("解析 hash 失败: %w", err)
	}
	if len(salt) != argonSaltLen || len(want) != argonKeyLen {
		return false, fmt.Errorf("密码哈希长度非法")
	}
	got := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return hmac.Equal(got, want), nil
}

// SHA256Hex 计算普通内容摘要的十六进制 SHA-256,用于版本、归档等非密钥完整性场景。
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ValidSHA256Hex 判断外部内容摘要是否为完整的十六进制 SHA-256。
func ValidSHA256Hex(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
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
	return hmac.Equal(left, right)
}
