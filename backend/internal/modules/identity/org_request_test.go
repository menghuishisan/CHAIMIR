// M1 组织接口请求结构测试。
package identity

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

// TestPromoteClassRequestDoesNotRequireMajorID 确认单班级升级只要求新名称与入学年份。
func TestPromoteClassRequestDoesNotRequireMajorID(t *testing.T) {
	req := PromoteClassRequest{Name: "计科 2301", EnrollmentYear: 2023}
	if err := validator.New().Struct(req); err != nil {
		t.Fatalf("promote class request should not require major_id: %v", err)
	}
}
