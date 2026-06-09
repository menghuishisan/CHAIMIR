// crypto 对称加密:提供 AES-256-GCM,用于手机号等敏感字段密文存储。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
)

// Cipher 封装 AES-GCM 所需的 AEAD 实例。
type Cipher struct {
	gcm cipher.AEAD
}

// NewCipher 用 32 字节密钥构造 AES-256-GCM;长度不符即报错。
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("加密密钥须为 32 字节(AES-256),实际 %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("初始化 AES 失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("初始化 GCM 失败: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

// Encrypt 加密明文,输出 nonce 前置的密文。
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	if c == nil || c.gcm == nil {
		return nil, fmt.Errorf("加密器尚未初始化")
	}
	nonce, err := RandomBytes(c.gcm.NonceSize())
	if err != nil {
		return nil, err
	}
	return c.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// EncryptString 加密文本并编码为标准 base64,用于 JSON/文本字段安全存储。
func (c *Cipher) EncryptString(plaintext string) (string, error) {
	encrypted, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// Decrypt 解密 nonce 前置的密文。
func (c *Cipher) Decrypt(data []byte) ([]byte, error) {
	if c == nil || c.gcm == nil {
		return nil, fmt.Errorf("加密器尚未初始化")
	}
	ns := c.gcm.NonceSize()
	if len(data) < ns {
		return nil, errors.New("密文长度非法")
	}
	nonce, ciphertext := data[:ns], data[ns:]
	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}
	return plaintext, nil
}

// DecryptString 解码 base64 密文并解密为文本。
func (c *Cipher) DecryptString(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("解析密文编码失败: %w", err)
	}
	plaintext, err := c.Decrypt(data)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
