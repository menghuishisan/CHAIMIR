// M8 DTO 定义:隔离 HTTP 请求/响应、服务内部编排结构与跨模块引用。
package contest

import "time"

// ContestRequest 是创建和更新竞赛定义的请求。
type ContestRequest struct {
	Name          string         `json:"name"`
	Mode          int16          `json:"mode"`
	MatchMode     int16          `json:"match_mode"`
	TeamMode      int16          `json:"team_mode"`
	SignupStart   time.Time      `json:"signup_start"`
	SignupEnd     time.Time      `json:"signup_end"`
	StartAt       time.Time      `json:"start_at"`
	EndAt         time.Time      `json:"end_at"`
	FreezeMinutes int32          `json:"freeze_minutes"`
	Rules         map[string]any `json:"rules"`
}

// ContestDTO 是竞赛定义响应。
type ContestDTO struct {
	ID            string         `json:"id"`
	TenantID      string         `json:"tenant_id,omitempty"`
	OrganizerID   string         `json:"organizer_id,omitempty"`
	Name          string         `json:"name"`
	Mode          int16          `json:"mode"`
	MatchMode     int16          `json:"match_mode,omitempty"`
	TeamMode      int16          `json:"team_mode"`
	SignupStart   time.Time      `json:"signup_start"`
	SignupEnd     time.Time      `json:"signup_end"`
	StartAt       time.Time      `json:"start_at"`
	EndAt         time.Time      `json:"end_at"`
	FreezeMinutes int32          `json:"freeze_minutes"`
	Rules         map[string]any `json:"rules"`
	Status        int16          `json:"status"`
	CreatedAt     time.Time      `json:"created_at,omitempty"`
	UpdatedAt     time.Time      `json:"updated_at,omitempty"`
}

// ContestProblemRequest 是竞赛题目编排请求。
type ContestProblemRequest struct {
	ItemCode     string         `json:"item_code"`
	ItemVersion  string         `json:"item_version"`
	Score        int32          `json:"score"`
	DynamicScore map[string]any `json:"dynamic_score"`
	BattleRule   int16          `json:"battle_rule"`
	Seq          int32          `json:"seq"`
}

// ContestProblemDTO 是竞赛题目引用响应。
type ContestProblemDTO struct {
	ID           string         `json:"id"`
	ContestID    string         `json:"contest_id"`
	ItemCode     string         `json:"item_code"`
	ItemVersion  string         `json:"item_version"`
	Score        int32          `json:"score"`
	DynamicScore map[string]any `json:"dynamic_score"`
	BattleRule   int16          `json:"battle_rule,omitempty"`
	Seq          int32          `json:"seq"`
	Face         map[string]any `json:"face,omitempty"`
}

// SignupRequest 是报名或创建队伍请求。
type SignupRequest struct {
	Name string `json:"name"`
}

// JoinTeamRequest 是邀请码入队请求。
type JoinTeamRequest struct {
	InviteCode string `json:"invite_code"`
}

// TeamDTO 是队伍响应。
type TeamDTO struct {
	ID         string          `json:"id"`
	ContestID  string          `json:"contest_id"`
	Name       string          `json:"name"`
	InviteCode string          `json:"invite_code,omitempty"`
	Status     int16           `json:"status"`
	Members    []TeamMemberDTO `json:"members"`
	CreatedAt  time.Time       `json:"created_at,omitempty"`
}

// TeamMemberDTO 是队员响应。
type TeamMemberDTO struct {
	ID             string `json:"id"`
	TeamID         string `json:"team_id"`
	AccountID      string `json:"account_id"`
	MemberTenantID string `json:"member_tenant_id"`
	IsLeader       bool   `json:"is_leader"`
}

// StartProblemEnvRequest 是实操题环境创建请求。
type StartProblemEnvRequest struct {
	RuntimeCode     string   `json:"runtime_code"`
	ToolCodes       []string `json:"tools"`
	InitCodeRef     string   `json:"init_code_ref"`
	InitScriptRef   string   `json:"init_script_ref"`
	KeepAlive       bool     `json:"keep_alive"`
	SnapshotEnabled bool     `json:"snapshot_enabled"`
}

