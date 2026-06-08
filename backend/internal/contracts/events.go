// 事件总线 subject 常量与事件载荷类型(模块反向通信契约)。
// 依据 docs/总-工程目录设计.md §3.1.1:低层通知高层一律走事件,高层订阅。
// subject 命名:<模块>.<动作>(过去式),如 judge.completed / sandbox.recycled。
// 新增反向通信能力时先声明 subject 与载荷,再由发布方/订阅方按契约实现。
package contracts

import "time"

const (
	// SubjectJudgeCompleted 判题完成:judge 发布,teaching/experiment/contest 订阅。
	SubjectJudgeCompleted = "judge.completed"
	// SubjectJudgeFailed 判题系统性失败终态:judge 发布,teaching/experiment/contest 订阅。
	SubjectJudgeFailed = "judge.failed"
	// SubjectSandboxRecycled 沙箱回收完成:sandbox 发布,experiment 等订阅。
	SubjectSandboxRecycled = "sandbox.recycled"
	// SubjectSimSessionEnded 仿真会话结束:sim 发布。
	SubjectSimSessionEnded = "sim.session.ended"
	// SubjectExperimentScored 实验得分完成:experiment 发布,teaching/grade 等上层流程按需消费。
	SubjectExperimentScored = "experiment.scored"
	// SubjectTeachingGradeUpdated 单课程成绩变更:teaching 发布,grade 订阅后重算 GPA。
	SubjectTeachingGradeUpdated = "teaching.grade.updated"
	// SubjectNotifySend 通知发送事件:各模块发布,M10 消费为站内信。
	SubjectNotifySend = "notify.send"
	// SubjectNotifyPush 实时推送事件:各模块发布,M10 消费为 WS 广播。
	SubjectNotifyPush = "notify.push"
	// SubjectNotifyDLQ 通知事件死信:notify 重试耗尽后发布,供运维告警消费。
	SubjectNotifyDLQ = "notify.dlq"
)

// JudgeCompletedEvent 判题完成事件载荷。
type JudgeCompletedEvent struct {
	TenantID  int64  `json:"tenant_id"`
	TaskID    int64  `json:"task_id"`
	SourceRef string `json:"source_ref"` // <来源>:<年份>:<资源类型>:<id>(总-API §6)。
	Status    int16  `json:"status"`
	Score     int    `json:"score"`
}

// JudgeFailedEvent 判题失败终态事件载荷。
type JudgeFailedEvent struct {
	TenantID  int64  `json:"tenant_id"`
	TaskID    int64  `json:"task_id"`
	SourceRef string `json:"source_ref"`
	Reason    string `json:"reason"`
}

// SandboxRecycledEvent 沙箱回收事件载荷。
type SandboxRecycledEvent struct {
	TenantID  int64  `json:"tenant_id"`
	SandboxID int64  `json:"sandbox_id"`
	SourceRef string `json:"source_ref"`
	Reason    string `json:"reason"` // idle / max-lifetime / cascade / manual。
}

// SimSessionEndedEvent 仿真会话结束事件载荷。
type SimSessionEndedEvent struct {
	TenantID  int64  `json:"tenant_id"`
	SessionID int64  `json:"session_id"`
	SourceRef string `json:"source_ref"`
	Reason    string `json:"reason"` // completed / idle / cascade / manual。
}

// ExperimentScoredEvent 实验得分事件载荷。
type ExperimentScoredEvent struct {
	TenantID     int64     `json:"tenant_id"`
	ExperimentID int64     `json:"experiment_id"`
	InstanceID   int64     `json:"instance_id"`
	StudentID    int64     `json:"student_id"`
	Score        float64   `json:"score"`
	ScoredAt     time.Time `json:"scored_at"`
}

// TeachingGradeUpdatedEvent 是 M6 单课程成绩变更事件载荷。
type TeachingGradeUpdatedEvent struct {
	TenantID  int64     `json:"tenant_id"`
	CourseID  int64     `json:"course_id"`
	StudentID int64     `json:"student_id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NotifyDeadLetterEvent 是 M10 事件消费重试耗尽后的死信载荷。
type NotifyDeadLetterEvent struct {
	Subject string `json:"subject"`
	Reason  string `json:"reason"`
	Payload string `json:"payload"`
}
