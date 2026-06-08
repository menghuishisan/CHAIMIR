// M5 枚举定义:集中维护内容类型、难度、作者来源、可见范围、状态与组卷模式。
package content

const (
	// ContentTypeExperimentTemplate 表示实验模板。
	ContentTypeExperimentTemplate int16 = 1
	// ContentTypeContestProblem 表示竞赛题。
	ContentTypeContestProblem int16 = 2
	// ContentTypeTheoryQuestion 表示理论题。
	ContentTypeTheoryQuestion int16 = 3
)

const (
	// DifficultyIntro 表示入门难度。
	DifficultyIntro int16 = 1
	// DifficultyAdvanced 表示进阶难度。
	DifficultyAdvanced int16 = 2
	// DifficultyExpert 表示高级难度。
	DifficultyExpert int16 = 3
	// DifficultyResearch 表示研究难度。
	DifficultyResearch int16 = 4
)

const (
	// AuthorTypeTeacher 表示教师创作内容。
	AuthorTypeTeacher int16 = 1
	// AuthorTypeSystem 表示系统自动建题。
	AuthorTypeSystem int16 = 2
	// AuthorTypeExternal 表示外部源固化内容。
	AuthorTypeExternal int16 = 3
)

const (
	// VisibilityPrivate 表示仅作者私有。
	VisibilityPrivate int16 = 1
	// VisibilityTenant 表示本租户可见。
	VisibilityTenant int16 = 2
	// VisibilityShared 表示跨校共享库可见。
	VisibilityShared int16 = 3
)

const (
	// ItemStatusDraft 表示草稿态。
	ItemStatusDraft int16 = 1
	// ItemStatusPublished 表示已发布定版。
	ItemStatusPublished int16 = 2
	// ItemStatusDeprecated 表示已弃用。
	ItemStatusDeprecated int16 = 3
)

const (
	// PaperGenManual 表示手动选题。
	PaperGenManual int16 = 1
	// PaperGenRandom 表示按条件随机抽题。
	PaperGenRandom int16 = 2
)

const initialVersion = "1.0.0"
