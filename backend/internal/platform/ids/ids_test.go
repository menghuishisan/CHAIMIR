// Package ids 的测试:统一雪花 ID 与外部字符串之间的解析和格式化规则。
package ids

import "testing"

// TestFormatAndParseRoundTrip 确认正整数 ID 可以稳定字符串化并解析回来。
func TestFormatAndParseRoundTrip(t *testing.T) {
	raw := int64(9007199254740993)
	text := Format(raw)
	if text != "9007199254740993" {
		t.Fatalf("unexpected formatted id: %q", text)
	}
	got, ok := Parse(text)
	if !ok || got != raw {
		t.Fatalf("expected parse round trip, got id=%d ok=%v", got, ok)
	}
}

// TestParseRejectsInvalidIDs 确认空值、非数字和非正数都不会被当作有效业务 ID。
func TestParseRejectsInvalidIDs(t *testing.T) {
	for _, raw := range []string{"", "abc", "0", "-1"} {
		if got, ok := Parse(raw); ok || got != 0 {
			t.Fatalf("expected %q to be invalid, got id=%d ok=%v", raw, got, ok)
		}
	}
}
