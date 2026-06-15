// storage 提供统一文件服务的下载授权模型,约束对象访问必须经过租户与资源边界校验。
package storage

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/platform/timex"
	pkgcrypto "chaimir/pkg/crypto"
)

// DownloadGrantRequest 描述一次经过业务鉴权后的受控下载授权请求。
type DownloadGrantRequest struct {
	TenantID     int64
	AccountID    int64
	ObjectRef    string
	Module       string
	ResourceType string
	ResourceID   string
	ExpiresAt    time.Time
}

// DownloadGrant 表示统一文件服务生成的一次性受控下载授权快照。
type DownloadGrant struct {
	TenantID     int64
	AccountID    int64
	Module       string
	ResourceType string
	ResourceID   string
	Object       ObjectRef
	ExpiresAt    time.Time
}

type downloadGrantTokenEnvelope struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

const maxDownloadGrantTokenLength = 8192

// BuildDownloadGrant 校验对象引用和租户前缀后生成下载授权,避免业务模块直接暴露存储直链。
func BuildDownloadGrant(req DownloadGrantRequest) (DownloadGrant, error) {
	if req.TenantID <= 0 {
		return DownloadGrant{}, fmt.Errorf("下载授权缺少 tenant_id")
	}
	if req.AccountID <= 0 {
		return DownloadGrant{}, fmt.Errorf("下载授权缺少 account_id")
	}
	if strings.TrimSpace(req.Module) == "" || strings.TrimSpace(req.ResourceType) == "" || strings.TrimSpace(req.ResourceID) == "" {
		return DownloadGrant{}, fmt.Errorf("下载授权缺少资源边界")
	}
	if req.ExpiresAt.IsZero() || !req.ExpiresAt.After(timex.Now()) {
		return DownloadGrant{}, fmt.Errorf("下载授权过期时间非法")
	}

	objectRef, err := ParseObjectRef(req.ObjectRef)
	if err != nil {
		return DownloadGrant{}, err
	}

	// 统一要求对象 key 落在 {tenant}/{module}/{resourceType}/... 前缀下,阻断跨租户与跨资源直链复用。
	expectedPrefix, err := ObjectKey(req.TenantID, req.Module, req.ResourceType)
	if err != nil {
		return DownloadGrant{}, err
	}
	if objectRef.Key != expectedPrefix && !strings.HasPrefix(objectRef.Key, expectedPrefix+"/") {
		return DownloadGrant{}, fmt.Errorf("对象引用不属于当前租户资源前缀")
	}

	return DownloadGrant{
		TenantID:     req.TenantID,
		AccountID:    req.AccountID,
		Module:       req.Module,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		Object:       objectRef,
		ExpiresAt:    req.ExpiresAt.UTC(),
	}, nil
}

