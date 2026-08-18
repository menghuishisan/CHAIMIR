// timex 统一平台时间边界处理,确保写库、事件和 API 机器时间都使用 UTC。
package timex

import (
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

const dateLayout = "2006-01-02"

// Now 返回平台统一的当前 UTC 时间,避免容器本地时区渗入后端边界。
func Now() time.Time {
	return time.Now().UTC()
}

// UTC 将有效时间归一到 UTC;零值保持零值,避免破坏可选字段语义。
func UTC(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return t.UTC()
}

// ParseRFC3339 解析 API 机器时间并归一到 UTC。
func ParseRFC3339(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, err
	}
	return UTC(parsed), nil
}

// ParseDate 解析平台统一的 YYYY-MM-DD 日期格式。
func ParseDate(value string) (time.Time, error) {
	return time.Parse(dateLayout, strings.TrimSpace(value))
}

// RFC3339OrEmpty 将 API 可选时间输出为 UTC RFC3339 字符串;零值保持空字符串。
func RFC3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return UTC(t).Format(time.RFC3339)
}

// DateOrEmpty 将有效时间输出为 YYYY-MM-DD 日期;零值保持空字符串。
func DateOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return UTC(t).Format(dateLayout)
}

// Timestamptz 构造可空 PostgreSQL timestamptz,写库前统一剥离本地时区影响。
func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: UTC(t), Valid: !t.IsZero()}
}

// RequiredTimestamptz 构造必填 PostgreSQL timestamptz,用于业务已校验非空的时间字段。
func RequiredTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: UTC(t), Valid: true}
}

// FromTimestamptz 将数据库 timestamptz 转为 Go 时间;无效值返回零值。
func FromTimestamptz(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return UTC(v.Time)
}

// PtrFromTimestamptz 将数据库 timestamptz 转为时间指针;无效值返回 nil。
func PtrFromTimestamptz(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := UTC(v.Time)
	return &t
}
