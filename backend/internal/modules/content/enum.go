// content enum 文件定义 M5 内容、作者、可见性、状态和组卷枚举。
package content

const (
	// TypeExperimentTemplate 表示实验模板。
	TypeExperimentTemplate int16 = 1
	// TypeContestProblem 表示竞赛题。
	TypeContestProblem int16 = 2
	// TypeTheoryQuestion 表示理论题。
	TypeTheoryQuestion int16 = 3
)

const (
	// DifficultyIntro 表示入门难度。
	DifficultyIntro int16 = 1
	// DifficultyBasic 表示基础难度。
	DifficultyBasic int16 = 2
	// DifficultyAdvanced 表示进阶难度。
	DifficultyAdvanced int16 = 3
	// DifficultyChallenge 表示挑战难度。
	DifficultyChallenge int16 = 4
)

const (
	// AuthorTeacher 表示教师手动创建。
	AuthorTeacher int16 = 1
	// AuthorSystem 表示平台系统生成。
	AuthorSystem int16 = 2
	// AuthorExternal 表示外部来源导入。
	AuthorExternal int16 = 3
)

const (
	// VisibilityPrivate 表示仅作者/本租户受控可见。
	VisibilityPrivate int16 = 1
	// VisibilityTenant 表示本租户教师可见。
	VisibilityTenant int16 = 2
	// VisibilityShared 表示跨租户共享库可见。
	VisibilityShared int16 = 3
)

const (
	// StatusDraft 表示草稿。
	StatusDraft int16 = 1
	// StatusPublished 表示已发布。
	StatusPublished int16 = 2
	// StatusDeprecated 表示已弃用。
	StatusDeprecated int16 = 3
)

const (
	// PaperModeManual 表示手动选题组卷。
	PaperModeManual int16 = 1
	// PaperModeRandom 表示按条件随机组卷。
	PaperModeRandom int16 = 2
)

const (
	contentAuditTargetItem     = "content.item"
	contentAuditTargetCategory = "content.category"
	contentAuditTargetPaper    = "content.paper"
)
