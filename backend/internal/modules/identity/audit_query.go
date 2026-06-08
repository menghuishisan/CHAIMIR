// M1 审计查询条件解析与传递结构。
package identity

import (
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// AuditQueryFilter 是审计日志查询的业务过滤条件。
type AuditQueryFilter struct {
	ActorID    int64
	Action     string
	TargetType string
	FromTime   pgtype.Timestamptz
	ToTime     pgtype.Timestamptz
}

// buildAuditQueryFilter 从 HTTP 查询参数构造 service 使用的过滤条件。
func buildAuditQueryFilter(actorIDText, action, fromText, toText string) (AuditQueryFilter, error) {
	var actorID int64
	if actorIDText != "" {
		v, ok := ids.Parse(actorIDText)
		if !ok {
			return AuditQueryFilter{}, apperr.ErrIdentityAuditQueryInvalid
		}
		actorID = v
	}
	from, to, err := parseAuditTimeRange(fromText, toText)
	if err != nil {
		return AuditQueryFilter{}, err
	}
	return AuditQueryFilter{
		ActorID:  actorID,
		Action:   action,
		FromTime: from,
		ToTime:   to,
	}, nil
}

// parseAuditTimeRange 按 RFC3339 解析审计查询时间范围;空值表示不过滤。
func parseAuditTimeRange(fromText, toText string) (pgtype.Timestamptz, pgtype.Timestamptz, error) {
	from, err := parseOptionalAuditTime(fromText)
	if err != nil {
		return pgtype.Timestamptz{}, pgtype.Timestamptz{}, err
	}
	to, err := parseOptionalAuditTime(toText)
	if err != nil {
		return pgtype.Timestamptz{}, pgtype.Timestamptz{}, err
	}
	if from.Valid && to.Valid && from.Time.After(to.Time) {
		return pgtype.Timestamptz{}, pgtype.Timestamptz{}, apperr.ErrIdentityAuditQueryInvalid
	}
	return from, to, nil
}

// parseOptionalAuditTime 解析单个审计时间参数,避免非法时间被当作空过滤条件。
func parseOptionalAuditTime(v string) (pgtype.Timestamptz, error) {
	if v == "" {
		return pgtype.Timestamptz{}, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return pgtype.Timestamptz{}, apperr.ErrIdentityAuditQueryInvalid
	}
	return timex.RequiredTimestamptz(t), nil
}
