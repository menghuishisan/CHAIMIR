// pgtypex 统一提供后端模块复用的 PostgreSQL 可空类型辅助函数。
package pgtypex

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"

	"github.com/jackc/pgx/v5/pgtype"
)

// Text 构造可空文本,会先裁剪可选输入两端空白。
func Text(v string) pgtype.Text {
	v = strings.TrimSpace(v)
	return pgtype.Text{String: v, Valid: v != ""}
}

// TextValue 读取可空文本,数据库 NULL 统一转为空字符串。
func TextValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

// Int8 构造可空 int8,非正数按可选 ID 或过滤条件缺省处理。
func Int8(v int64) pgtype.Int8 {
	return Int8When(v, v > 0)
}

// Int8When 按显式有效标记构造可空 int8。
func Int8When(v int64, valid bool) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: valid}
}

// Int8Value 读取可空 int8,数据库 NULL 统一转为零值。
func Int8Value(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

// Int2 构造可空 int2,非正数按可选过滤条件缺省处理。
func Int2(v int16) pgtype.Int2 {
	return Int2When(v, v > 0)
}

// Int2When 按显式有效标记构造可空 int2。
func Int2When(v int16, valid bool) pgtype.Int2 {
	return pgtype.Int2{Int16: v, Valid: valid}
}

// Int2Value 读取可空 int2,数据库 NULL 统一转为零值。
func Int2Value(v pgtype.Int2) int16 {
	if !v.Valid {
		return 0
	}
	return v.Int16
}

// Int4 构造有效的可空 int4 数值。
func Int4(v int32) pgtype.Int4 {
	return Int4When(v, true)
}

// Int4When 按显式有效标记构造可空 int4。
func Int4When(v int32, valid bool) pgtype.Int4 {
	return pgtype.Int4{Int32: v, Valid: valid}
}

// Int4Ptr 从可选指针构造可空 int4。
func Int4Ptr(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return Int4(*v)
}

// Int4Value 读取可空 int4,数据库 NULL 统一转为零值。
func Int4Value(v pgtype.Int4) int32 {
	if !v.Valid {
		return 0
	}
	return v.Int32
}

// Int4PtrValue 读取可空 int4 指针值。
func Int4PtrValue(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	return &v.Int32
}

// BoolPtr 从可选指针构造可空 bool。
func BoolPtr(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}

// Numeric 按平台默认两位小数构造 PostgreSQL numeric。
func Numeric(v float64) (pgtype.Numeric, error) {
	return NumericScale(v, 2)
}

// NumericScale 按显式小数位构造 PostgreSQL numeric,非法浮点值必须显式返回错误。
func NumericScale(v float64, scale int) (pgtype.Numeric, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return pgtype.Numeric{}, fmt.Errorf("数值不是有限数字: %v", v)
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(v, 'f', scale, 64)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

// NumericPtr 从可选浮点指针构造可空 PostgreSQL numeric。
func NumericPtr(v *float64) (pgtype.Numeric, error) {
	if v == nil {
		return pgtype.Numeric{}, nil
	}
	return Numeric(*v)
}

// NumericValue 读取 PostgreSQL numeric,数据库 NULL 或非法值统一转为零值。
func NumericValue(v pgtype.Numeric) float64 {
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

// NumericPtrValue 读取可空 PostgreSQL numeric 指针值。
func NumericPtrValue(v pgtype.Numeric) *float64 {
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	return &f.Float64
}

// Date 构造 PostgreSQL date,零值时间按 NULL 处理。
func Date(v time.Time) pgtype.Date {
	if v.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: timex.UTC(v), Valid: true}
}

// DateValue 读取 PostgreSQL date,数据库 NULL 统一转为零值时间。
func DateValue(v pgtype.Date) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return timex.UTC(v.Time)
}

// TimestamptzPtr 从可选时间指针构造 PostgreSQL timestamptz,保持 nil 与零值时间的空值语义一致。
func TimestamptzPtr(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return timex.Timestamptz(*v)
}

// IDString 把可空 int8 雪花 ID 转为 JSON DTO 使用的字符串。
func IDString(v pgtype.Int8) string {
	if !v.Valid {
		return ""
	}
	return ids.Format(v.Int64)
}
