// M6 服务配置测试:确认跨模块读取边界由装配注入。
package teaching

import (
	"os"
	"strings"
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

// TestNewServiceKeepsJudgeOutboxConfig 确认判题 outbox worker 的运行阈值来自装配配置。
func TestNewServiceKeepsJudgeOutboxConfig(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, config.TeachingConfig{
		JudgeOutboxBatchSize:      37,
		JudgeOutboxPollIntervalMs: 2500,
		GradeExportBatchSize:      88,
	})

	if svc.judgeOutboxBatchSize != 37 || svc.judgeOutboxPollIntervalMs != 2500 {
		t.Fatalf("judge outbox config was not injected: batch=%d interval=%d", svc.judgeOutboxBatchSize, svc.judgeOutboxPollIntervalMs)
	}
	if svc.gradeExportBatchSize != 88 {
		t.Fatalf("grade export batch size was not injected: %d", svc.gradeExportBatchSize)
	}
}

// TestJudgeOutboxBatchSizeRequiresInjectedConfig 确认服务层不再用硬编码默认值掩盖配置错误。
func TestJudgeOutboxBatchSizeRequiresInjectedConfig(t *testing.T) {
	src, err := os.ReadFile("service_assignment.go")
	if err != nil {
		t.Fatalf("read assignment service: %v", err)
	}
	fn := functionSource(string(src), "normalizedJudgeOutboxBatchSize")
	if strings.Contains(fn, "return 10") || strings.Contains(fn, "<= 0") {
		t.Fatalf("judge outbox batch size must fail at config loading instead of using a service default: %s", fn)
	}
}

// TestGradeExportBatchSizeHasNoModuleConstant 确认成绩导出分页批量来自配置,不在导出文件硬编码。
func TestGradeExportBatchSizeHasNoModuleConstant(t *testing.T) {
	src, err := os.ReadFile("service_grade_export.go")
	if err != nil {
		t.Fatalf("read grade export: %v", err)
	}
	text := string(src)
	if strings.Contains(text, "gradeExportBatchSize =") || strings.Contains(text, "const (\n\tgradeExportBatchSize") {
		t.Fatalf("grade export batch size must be injected from TeachingConfig instead of module constant")
	}
}
