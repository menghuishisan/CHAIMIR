// pagex 测试确认全平台分页默认值与上限只有一个实现来源。
package pagex

import "testing"

// TestNormalizeAppliesDefaultAndCap 确认分页入参缺省或越界时按平台规则归一。
func TestNormalizeAppliesDefaultAndCap(t *testing.T) {
	page, size := Normalize(0, 500)
	if page != 1 || size != 100 {
		t.Fatalf("expected page=1 size=100, got page=%d size=%d", page, size)
	}

	page, size = Normalize(3, 0)
	if page != 3 || size != 20 {
		t.Fatalf("expected page=3 size=20, got page=%d size=%d", page, size)
	}
}
