// storage api 文件提供全平台唯一的短时授权文件下载入口。
package storage

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/logging"

	"github.com/gin-gonic/gin"
)

type downloadStore interface {
	OpenDownload(ctx context.Context, bucket, key string) (ReadSeekCloser, int64, string, error)
}

type grantConsumer interface {
	SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

type downloadAPI struct {
	objects    downloadStore
	consumer   grantConsumer
	signingKey string
}

// DownloadCookiePath 是统一文件服务入口的 Cookie 作用域。
// 签发 mode=stream 授权的业务接口按它写路径受限 Cookie,浏览器直连播放时凭它通过鉴权;
// 路径写在这里而不是各业务模块,避免作用域与真实路由漂移。
const DownloadCookiePath = "/api/v1/storage"

// RegisterDownloadRoutes 注册统一下载入口,业务模块不得再注册私有对象下载路由。
func RegisterDownloadRoutes(r gin.IRouter, objects downloadStore, consumer grantConsumer, signingKey string, authn *auth.Manager) error {
	if r == nil {
		return apperr.ErrHTTPRouterMissing
	}
	if objects == nil || consumer == nil || strings.TrimSpace(signingKey) == "" {
		return apperr.ErrHTTPServiceMissing
	}
	if authn == nil {
		return apperr.ErrHTTPAuthMissing
	}
	api := downloadAPI{objects: objects, consumer: consumer, signingKey: signingKey}
	// 本入口接受 Bearer 头或路径受限 Cookie:`<video src>` 这类由浏览器自身发起的请求
	// 发不出 Authorization 头,而 query 的 `token` 参数位已被投放授权占用,不能同名两义
	// (见 docs/总-API接口总览.md §统一文件服务「鉴权载体」)。
	r.Group("/api/v1/storage", authn.FileAccessMiddleware()).GET("/download", api.download)
	return nil
}

// download 校验、消费授权并把对象内容流式写入附件响应。
func (a downloadAPI) download(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	grant, err := VerifyDownloadGrantToken(token, a.signingKey)
	if err != nil {
		response.Fail(c, apperr.ErrDownloadGrantInvalid.WithCause(err))
		return
	}
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	if err := authorizeDownloadGrant(grant, id); err != nil {
		response.Fail(c, err)
		return
	}
	if grant.Mode != DownloadModeStream {
		if err := consumeDownloadGrant(c.Request.Context(), a.consumer, token, grant.ExpiresAt, time.Now().UTC()); err != nil {
			response.Fail(c, err)
			return
		}
	}
	reader, size, contentType, err := a.objects.OpenDownload(c.Request.Context(), grant.Object.Bucket, grant.Object.Key)
	if err != nil {
		response.Fail(c, apperr.ErrDownloadObjectUnavailable.WithCause(err))
		return
	}
	defer logging.CloseContext(c.Request.Context(), "close downloaded object", reader)
	fileName := downloadFileName(grant.Object.Key)
	if grant.Mode == DownloadModeStream {
		if strings.TrimSpace(contentType) == "" {
			contentType = "application/octet-stream"
		}
		c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, httpx.SafeAttachmentName(fileName)))
		c.Header("Content-Type", contentType)
		c.Header("X-Content-Type-Options", "nosniff")
		http.ServeContent(c.Writer, c.Request, fileName, time.Time{}, reader)
		return
	}
	httpx.WriteAttachmentStream(c, fileName, contentType, size, reader)
}

// authorizeDownloadGrant 确保授权只能由签发时绑定的租户和账号消费。
func authorizeDownloadGrant(grant DownloadGrant, id tenant.Identity) error {
	if grant.TenantID != id.TenantID {
		return apperr.ErrCrossTenant
	}
	if grant.AccountID != id.AccountID {
		return apperr.ErrForbidden
	}
	if (grant.TenantID == 0) != id.IsPlatform {
		return apperr.ErrForbidden
	}
	return nil
}

// consumeDownloadGrant 使用令牌摘要原子标记授权已消费,不把敏感 token 写入 Redis 键。
func consumeDownloadGrant(ctx context.Context, consumer grantConsumer, token string, expiresAt, now time.Time) error {
	ttl := expiresAt.UTC().Sub(now.UTC())
	if ttl <= 0 {
		return apperr.ErrDownloadGrantInvalid
	}
	key := "storage:download-grant:" + pkgcrypto.SHA256Hex([]byte(token))
	ok, err := consumer.SetNX(ctx, key, ttl)
	if err != nil {
		return apperr.ErrDownloadObjectUnavailable.WithCause(fmt.Errorf("记录下载授权消费状态失败: %w", err))
	}
	if !ok {
		return apperr.ErrDownloadGrantConsumed
	}
	return nil
}

// downloadFileName 从已校验对象 key 取最后一段作为附件名。
func downloadFileName(key string) string {
	return path.Base(key)
}