// SignDownloadGrantToken 把受控下载授权编码并签名为短时令牌,供统一文件服务下载入口校验。
func SignDownloadGrantToken(grant DownloadGrant, signingKey string) (string, error) {
	if strings.TrimSpace(signingKey) == "" {
		return "", fmt.Errorf("下载授权签名密钥不能为空")
	}
	if err := validateDownloadGrant(grant, timex.Now()); err != nil {
		return "", err
	}

	payload, err := json.Marshal(grant)
	if err != nil {
		return "", fmt.Errorf("编码下载授权失败: %w", err)
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payload)
	signature, err := pkgcrypto.HMACSHA256Hex([]byte(signingKey), payloadEncoded)
	if err != nil {
		return "", fmt.Errorf("签名下载授权失败: %w", err)
	}
	token, err := json.Marshal(downloadGrantTokenEnvelope{
		Payload:   payloadEncoded,
		Signature: signature,
	})
	if err != nil {
		return "", fmt.Errorf("编码下载授权令牌失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(token), nil
}

// VerifyDownloadGrantToken 校验下载授权令牌签名、内容和有效期,拒绝过期或被篡改的对象下载请求。
func VerifyDownloadGrantToken(token string, signingKey string) (DownloadGrant, error) {
	return verifyDownloadGrantTokenAt(token, signingKey, timex.Now())
}

// verifyDownloadGrantTokenAt 在给定时间点评估下载授权令牌,供测试稳定验证过期边界。
func verifyDownloadGrantTokenAt(token string, signingKey string, now time.Time) (DownloadGrant, error) {
	if strings.TrimSpace(signingKey) == "" {
		return DownloadGrant{}, fmt.Errorf("下载授权签名密钥不能为空")
	}
	if strings.TrimSpace(token) == "" {
		return DownloadGrant{}, fmt.Errorf("下载授权令牌不能为空")
	}
	if len(token) > maxDownloadGrantTokenLength {
		return DownloadGrant{}, fmt.Errorf("下载授权令牌超出长度限制")
	}

	rawEnvelope, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return DownloadGrant{}, fmt.Errorf("解码下载授权令牌失败: %w", err)
	}
	var envelope downloadGrantTokenEnvelope
	if err := json.Unmarshal(rawEnvelope, &envelope); err != nil {
		return DownloadGrant{}, fmt.Errorf("解析下载授权令牌失败: %w", err)
	}
	if strings.TrimSpace(envelope.Payload) == "" || strings.TrimSpace(envelope.Signature) == "" {
		return DownloadGrant{}, fmt.Errorf("下载授权令牌缺少必要字段")
	}

	expectedSignature, err := pkgcrypto.HMACSHA256Hex([]byte(signingKey), envelope.Payload)
	if err != nil {
		return DownloadGrant{}, fmt.Errorf("校验下载授权签名失败: %w", err)
	}
	if !pkgcrypto.EqualHexHMAC(expectedSignature, envelope.Signature) {
		return DownloadGrant{}, fmt.Errorf("下载授权签名无效")
	}

	payload, err := base64.RawURLEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return DownloadGrant{}, fmt.Errorf("解码下载授权负载失败: %w", err)
	}
	var grant DownloadGrant
	if err := json.Unmarshal(payload, &grant); err != nil {
		return DownloadGrant{}, fmt.Errorf("解析下载授权负载失败: %w", err)
	}
	if err := validateDownloadGrant(grant, now.UTC()); err != nil {
		return DownloadGrant{}, err
	}
	return grant, nil
}

// validateDownloadGrant 在签发和验签两端统一执行授权边界校验,避免令牌内容绕过对象前缀限制。
func validateDownloadGrant(grant DownloadGrant, now time.Time) error {
	if grant.TenantID <= 0 {
		return fmt.Errorf("下载授权缺少 tenant_id")
	}
	if grant.AccountID <= 0 {
		return fmt.Errorf("下载授权缺少 account_id")
	}
	if strings.TrimSpace(grant.Module) == "" || strings.TrimSpace(grant.ResourceType) == "" || strings.TrimSpace(grant.ResourceID) == "" {
		return fmt.Errorf("下载授权缺少资源边界")
	}
	if grant.ExpiresAt.IsZero() || !grant.ExpiresAt.UTC().After(now) {
		return fmt.Errorf("下载授权已过期")
	}
	if !safeObjectRefBucket(grant.Object.Bucket) || !safeObjectRefKey(grant.Object.Key) {
		return fmt.Errorf("下载授权对象引用非法")
	}

	// 重新校验对象 key 是否仍受统一租户前缀约束,避免篡改后的 payload 越权下载其他资源。
	expectedPrefix, err := ObjectKey(grant.TenantID, grant.Module, grant.ResourceType)
	if err != nil {
		return err
	}
	if grant.Object.Key != expectedPrefix && !strings.HasPrefix(grant.Object.Key, expectedPrefix+"/") {
		return fmt.Errorf("对象引用不属于当前租户资源前缀")
	}
	return nil
}
