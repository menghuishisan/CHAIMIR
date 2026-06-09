// M6 服务入口:定义教学服务依赖、构造函数与后台任务。
package teaching

import (
	"context"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/snowflake"
)

// Service 是 M6 教学模块服务。
type Service struct {
	repo                      *repo
	idgen                     *snowflake.Node
	auditor                   audit.Writer
	identity                  contracts.IdentityService
	content                   contracts.ContentReadService
	judge                     contracts.JudgeService
	bus                       eventbus.Bus
	courseGradesMaxRows       int
	judgeOutboxBatchSize      int
	judgeOutboxPollIntervalMs int
	gradeExportBatchSize      int
}

// NewService 构造 M6 服务。
func NewService(database *db.DB, idgen *snowflake.Node, auditor audit.Writer, identity contracts.IdentityService, content contracts.ContentReadService, judge contracts.JudgeService, bus eventbus.Bus, cfg config.TeachingConfig) *Service {
	return &Service{
		repo:                      newRepo(database),
		idgen:                     idgen,
		auditor:                   auditor,
		identity:                  identity,
		content:                   content,
		judge:                     judge,
		bus:                       bus,
		courseGradesMaxRows:       cfg.CourseGradesMaxRows,
		judgeOutboxBatchSize:      cfg.JudgeOutboxBatchSize,
		judgeOutboxPollIntervalMs: cfg.JudgeOutboxPollIntervalMs,
		gradeExportBatchSize:      cfg.GradeExportBatchSize,
	}
}

// StartJudgeOutboxWorker 轮询 M6 本地 outbox,把已提交的自动判题请求可靠派发到 M3。
func (s *Service) StartJudgeOutboxWorker(ctx context.Context) {
	background.Run(ctx, background.Task{
		Name:     "teaching.judge_outbox",
		Interval: time.Duration(s.judgeOutboxPollIntervalMs) * time.Millisecond,
		Run:      s.DispatchPendingSubmissionJudges,
	})
}
