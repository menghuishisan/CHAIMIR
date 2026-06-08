// M1 审计查询参数测试。
package identity

import (
	"testing"
	"time"

	"chaimir/pkg/apperr"
)

// TestParseAuditTimeRangeAcceptsRFC3339 确认审计查询按文档支持 from/to 时间过滤。
func TestParseAuditTimeRangeAcceptsRFC3339(t *testing.T) {
	from, to, err := parseAuditTimeRange("2026-06-01T00:00:00Z", "2026-06-02T23:59:59Z")
	if err != nil {
		t.Fatalf("parse audit time range: %v", err)
	}
	if !from.Valid || !to.Valid {
		t.Fatalf("expected both audit time filters to be valid")
	}
	if !from.Time.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected from time: %s", from.Time)
	}
}

// TestParseAuditTimeRangeRejectsInvalidTime 确认非法时间不会被静默忽略。
func TestParseAuditTimeRangeRejectsInvalidTime(t *testing.T) {
	_, _, err := parseAuditTimeRange("not-a-time", "")
	if err != apperr.ErrIdentityAuditQueryInvalid {
		t.Fatalf("expected audit query error for invalid audit time, got %v", err)
	}
}

// TestBuildAuditQueryFilterCarriesTimeRange 确认 API 层解析出的时间会进入 service 查询条件。
func TestBuildAuditQueryFilterCarriesTimeRange(t *testing.T) {
	filter, err := buildAuditQueryFilter("123", "account.import", "2026-06-01T00:00:00Z", "2026-06-02T00:00:00Z")
	if err != nil {
		t.Fatalf("build audit query filter: %v", err)
	}
	if filter.ActorID != 123 || filter.Action != "account.import" {
		t.Fatalf("unexpected basic filter: %#v", filter)
	}
	if !filter.FromTime.Valid || !filter.ToTime.Valid {
		t.Fatalf("expected time range to be carried into filter")
	}
}

// TestBuildAuditQueryFilterRejectsInvalidActorID 确认非法 actor_id 不会被当作空过滤条件。
func TestBuildAuditQueryFilterRejectsInvalidActorID(t *testing.T) {
	_, err := buildAuditQueryFilter("not-an-id", "account.import", "", "")
	if err != apperr.ErrIdentityAuditQueryInvalid {
		t.Fatalf("expected identity audit query error for invalid actor_id, got %v", err)
	}
}
