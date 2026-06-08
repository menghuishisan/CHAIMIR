// M7 枚举定义:集中维护实验定义、实例、协作与报告状态常量。
package experiment

const (
	// ExperimentStatusDraft 表示实验仍处于向导草稿态。
	ExperimentStatusDraft int16 = 1
	// ExperimentStatusPublished 表示实验已发布,可被学生发起实例。
	ExperimentStatusPublished int16 = 2
	// ExperimentStatusUnpublished 表示实验已下架,保留历史实例与结果。
	ExperimentStatusUnpublished int16 = 3
)

const (
	// CollabModeSingle 表示单人实验。
	CollabModeSingle int16 = 1
	// CollabModeGroup 表示小组共享环境实验。
	CollabModeGroup int16 = 2
)

const (
	// InstanceStatusCreating 表示实例正在拉起沙箱或仿真组件。
	InstanceStatusCreating int16 = 1
	// InstanceStatusRunning 表示所有必需组件就绪,学生可进入工作台。
	InstanceStatusRunning int16 = 2
	// InstanceStatusPaused 表示实例被暂停,仍可恢复。
	InstanceStatusPaused int16 = 3
	// InstanceStatusCompleted 表示学生已完成实验,结果已保留。
	InstanceStatusCompleted int16 = 4
	// InstanceStatusRecycled 表示引擎资源已释放。
	InstanceStatusRecycled int16 = 5
	// InstanceStatusError 表示组件拉起或编排失败。
	InstanceStatusError int16 = 6
	// InstanceStatusReleased 表示底层沙箱已独立回收,恢复时需要重建环境。
	InstanceStatusReleased int16 = 7
)

const (
	// ReportStatusSubmitted 表示学生已提交实验报告。
	ReportStatusSubmitted int16 = 1
	// ReportStatusGraded 表示教师已批改实验报告。
	ReportStatusGraded int16 = 2
)
