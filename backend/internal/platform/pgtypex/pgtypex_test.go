// pgtypex 测试模块共享的 PostgreSQL 可空类型辅助函数。
package pgtypex

import (
	"math"
	"testing"
	"time"
)

// TestTextTrimsAndInvalidatesBlank 验证可选文本只在平台层统一裁剪和判空。
func TestTextTrimsAndInvalidatesBlank(t *testing.T) {
	text := Text("  name  ")
	if !text.Valid || text.String != "name" {
		t.Fatalf("Text() = (%q, %v), want trimmed valid text", text.String, text.Valid)
	}
	blank := Text("   ")
	if blank.Valid || blank.String != "" {
		t.Fatalf("Text(blank) = (%q, %v), want invalid empty text", blank.String, blank.Valid)
	}
}

// TestNumericOptionalConstructors 验证常用可空整数构造函数共用一套有效性规则。
func TestNumericOptionalConstructors(t *testing.T) {
	if v := Int8(42); !v.Valid || v.Int64 != 42 {
		t.Fatalf("Int8(42) = (%d, %v), want valid 42", v.Int64, v.Valid)
	}
	if v := Int8(0); v.Valid {
		t.Fatalf("Int8(0) should be invalid")
	}
	if v := Int2When(7, false); v.Valid {
		t.Fatalf("Int2When(..., false) should be invalid")
	}
	if v := Int4When(9, true); !v.Valid || v.Int32 != 9 {
		t.Fatalf("Int4When(9, true) = (%d, %v), want valid 9", v.Int32, v.Valid)
	}
}

// TestNullableReaders 验证可空值读取口径一致。
func TestNullableReaders(t *testing.T) {
	if got := TextValue(Text("  hello ")); got != "hello" {
		t.Fatalf("TextValue(Text()) = %q, want hello", got)
	}
	if got := IDString(Int8(123)); got != "123" {
		t.Fatalf("IDString(Int8(123)) = %q, want 123", got)
	}
}

// TestNumericHelpers 验证 numeric 构造和读取由平台层统一处理。
func TestNumericHelpers(t *testing.T) {
	n, err := Numeric(12.345)
	if err != nil {
		t.Fatalf("Numeric() error = %v", err)
	}
	if got := NumericValue(n); got != 12.35 {
		t.Fatalf("NumericValue(Numeric()) = %v, want 12.35", got)
	}
	precise, err := NumericScale(12.3456, 3)
	if err != nil {
		t.Fatalf("NumericScale() error = %v", err)
	}
	if got := NumericValue(precise); got != 12.346 {
		t.Fatalf("NumericValue(NumericScale()) = %v, want 12.346", got)
	}
}

// TestNumericRejectsInvalidFloat 验证非法浮点值不会静默进入数据库参数。
func TestNumericRejectsInvalidFloat(t *testing.T) {
	if _, err := Numeric(math.Inf(1)); err == nil {
		t.Fatalf("Numeric(+Inf) should return error")
	}
}

// TestScalarReaders 验证 smallint、int4、int8 的读取口径集中在平台层。
func TestScalarReaders(t *testing.T) {
	if got := Int2Value(Int2(7)); got != 7 {
		t.Fatalf("Int2Value(Int2(7)) = %d, want 7", got)
	}
	if got := Int8Value(Int8(42)); got != 42 {
		t.Fatalf("Int8Value(Int8(42)) = %d, want 42", got)
	}
	if got := Int4PtrValue(Int4(9)); got == nil || *got != 9 {
		t.Fatalf("Int4PtrValue(Int4(9)) = %v, want pointer to 9", got)
	}
	if got := Int4PtrValue(Int4When(0, false)); got != nil {
		t.Fatalf("Int4PtrValue(invalid) = %v, want nil", got)
	}
}

// TestDateHelpers 验证 PostgreSQL date 与 UTC 时间互转不分散到各模块。
func TestDateHelpers(t *testing.T) {
	src := time.Date(2026, 6, 8, 12, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	date := Date(src)
	if !date.Valid {
		t.Fatalf("Date() should be valid")
	}
	got := DateValue(date)
	if got.Location() != time.UTC {
		t.Fatalf("DateValue() location = %v, want UTC", got.Location())
	}
	if got.Year() != 2026 || got.Month() != 6 || got.Day() != 8 {
		t.Fatalf("DateValue() = %v, want 2026-06-08", got)
	}
}
