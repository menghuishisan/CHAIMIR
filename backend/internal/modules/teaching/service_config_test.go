// M6 服务配置测试:确认跨模块读取边界由装配注入。
package teaching

import (
	"testing"

	"chaimir/internal/platform/config"
)

// TestNewServiceKeepsCourseGradeLimit 确认课程成绩读取上限不在服务内硬编码。
func TestNewServiceKeepsCourseGradeLimit(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, config.TeachingConfig{CourseGradesMaxRows: 3000})

	if svc.courseGradesMaxRows != 3000 {
		t.Fatalf("course grade limit was not injected: %d", svc.courseGradesMaxRows)
	}
}
