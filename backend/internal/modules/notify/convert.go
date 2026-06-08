// M10 转换工具:统一处理 ID、PostgreSQL 可空类型与请求解析。
package notify

import (
	"strings"
	"time"

	"chaimir/internal/platform/ids"

	"github.com/jackc/pgx/v5/pgtype"
)

// pgText 构造可空文本。
func pgText(v string) pgtype.Text {
	v = strings.TrimSpace(v)
	return pgtype.Text{String: v, Valid: v != ""}
}

// pgBool 构造可空布尔。
func pgBool(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}

// pgInt8 构造可空 int8。
func pgInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v > 0}
}

// textValue 读取可空文本。
func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

// optionalID 转换可空 ID。
func optionalID(v pgtype.Int8) string {
	if !v.Valid {
		return ""
	}
	return ids.Format(v.Int64)
}

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
