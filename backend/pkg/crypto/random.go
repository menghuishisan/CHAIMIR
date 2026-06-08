// Package crypto 提供随机凭证生成,用于 Refresh Token、激活码和一次性短期密码等不透明秘密。
package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const randomTokenAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// RandomToken 使用系统 CSPRNG 生成固定长度的不透明凭证。
func RandomToken(length int) (string, error) {
	return RandomTokenFromReader(rand.Reader, length)
}

// RandomTokenFromReader 从指定读取器生成固定长度凭证,便于测试随机源错误。
func RandomTokenFromReader(reader io.Reader, length int) (string, error) {
	if length <= 0 {
		return "", errors.New("随机凭证长度必须大于 0")
	}
	raw := make([]byte, length)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return "", fmt.Errorf("生成随机凭证失败: %w", err)
	}
	out := make([]byte, length)
	for i, b := range raw {
		out[i] = randomTokenAlphabet[int(b)&31]
	}
	return string(out), nil
}
