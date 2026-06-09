// timex_test 校验平台统一时间入口的 UTC 与数据库边界语义。
package timex

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestFromTimestamptzNormalizesToUTC 确认数据库时间输出统一归一到 UTC。
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

// TestOptionalBoundariesPreserveEmptyValues 确认可空时间在数据库边界不会被误写为有效值。
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

// TestNowReturnsUTC 确认统一当前时间入口不会泄露容器本地时区。
func TestNowReturnsUTC(t *testing.T) {
	if got := Now(); got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %s", got.Location())
	}
}
