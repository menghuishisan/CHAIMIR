// httpx 提供 HTTP handler 层的无业务通用辅助。
package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

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
