// Package timex 测试平台时间边界,确保数据库和 API 时间不会受容器本地时区影响。
package timex

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestFromTimestamptzNormalizesToUTC 验证 PostgreSQL timestamptz 输出统一为 UTC。
func TestFromTimestamptzNormalizesToUTC(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	got := FromTimestamptz(pgtype.Timestamptz{
		Time:  time.Date(2026, 6, 6, 18, 30, 0, 0, shanghai),
		Valid: true,
	})

	if got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %s", got.Location())
	}
	if got.Format(time.RFC3339) != "2026-06-06T10:30:00Z" {
		t.Fatalf("unexpected UTC time: %s", got.Format(time.RFC3339))
	}
	body, err := json.Marshal(struct {
		At time.Time `json:"at"`
	}{At: got})
	if err != nil {
		t.Fatalf("marshal time: %v", err)
	}
	if string(body) != `{"at":"2026-06-06T10:30:00Z"}` {
		t.Fatalf("unexpected json time: %s", body)
	}
}

// TestOptionalBoundariesPreserveEmptyValues 验证空值在数据库边界不被误标记为有效时间。
func TestOptionalBoundariesPreserveEmptyValues(t *testing.T) {
	if got := Timestamptz(time.Time{}); got.Valid {
		t.Fatal("zero time must remain invalid when writing optional timestamptz")
	}
	if got := FromTimestamptz(pgtype.Timestamptz{}); !got.IsZero() {
		t.Fatalf("invalid timestamptz must become zero time, got %s", got)
	}
	if got := PtrFromTimestamptz(pgtype.Timestamptz{}); got != nil {
		t.Fatalf("invalid timestamptz must become nil pointer, got %v", got)
	}
}

// TestNowReturnsUTC 验证平台当前时间入口不会继承容器本地时区。
func TestNowReturnsUTC(t *testing.T) {
	if got := Now(); got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %s", got.Location())
	}
}
