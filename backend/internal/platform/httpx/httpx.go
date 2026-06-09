// httpx 提供 HTTP handler 层的无业务通用辅助。
package httpx

import (
	"strconv"
	"strings"

	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

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

// BindJSONWithError 允许模块在同一绑定流程中替换错误码,但仍复用统一响应信封。
func BindJSONWithError(c *gin.Context, dst any, bindErr *apperr.Error) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
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

// WritePage 把分页 service 返回值转换成统一响应,分页结构由 pkg/response 单一维护。
func WritePage(c *gin.Context, list any, total int64, page, size int, err error) {
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OKPage(c, list, total, page, size)
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

// Int 为 handler 层可选数字字段提供零值解析,必填语义应由 rules/service 校验。
func Int(v string) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0
	}
	return n
}

// Int16 复用 Int 的可选字段语义,用于枚举查询参数等 handler 轻量解析场景。
func Int16(v string) int16 {
	return int16(Int(v))
}
