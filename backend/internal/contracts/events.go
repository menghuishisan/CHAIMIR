// contracts 定义跨模块反向通信统一使用的事件主题常量与载荷 DTO。
package contracts

import "time"

const (
	// SubjectJudgeCompleted 表示判题完成事件,由 M3 发布供业务模块订阅。
	SubjectJudgeCompleted = "judge.completed"
	// SubjectJudgeFailed 表示判题失败终态事件,由 M3 发布供业务模块订阅。
	SubjectJudgeFailed = "judge.failed"
	// SubjectSandboxRecycled 表示沙箱回收完成事件,由 M2 发布供来源模块订阅。
	SubjectSandboxRecycled = "sandbox.recycled"
	// SubjectExperimentScored 表示实验得分落定事件,由 M7 发布供上层流程消费。
	SubjectExperimentScored = "experiment.scored"
	// SubjectTeachingGradeUpdated 表示单课程成绩更新事件,由 M6 发布供 M11 重算。
	SubjectTeachingGradeUpdated = "teaching.grade.updated"
	// SubjectGradeReviewLockChanged 表示 M11 审核流程锁定态变化,由 M11 发布供 M6 同步写保护投影。
	SubjectGradeReviewLockChanged = "grade.review.lock_changed"
	// SubjectIdentitySessionRevoked 表示账号会话被吊销,由 M1 发布供 M10 踢线联动。
	SubjectIdentitySessionRevoked = "identity.session.revoked"
)

// JudgeCompletedEvent 是判题完成事件载荷。
type JudgeCompletedEvent struct {
	TenantID   int64     `json:"tenant_id"`
	TraceID    string    `json:"trace_id"`
	TaskID     int64     `json:"task_id"`
	SourceRef  string    `json:"source_ref"`
	Status     int16     `json:"status"`
	Score      int32     `json:"score"`
	Passed     bool      `json:"passed"`
	FinishedAt time.Time `json:"finished_at"`
}

// JudgeFailedEvent 是判题失败终态事件载荷。
type JudgeFailedEvent struct {
	TenantID  int64     `json:"tenant_id"`
	TraceID   string    `json:"trace_id"`
	TaskID    int64     `json:"task_id"`
	SourceRef string    `json:"source_ref"`
	Reason    string    `json:"reason"`
	FailedAt  time.Time `json:"failed_at"`
}

// SandboxRecycledEvent 是沙箱完成回收后的事件载荷。
type SandboxRecycledEvent struct {
	TenantID   int64     `json:"tenant_id"`
	TraceID    string    `json:"trace_id"`
	SandboxID  int64     `json:"sandbox_id"`
	SourceRef  string    `json:"source_ref"`
	Reason     string    `json:"reason"`
	RecycledAt time.Time `json:"recycled_at"`
}

// ExperimentScoredEvent 是实验实例得分落定后的事件载荷。
type ExperimentScoredEvent struct {
	TenantID     int64     `json:"tenant_id"`
	TraceID      string    `json:"trace_id"`
	ExperimentID int64     `json:"experiment_id"`
	InstanceID   int64     `json:"instance_id"`
	StudentID    int64     `json:"student_id"`
	Score        float64   `json:"score"`
	ScoredAt     time.Time `json:"scored_at"`
}

// TeachingGradeUpdatedEvent 是单课程成绩调整后的事件载荷。
type TeachingGradeUpdatedEvent struct {
	TenantID  int64     `json:"tenant_id"`
	TraceID   string    `json:"trace_id"`
	CourseID  int64     `json:"course_id"`
	StudentID int64     `json:"student_id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GradeReviewLockChangedEvent 是 M11 驱动 M6 同步单课程写保护投影时使用的事件载荷。
type GradeReviewLockChangedEvent struct {
	TenantID  int64     `json:"tenant_id"`
	TraceID   string    `json:"trace_id"`
	ReviewID  int64     `json:"review_id"`
	CourseID  int64     `json:"course_id"`
	Locked    bool      `json:"locked"`
	Reason    string    `json:"reason"`
	ChangedAt time.Time `json:"changed_at"`
}

// IdentitySessionRevokedEvent 是 M1 吊销会话后供 M10 关闭旧连接的事件载荷。
type IdentitySessionRevokedEvent struct {
	TenantID   int64     `json:"tenant_id"`
	TraceID    string    `json:"trace_id"`
	AccountID  int64     `json:"account_id"`
	Scope      string    `json:"scope"`
	Reason     string    `json:"reason"`
	RevokedAt  time.Time `json:"revoked_at"`
	IsPlatform bool      `json:"is_platform"`
}
