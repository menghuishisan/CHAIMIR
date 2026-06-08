// Package crypto 的随机凭证测试:确认不透明凭证统一来自可替换的 CSPRNG 读取器。
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
