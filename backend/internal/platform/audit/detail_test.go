// audit_test 校验审计详情序列化边界,避免模块各自定义 JSON 语义。
package audit

import (
	"testing"

	"chaimir/pkg/apperr"
)

// TestDetailStringUsesStableObjectSemantics 验证空审计详情统一保存为空 JSON 对象。
func TestDetailStringUsesStableObjectSemantics(t *testing.T) {
	got, err := DetailString(nil)
	if err != nil {
		t.Fatalf("detail string nil: %v", err)
	}
	if got != "{}" {
		t.Fatalf("unexpected nil detail: %s", got)
	}
}

// TestDetailStringReturnsInternalError 验证非法详情不会被静默写入审计表。
func TestDetailStringReturnsInternalError(t *testing.T) {
	_, err := DetailString(map[string]any{"bad": make(chan int)})
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrInternal.Code {
		t.Fatalf("expected internal app error, got %v", err)
	}
}
