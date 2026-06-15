// crypto 提供随机凭证生成,用于 Refresh Token、激活码和一次性短期密码等不透明秘密。
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
	return randomStringFromReader(reader, randomTokenAlphabet, length, "随机凭证")
}

// randomStringFromReader 使用 crypto/rand.Int 在默认字符集内均匀抽样,避免不同短码各自实现随机逻辑。
func randomStringFromReader(reader io.Reader, alphabet string, length int, label string) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("%s长度必须大于 0", label)
	}
	if reader == nil {
		return "", fmt.Errorf("%s熵源不能为空", label)
	}
	if len(alphabet) == 0 || len(alphabet) > 256 {
		return "", errors.New("随机字母表长度必须在 1 到 256 之间")
	}
	max := big.NewInt(int64(len(alphabet)))
	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(reader, max)
		if err != nil {
			return "", fmt.Errorf("生成%s失败: %w", label, err)
		}
		out[i] = alphabet[n.Int64()]
	}
	return string(out), nil
}
