// experiment enum 文件定义 M7 实验、实例、协作和报告状态常量。
package experiment

const (
	// CollabModeSolo 表示单人实验。
	CollabModeSolo int16 = 1
	// CollabModeGroup 表示小组协作实验。
	CollabModeGroup int16 = 2
)

const (
	// ExperimentStatusDraft 表示实验定义处于草稿向导态。
	ExperimentStatusDraft int16 = 1
	// ExperimentStatusPublished 表示实验已发布可被发起实例。
	ExperimentStatusPublished int16 = 2
	// ExperimentStatusUnpublished 表示实验已下架不可新发起。
	ExperimentStatusUnpublished int16 = 3
)

const (
	// InstanceStatusCreating 表示实例正在创建引擎资源。
	InstanceStatusCreating int16 = 1
	// InstanceStatusRunning 表示实例正在进行。
	InstanceStatusRunning int16 = 2
	// InstanceStatusPaused 表示实例已暂停。
	InstanceStatusPaused int16 = 3
	// InstanceStatusFinished 表示实例已完成并保留结果。
	InstanceStatusFinished int16 = 4
	// InstanceStatusRecycled 表示实例引擎资源已释放。
	InstanceStatusRecycled int16 = 5
	// InstanceStatusError 表示实例创建或恢复失败。
	InstanceStatusError int16 = 6
	// InstanceStatusReleased 表示底层沙箱被独立回收,等待恢复重建。
	InstanceStatusReleased int16 = 7
)

const (
	// ReportStatusSubmitted 表示报告已提交未批改。
	ReportStatusSubmitted int16 = 1
	// ReportStatusGraded 表示报告已批改。
	ReportStatusGraded int16 = 2
)

const (
	// ValidationLevelError 表示阻断发布的问题。
	ValidationLevelError = "error"
	// ValidationLevelWarning 表示允许发布但需要教师关注的问题。
	ValidationLevelWarning = "warning"
)

const (
	// progressChannelName 是 M7 统一返回给 M10 订阅的频道名。
	progressChannelName = "progress"
)
