// M10 转换工具:统一处理 ID、PostgreSQL 可空类型与请求解析。
package notify

import (
	"strings"
	"time"
)

// parseOptionalDateTime 解析公告过期时间,空值表示不过期。
func parseOptionalDateTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.DateTime, raw); err == nil {
		return t, true
	}
	return time.Time{}, false
}
