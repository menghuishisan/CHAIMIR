// 定义运维告警的租户级只读汇总，不传递业务明细或数据库对象。
package contracts

import (
	"context"
	"time"
)

// AlertMetricQuery 为所有指标提供同一采样时点与滑动窗口。
type AlertMetricQuery struct {
	TenantID int64     `json:"tenant_id"`
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
}

// AlertMetricReader 只返回实现方拥有的指标；读取失败必须返回错误，不能填零。
type AlertMetricReader interface {
	AlertMetrics(context.Context, AlertMetricQuery) (map[string]float64, error)
}

// AlertRecipientReader 只解析本校有效管理员，不返回账号明细。
type AlertRecipientReader interface {
	AlertRecipients(context.Context, int64) ([]int64, error)
}