// ProblemEnvDTO 是 M2 沙箱环境摘要。
type ProblemEnvDTO struct {
	SandboxID string `json:"sandbox_id"`
	SourceRef string `json:"source_ref"`
	Status    int16  `json:"status"`
}

// SolveSubmitRequest 是解题赛提交请求。
type SolveSubmitRequest struct {
	TeamID         string         `json:"team_id"`
	ContentRef     map[string]any `json:"content_ref"`
	CodeStorageKey string         `json:"code_storage_key"`
	CodeHash       string         `json:"code_hash"`
	JudgerCode     string         `json:"judger_code"`
	SandboxRef     string         `json:"sandbox_ref"`
	ExtraInput     map[string]any `json:"extra_input"`
}

// SolveSubmissionDTO 是解题提交响应。
type SolveSubmissionDTO struct {
	ID           string         `json:"id"`
	ContestID    string         `json:"contest_id"`
	ProblemID    string         `json:"problem_id"`
	TeamID       string         `json:"team_id"`
	SubmitterID  string         `json:"submitter_id"`
	ContentRef   map[string]any `json:"content_ref"`
	SourceRef    string         `json:"source_ref,omitempty"`
	JudgeTaskRef string         `json:"judge_task_ref,omitempty"`
	Passed       bool           `json:"passed"`
	Score        int32          `json:"score"`
	SandboxRef   string         `json:"sandbox_ref,omitempty"`
	SubmittedAt  time.Time      `json:"submitted_at,omitempty"`
}

// BattleEntryRequest 是对抗赛参战物提交请求。
type BattleEntryRequest struct {
	TeamID      string `json:"team_id"`
	Role        int16  `json:"role"`
	ArtifactRef string `json:"artifact_ref"`
}

// BattleEntryDTO 是参战物响应。
type BattleEntryDTO struct {
	ID          string    `json:"id"`
	ContestID   string    `json:"contest_id"`
	TeamID      string    `json:"team_id"`
	Role        int16     `json:"role"`
	ArtifactRef string    `json:"artifact_ref"`
	VersionNo   int32     `json:"version_no"`
	IsActive    bool      `json:"is_active"`
	SubmittedAt time.Time `json:"submitted_at,omitempty"`
}

// BattleMatchResult 是撮合 worker 结算一场对局的输入。
type BattleMatchResult struct {
	ContestID  int64
	EntryAID   int64
	EntryBID   int64
	SandboxRef string
	Result     int16
	ReplayRef  string
}

// BattleMatchDTO 是对抗对局响应。
type BattleMatchDTO struct {
	ID         string         `json:"id"`
	ContestID  string         `json:"contest_id"`
	EntryAID   string         `json:"entry_a_id"`
	EntryBID   string         `json:"entry_b_id"`
	SandboxRef string         `json:"sandbox_ref"`
	Result     int16          `json:"result"`
	ScoreDelta map[string]any `json:"score_delta"`
	ReplayRef  string         `json:"replay_ref"`
	MatchedAt  time.Time      `json:"matched_at,omitempty"`
	FinishedAt time.Time      `json:"finished_at,omitempty"`
}

