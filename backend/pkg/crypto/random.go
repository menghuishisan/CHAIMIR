package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
)

const randomTokenAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// RandomBytes 生成指定长度的加密安全随机字节,供加密 nonce 或一次性随机值复用。
func RandomBytes(length int) ([]byte, error) {
	if length <= 0 {
		return nil, errors.New("随机字节长度必须大于 0")
	}
	raw := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return nil, fmt.Errorf("生成随机字节失败: %w", err)
	}
	return raw, nil
}

// RandomToken 使用系统 CSPRNG 生成固定长度的不透明凭证。
func RandomToken(length int) (string, error) {
	return RandomTokenFromReader(rand.Reader, length)
}

// RandomTokenFromReader 从指定读取器生成固定长度凭证,供需要外部熵源的安全流程复用。
func RandomTokenFromReader(reader io.Reader, length int) (string, error) {
	if length <= 0 {
		return "", errors.New("随机凭证长度必须大于 0")
	}
	if reader == nil {
		return "", errors.New("随机凭证熵源不能为空")
	}
	max := big.NewInt(int64(len(randomTokenAlphabet)))
	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(reader, max)
		if err != nil {
			return "", fmt.Errorf("生成随机凭证失败: %w", err)
		}
		out[i] = randomTokenAlphabet[n.Int64()]
	}
	return string(out), nil
}
