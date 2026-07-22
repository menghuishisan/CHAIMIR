// contest model 文件定义 M8 领域模型和内部快照。
package contest

import "time"

// Contest 是竞赛定义领域模型。
type Contest struct {
	ID            int64
	TenantID      int64
	OrganizerID   int64
	Name          string
	Mode          int16
	MatchMode     int16
	TeamMode      int16
	SignupStart   time.Time
	SignupEnd     time.Time
	StartAt       time.Time
	EndAt         time.Time
	FreezeMinutes int32
	Rules         map[string]any
	Status        int16
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ContestProblem 是竞赛内题目引用和赛内配置。
type ContestProblem struct {
	ID           int64
	TenantID     int64
	ContestID    int64
	ItemCode     string
	ItemVersion  string
	Score        int32
	DynamicScore map[string]any
	BattleConfig map[string]any
	BattleRule   int16
	Seq          int32
}

// Team 是参赛队伍,个人赛也以单人队建模。
type Team struct {
	ID         int64
	TenantID   int64
	ContestID  int64
	Name       string
	InviteCode string
	Status     int16
	CreatedAt  time.Time
	Members    []TeamMember
}

// TeamMember 是队伍成员及跨校归属。
type TeamMember struct {
	ID             int64
	TenantID       int64
	TeamID         int64
	AccountID      int64
	MemberTenantID int64
	IsLeader       bool
	JoinedAt       time.Time
}

// SolveSubmission 是一次解题赛提交。
type SolveSubmission struct {
	ID           int64
	TenantID     int64
	ContestID    int64
	ProblemID    int64
	TeamID       int64
	SubmitterID  int64
	ContentRef   map[string]any
	SourceRef    string
	JudgeTaskRef string
	Passed       bool
	Score        int32
	SandboxRef   string
	SubmittedAt  time.Time
}

// BattleEntry 是对抗赛参战物。
type BattleEntry struct {
	ID           int64
	TenantID     int64
	ContestID    int64
	ProblemID    int64
	TeamID       int64
	Role         int16
	ArtifactRef  string
	ArtifactHash string
	VersionNo    int32
	IsActive     bool
	SubmittedAt  time.Time
}

// BattleMatch 是异步撮合产生的一场对局。
type BattleMatch struct {
	ID           int64
	TenantID     int64
	ContestID    int64
	ProblemID    int64
	EntryAID     int64
	EntryBID     int64
	SourceRef    string
	SandboxRef   string
	JudgeTaskRef string
	Result       int16
	ScoreDelta   map[string]any
	ReplayRef    string
	Status       int16
	MatchedAt    time.Time
	FinishedAt   time.Time
}

// LadderRank 是排行榜积分投影。
type LadderRank struct {
	ID          int64
	TenantID    int64
	ContestID   int64
	TeamID      int64
	Score       float64
	SolvedCount int32
	LastSolveAt time.Time
	Rank        int32
	UpdatedAt   time.Time
}

// LadderSnapshot 是封榜或归档时固化的排行榜快照。
type LadderSnapshot struct {
	ID             int64
	TenantID       int64
	ContestID      int64
	SnapshotStatus int16
	Ranking        []map[string]any
	GeneratedAt    time.Time
}

// CheatRecord 是教师或管理员确认后的违规处理记录。
type CheatRecord struct {
	ID         int64
	TenantID   int64
	ContestID  int64
	TeamID     int64
	Type       int16
	Evidence   map[string]any
	Action     int16
	OperatorID int64
	CreatedAt  time.Time
}

// VulnSource 是真实漏洞外部源配置。
type VulnSource struct {
	ID           int64
	TenantID     int64
	Type         int16
	Name         string
	Config       map[string]any
	DefaultLevel int16
	Enabled      bool
	LastSyncAt   time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// VulnProblem 是漏洞题转化草稿和预验证记录。
type VulnProblem struct {
	ID                 int64
	TenantID           int64
	SourceID           int64
	ExternalRef        string
	Title              string
	Level              int16
	RuntimeMode        int16
	DraftBody          map[string]any
	PrevalidateStatus  int16
	PrevalidateDetail  map[string]any
	ContentItemCode    string
	ContentItemVersion string
	Status             int16
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ContestStatsSnapshot 是 M8 输出给聚合层的统计投影。
type ContestStatsSnapshot struct {
	ContestCount       int64
	ActiveContestCount int64
	ParticipantCount   int64
}

// StudentContestRecord 是学生个人竞赛战绩派生投影。
type StudentContestRecord struct {
	ContestID     int64
	TeamID        int64
	Score         float64
	Rank          int32
	ContestName   string
	ContestStatus int16
}