// LadderRankDTO 是排行榜响应。
type LadderRankDTO struct {
	ID          string    `json:"id,omitempty"`
	ContestID   string    `json:"contest_id,omitempty"`
	TeamID      string    `json:"team_id"`
	Score       float64   `json:"score"`
	SolvedCount int32     `json:"solved_count"`
	Rank        int32     `json:"rank"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// ResultSnapshotDTO 是竞赛归档成绩快照。
type ResultSnapshotDTO struct {
	ID           string          `json:"id"`
	ContestID    string          `json:"contest_id"`
	FinalRanking []LadderRankDTO `json:"final_ranking"`
	GeneratedAt  time.Time       `json:"generated_at,omitempty"`
}

// CheatRecordRequest 是作弊判定记录请求。
type CheatRecordRequest struct {
	TeamID   string         `json:"team_id"`
	Type     int16          `json:"type"`
	Evidence map[string]any `json:"evidence"`
	Action   int16          `json:"action"`
}

// CheatRecordDTO 是作弊判定响应。
type CheatRecordDTO struct {
	ID         string         `json:"id"`
	ContestID  string         `json:"contest_id"`
	TeamID     string         `json:"team_id"`
	Type       int16          `json:"type"`
	Evidence   map[string]any `json:"evidence"`
	Action     int16          `json:"action"`
	OperatorID string         `json:"operator_id,omitempty"`
	CreatedAt  time.Time      `json:"created_at,omitempty"`
}

// VulnSourceRequest 是漏洞源配置请求。
type VulnSourceRequest struct {
	Type         int16          `json:"type"`
	Name         string         `json:"name"`
	Config       map[string]any `json:"config"`
	DefaultLevel int16          `json:"default_level"`
	Enabled      bool           `json:"enabled"`
}

// VulnSourceDTO 是漏洞源配置响应。
type VulnSourceDTO struct {
	ID           string         `json:"id"`
	TenantID     string         `json:"tenant_id,omitempty"`
	Type         int16          `json:"type"`
	Name         string         `json:"name"`
	Config       map[string]any `json:"config"`
	DefaultLevel int16          `json:"default_level"`
	Enabled      bool           `json:"enabled"`
	LastSyncAt   time.Time      `json:"last_sync_at,omitempty"`
}

// VulnSyncResultDTO 是漏洞源同步结果响应。
type VulnSyncResultDTO struct {
	SourceID      string           `json:"source_id"`
	ImportedCount int              `json:"imported_count"`
	Problems      []VulnProblemDTO `json:"problems"`
}

// VulnProblemImportRequest 是手工导入漏洞案例请求。
type VulnProblemImportRequest struct {
	SourceID    string         `json:"source_id"`
	ExternalRef string         `json:"external_ref"`
	Title       string         `json:"title"`
	Level       int16          `json:"level"`
	RuntimeMode int16          `json:"runtime_mode"`
	DraftBody   map[string]any `json:"draft_body"`
}

// VulnPrevalidateRequest 是漏洞题预验证结果写入请求。
type VulnPrevalidateRequest struct {
	Passed bool           `json:"passed"`
	Detail map[string]any `json:"detail"`
}

// VulnFinalizeRequest 是漏洞题固化入 M5 的请求补充。
type VulnFinalizeRequest struct {
	Code            string   `json:"code"`
	Version         string   `json:"version"`
	CategoryID      string   `json:"category_id"`
	Difficulty      int16    `json:"difficulty"`
	Tags            []string `json:"tags"`
	KnowledgePoints []string `json:"knowledge_points"`
	SensitiveFields []string `json:"sensitive_fields"`
}

// VulnProblemDTO 是漏洞题转化草稿响应。
type VulnProblemDTO struct {
	ID                 string         `json:"id"`
	SourceID           string         `json:"source_id,omitempty"`
	ExternalRef        string         `json:"external_ref,omitempty"`
	Title              string         `json:"title"`
	Level              int16          `json:"level"`
	RuntimeMode        int16          `json:"runtime_mode"`
	DraftBody          map[string]any `json:"draft_body"`
	PrevalidateStatus  int16          `json:"prevalidate_status"`
	PrevalidateDetail  map[string]any `json:"prevalidate_detail"`
	ContentItemCode    string         `json:"content_item_code,omitempty"`
	ContentItemVersion string         `json:"content_item_version,omitempty"`
	Status             int16          `json:"status"`
}

// StatsDTO 是 M8 给内部看板使用的统计响应。
type StatsDTO struct {
	TenantID           string `json:"tenant_id"`
	ContestCount       int64  `json:"contest_count"`
	ActiveContestCount int64  `json:"active_contest_count"`
	TeamCount          int64  `json:"team_count"`
}

// ContestAchievementDTO 是竞赛成就只读聚合响应。
type ContestAchievementDTO struct {
	ContestID string  `json:"contest_id"`
	TeamID    string  `json:"team_id"`
	Score     float64 `json:"score"`
	Rank      int32   `json:"rank"`
}

// pendingSolveSubmission 是等待判题事件回写的提交定位信息。
type pendingSolveSubmission struct {
	ID        int64
	TenantID  int64
	ContestID int64
	TeamID    int64
	ProblemID int64
	SourceRef string
	MaxScore  int32
}
