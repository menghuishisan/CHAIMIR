// M9 转换工具:统一处理 ID、JSONB 与 PostgreSQL 可空类型。
package admin

import (
	"strings"

	"chaimir/internal/platform/ids"

	"github.com/jackc/pgx/v5/pgtype"
)

// pgInt8 构造可空 int8。
func pgInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v > 0}
}

// pgInt2 构造可空 int2。
func pgInt2(v int16) pgtype.Int2 {
	return pgtype.Int2{Int16: v, Valid: v > 0}
}

// pgText 构造可空文本。
func pgText(v string) pgtype.Text {
	return pgtype.Text{String: v, Valid: strings.TrimSpace(v) != ""}
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
