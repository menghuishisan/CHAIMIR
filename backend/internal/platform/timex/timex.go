// Package timex 统一平台时间边界处理。
// 数据库存储和 API 机器可读响应保持 UTC,展示时区由前端按用户/学校上下文转换。
package timex

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Now 返回平台统一的当前 UTC 时间,用于写库、事件和跨模块引用等后端边界。
func Now() time.Time {
	return time.Now().UTC()
}

// UTC 将有效时间归一到 UTC;零值保持零值,避免 omitempty 和可选字段语义被破坏。
func UTC(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return t.UTC()
}

// Timestamptz 构造可空 PostgreSQL timestamptz,写库前统一剥离容器本地时区影响。
func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: UTC(t), Valid: !t.IsZero()}
}

// RequiredTimestamptz 构造必填 PostgreSQL timestamptz,用于业务已校验非空的时间字段。
func RequiredTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: UTC(t), Valid: true}
}

// FromTimestamptz 将数据库 timestamptz 转为 API DTO 时间;无效值返回零值。
func FromTimestamptz(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return UTC(v.Time)
}

// PtrFromTimestamptz 将数据库 timestamptz 转为 API DTO 时间指针;无效值返回 nil。
func PtrFromTimestamptz(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := UTC(v.Time)
	return &t
}
