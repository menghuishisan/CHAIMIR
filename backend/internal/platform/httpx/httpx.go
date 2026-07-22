// httpx 提供 HTTP handler 层的无业务通用辅助。
package httpx

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// AuditContextMiddleware 把请求 IP 和 trace_id 注入 context,供业务成功后统一构造审计条目。
func AuditContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := audit.WithRequestContext(c.Request.Context(), audit.RequestContext{
			IP:      c.ClientIP(),
			TraceID: response.TraceFromGin(c),
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// BindJSON 是 handler 层统一请求绑定入口,失败时只返回用户向 bad request 文案。
func BindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		response.Fail(c, apperr.ErrRequestBodyInvalid.WithCause(err))
		return false
	}
	return true
}

// BindJSONWithError 在统一绑定流程中使用调用方指定的稳定错误模板。
func BindJSONWithError(c *gin.Context, dst any, bindErr *apperr.Error) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		if bindErr == nil {
			response.Fail(c, apperr.ErrRequestBodyInvalid.WithCause(err))
			return false
		}
		response.Fail(c, bindErr.WithCause(err))
		return false
	}
	return true
}

// Write 把 service 返回值转换成统一 HTTP 响应,让 API 文件不重复写成功/失败分支。
func Write(c *gin.Context, data any, err error) {
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, data)
}

// WritePage 把分页 service 返回值转换成统一响应,分页结构由 internal/platform/response 单一维护。
func WritePage(c *gin.Context, list any, total int64, page, size int, err error) {
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OKPage(c, list, total, page, size)
}

// WriteAttachment 统一输出小型附件内容,避免各模块手写不安全的 Content-Disposition。
func WriteAttachment(c *gin.Context, fileName, contentType string, data []byte) {
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeAttachmentName(fileName)))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, contentType, data)
}

// WriteAttachmentStream 统一输出对象存储文件流,复用安全文件名与防嗅探响应头。
func WriteAttachmentStream(c *gin.Context, fileName, contentType string, size int64, reader io.Reader) {
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeAttachmentName(fileName)))
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, size, contentType, reader, nil)
}

// PrefixReverseProxyConfig 描述挂在平台代理前缀下的内部 Web 服务。
type PrefixReverseProxyConfig struct {
	Target         *url.URL
	ProxyPath      string
	ExternalPrefix string
	ErrorHandler   func(http.ResponseWriter, *http.Request, error)
}

// NewPrefixReverseProxy 构造前缀感知反向代理,统一收敛上游跳转、Cookie 作用域和敏感转发头。
func NewPrefixReverseProxy(cfg PrefixReverseProxyConfig) *httputil.ReverseProxy {
	target := cfg.Target
	if target == nil {
		target = &url.URL{Scheme: "http"}
	}
	externalPrefix := cleanProxyPrefix(cfg.ExternalPrefix)
	return &httputil.ReverseProxy{
		ErrorHandler: cfg.ErrorHandler,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.URL.Path = cleanProxyPath(cfg.ProxyPath)
			pr.Out.Host = target.Host
			SanitizeForwardHeaders(pr.Out.Header)
			setProxyForwardedHeaders(pr, externalPrefix)
		},
		ModifyResponse: func(resp *http.Response) error {
			rewriteProxyLocation(resp.Header, target, externalPrefix)
			rewriteProxyCookies(resp.Header, externalPrefix)
			return nil
		},
	}
}

// SanitizeForwardHeaders 删除不能透传给内嵌工具的用户凭据、平台代理头和内部服务签名。
func SanitizeForwardHeaders(header http.Header) {
	for _, key := range []string{
		"Authorization",
		"Cookie",
		"Proxy-Authorization",
		"X-Api-Key",
		"X-CSRF-Token",
		"X-Forwarded-For",
		"X-Forwarded-Host",
		"X-Forwarded-Proto",
		"X-Real-IP",
		auth.ServiceNameHeader,
		auth.ServiceTenantHeader,
		auth.ServiceSourceRefHeader,
		auth.ServiceTimestampHeader,
		auth.ServiceSignatureHeader,
	} {
		header.Del(key)
	}
}

// PathID 统一解析 URL 路径 ID,非法 ID 立即写响应并阻断 handler 后续逻辑。
func PathID(c *gin.Context, name string) (int64, bool) {
	id, ok := ids.Parse(c.Param(name))
	if !ok {
		response.Fail(c, apperr.ErrPathIDInvalid)
		return 0, false
	}
	return id, true
}

// QueryIntRule 描述 HTTP 查询整数的统一解析规则,避免每种参数场景各自实现一套函数。
type QueryIntRule struct {
	BitSize int
	Default int64
	Min     int64
	Max     int64
	HasMax  bool
}

// QueryInt 按统一规则解析整数查询参数,缺失时使用 Default,非法或越界时写统一用户向错误。
func QueryInt(c *gin.Context, key string, rule QueryIntRule) (int64, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return rule.Default, true
	}
	bitSize := rule.BitSize
	if bitSize == 0 {
		bitSize = 64
	}
	value, err := strconv.ParseInt(raw, 10, bitSize)
	if err != nil || value < rule.Min || (rule.HasMax && value > rule.Max) {
		response.Fail(c, apperr.ErrQueryParamInvalid)
		return 0, false
	}
	return value, true
}

// Page 统一解析 page/size 查询参数,具体默认值和上限由 pagex 单一维护。
func Page(c *gin.Context) (int, int, bool) {
	page, ok := QueryInt(c, "page", QueryIntRule{Default: 0, Min: 0})
	if !ok {
		return 0, 0, false
	}
	size, ok := QueryInt(c, "size", QueryIntRule{Default: 0, Min: 0})
	if !ok {
		return 0, 0, false
	}
	p, s := pagex.Normalize(int(page), int(size))
	return p, s, true
}

// safeAttachmentName 把响应头文件名限制为单段可见字符,防止头注入和路径片段进入下载名。
func safeAttachmentName(fileName string) string {
	name := strings.TrimSpace(fileName)
	name = strings.ReplaceAll(name, "\\", "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '"' || r == '\\' || r == '\r' || r == '\n':
			b.WriteByte('_')
		case r >= 32 && r < 127:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" || out == "." || out == ".." {
		return "download"
	}
	return out
}

// cleanProxyPath 将路由通配路径转换为上游服务的绝对路径。
func cleanProxyPath(proxyPath string) string {
	path := strings.TrimSpace(proxyPath)
	if path == "" || path == "/" {
		return "/"
	}
	return "/" + strings.TrimPrefix(path, "/")
}

// cleanProxyPrefix 规范化浏览器可见的代理前缀,防止重写时出现双斜杠。
func cleanProxyPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return ""
	}
	return "/" + strings.Trim(strings.TrimPrefix(prefix, "/"), "/")
}

// setProxyForwardedHeaders 写入平台代理控制的转发头,不信任客户端传入的同名头。
func setProxyForwardedHeaders(pr *httputil.ProxyRequest, externalPrefix string) {
	if externalPrefix != "" {
		pr.Out.Header.Set("X-Forwarded-Prefix", externalPrefix)
	}
	if host := strings.TrimSpace(pr.In.Host); host != "" {
		pr.Out.Header.Set("X-Forwarded-Host", host)
	}
	if pr.In.TLS != nil {
		pr.Out.Header.Set("X-Forwarded-Proto", "https")
		return
	}
	pr.Out.Header.Set("X-Forwarded-Proto", "http")
}

// rewriteProxyLocation 把上游服务的根路径或同源绝对跳转收回平台代理前缀。
func rewriteProxyLocation(header http.Header, target *url.URL, externalPrefix string) {
	location := strings.TrimSpace(header.Get("Location"))
	if location == "" || externalPrefix == "" {
		return
	}
	if location == externalPrefix || strings.HasPrefix(location, externalPrefix+"/") {
		return
	}
	parsed, err := url.Parse(location)
	if err == nil && parsed.IsAbs() {
		if !strings.EqualFold(parsed.Host, target.Host) {
			return
		}
		location = parsed.RequestURI()
	}
	if !strings.HasPrefix(location, "/") || strings.HasPrefix(location, "//") {
		return
	}
	header.Set("Location", externalPrefix+location)
}

// rewriteProxyCookies 将上游 Cookie 限定在当前代理前缀下,避免同域 Cookie 互相污染。
func rewriteProxyCookies(header http.Header, externalPrefix string) {
	if externalPrefix == "" {
		return
	}
	cookies := header.Values("Set-Cookie")
	if len(cookies) == 0 {
		return
	}
	header.Del("Set-Cookie")
	for _, raw := range cookies {
		header.Add("Set-Cookie", scopeProxyCookie(raw, externalPrefix))
	}
}

// scopeProxyCookie 移除上游 Domain,并把根路径 Cookie 收敛到代理前缀。
func scopeProxyCookie(raw, externalPrefix string) string {
	parts := strings.Split(raw, ";")
	if len(parts) == 0 {
		return raw
	}
	scoped := []string{strings.TrimSpace(parts[0])}
	hasPath := false
	for _, part := range parts[1:] {
		attr := strings.TrimSpace(part)
		if attr == "" || strings.HasPrefix(strings.ToLower(attr), "domain=") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(attr), "path=") {
			hasPath = true
			path := strings.TrimSpace(attr[len("path="):])
			if path == "" || path == "/" {
				attr = "Path=" + externalPrefix
			}
		}
		scoped = append(scoped, attr)
	}
	if !hasPath {
		scoped = append(scoped, "Path="+externalPrefix)
	}
	return strings.Join(scoped, "; ")
}
