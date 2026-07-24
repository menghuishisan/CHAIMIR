// contest dto 文件定义 M8 HTTP 请求和响应结构,不承载业务逻辑。
package contest

import (
	"time"

	"chaimir/internal/platform/ids"
)

// ContestRequest 是创建或编辑竞赛的请求。
type ContestRequest struct {
	Name          string    `json:"name"`
	Mode          int16     `json:"mode"`
	MatchMode     int16     `json:"match_mode"`
	TeamMode      int16     `json:"team_mode"`
	SignupStart   time.Time `json:"signup_start"`
	SignupEnd     time.Time `json:"signup_end"`
	StartAt       time.Time `json:"start_at"`
	EndAt         time.Time `json:"end_at"`
	FreezeMinutes int32     `json:"freeze_minutes"`
}

// ContestDTO 是竞赛定义输出。
type ContestDTO struct {
	ID            ids.ID    `json:"id"`
	OrganizerID   ids.ID    `json:"organizer_id"`
	Name          string    `json:"name"`
	Mode          int16     `json:"mode"`
	MatchMode     int16     `json:"match_mode,omitempty"`
	TeamMode      int16     `json:"team_mode"`
	SignupStart   time.Time `json:"signup_start"`
	SignupEnd     time.Time `json:"signup_end"`
	StartAt       time.Time `json:"start_at"`
	EndAt         time.Time `json:"end_at"`
	FreezeMinutes int32     `json:"freeze_minutes"`
	Status        int16     `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ProblemRequest 是竞赛题目编排请求。
type ProblemRequest struct {
	ItemCode     string               `json:"item_code"`
	ItemVersion  string               `json:"item_version"`
	Score        int32                `json:"score"`
	DynamicScore *DynamicScoreConfig  `json:"dynamic_score"`
	BattleConfig *BattleRuntimeConfig `json:"battle_config"`
	BattleRule   int16                `json:"battle_rule"`
	Seq          int32                `json:"seq"`
}

// ProblemDTO 是竞赛题目引用输出。
type ProblemDTO struct {
	ID           ids.ID               `json:"id"`
	ContestID    ids.ID               `json:"contest_id"`
	Title        string               `json:"title"`
	ItemCode     string               `json:"item_code"`
	ItemVersion  string               `json:"item_version"`
	Score        int32                `json:"score"`
	DynamicScore *DynamicScoreConfig  `json:"dynamic_score,omitempty"`
	BattleConfig *BattleRuntimeConfig `json:"battle_config,omitempty"`
	BattleRule   int16                `json:"battle_rule,omitempty"`
	Seq          int32                `json:"seq"`
	Face         map[string]any       `json:"face,omitempty"`
}

// SignupRequest 是学生报名或创建队伍请求。
type SignupRequest struct {
	TeamName string `json:"team_name"`
}

// JoinTeamRequest 是邀请码加入队伍请求。
type JoinTeamRequest struct {
	InviteCode string `json:"invite_code"`
}

// TeamDTO 是参赛队伍输出。
type TeamDTO struct {
	ID         ids.ID          `json:"id"`
	ContestID  ids.ID          `json:"contest_id"`
	Name       string          `json:"name"`
	InviteCode string          `json:"invite_code,omitempty"`
	Status     int16           `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
	Members    []TeamMemberDTO `json:"members"`
}

// TeamMemberDTO 是队员输出。
type TeamMemberDTO struct {
	ID             ids.ID    `json:"id"`
	TeamID         ids.ID    `json:"team_id"`
	AccountID      ids.ID    `json:"account_id"`
	MemberTenantID ids.ID    `json:"member_tenant_id"`
	IsLeader       bool      `json:"is_leader"`
	JoinedAt       time.Time `json:"joined_at"`
}

// EnvRequest 是实操题环境启动请求。
type EnvRequest struct {
	RuntimeCode         string   `json:"runtime_code"`
	RuntimeImageVersion string   `json:"runtime_image_version"`
	ToolCodes           []string `json:"tool_codes"`
	InitCodeRef         string   `json:"init_code_ref"`
	InitScriptRef       string   `json:"init_script_ref"`
}

// EnvDTO 是 M2 沙箱环境摘要。
type EnvDTO struct {
	SandboxID ids.ID `json:"sandbox_id"`
	SourceRef string `json:"source_ref"`
	Status    int16  `json:"status"`
}

// SubmitRequest 是解题赛提交请求。
type SubmitRequest struct {
	ContentRef     map[string]any `json:"content_ref"`
	CodeStorageKey string         `json:"code_storage_key"`
	CodeHash       string         `json:"code_hash"`
	SandboxRef     string         `json:"sandbox_ref"`
}

// SubmissionDTO 是解题赛提交输出。
type SubmissionDTO struct {
	ID           ids.ID         `json:"id"`
	ContestID    ids.ID         `json:"contest_id"`
	ProblemID    ids.ID         `json:"problem_id"`
	TeamID       ids.ID         `json:"team_id"`
	SubmitterID  ids.ID         `json:"submitter_id"`
	ContentRef   map[string]any `json:"content_ref"`
	SourceRef    string         `json:"source_ref"`
	JudgeTaskRef string         `json:"judge_task_ref,omitempty"`
	Passed       bool           `json:"passed"`
	Score        int32          `json:"score"`
	SandboxRef   string         `json:"sandbox_ref,omitempty"`
	SubmittedAt  time.Time      `json:"submitted_at"`
}

// BattleEntryRequest 是提交参战物请求。
type BattleEntryRequest struct {
	ProblemID   ids.ID `json:"problem_id"`
	Role        int16  `json:"role"`
	ArtifactRef string `json:"artifact_ref"`
	CodeHash    string `json:"code_hash"`
}

// BattleEntryDTO 是参战物输出。
type BattleEntryDTO struct {
	ID          ids.ID    `json:"id"`
	ContestID   ids.ID    `json:"contest_id"`
	ProblemID   ids.ID    `json:"problem_id"`
	TeamID      ids.ID    `json:"team_id"`
	Role        int16     `json:"role"`
	ArtifactRef string    `json:"artifact_ref"`
	CodeHash    string    `json:"code_hash"`
	VersionNo   int32     `json:"version_no"`
	IsActive    bool      `json:"is_active"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// BattleMatchDTO 是对局输出。
type BattleMatchDTO struct {
	ID              ids.ID         `json:"id"`
	ContestID       ids.ID         `json:"contest_id"`
	ProblemID       ids.ID         `json:"problem_id"`
	EntryAID        ids.ID         `json:"entry_a_id"`
	EntryBID        ids.ID         `json:"entry_b_id"`
	SourceRef       string         `json:"source_ref"`
	SandboxRef      string         `json:"sandbox_ref,omitempty"`
	JudgeTaskRef    string         `json:"judge_task_ref,omitempty"`
	Result          int16          `json:"result,omitempty"`
	ScoreDelta      map[string]any `json:"score_delta"`
	ReplayAvailable bool           `json:"replay_available"`
	Status          int16          `json:"status"`
	MatchedAt       time.Time      `json:"matched_at"`
	FinishedAt      time.Time      `json:"finished_at,omitempty"`
}

// BattleReplayDTO 是参赛者可见的真实对局回放时间轴。
type BattleReplayDTO struct {
	MatchID      ids.ID             `json:"match_id"`
	ProblemTitle string             `json:"problem_title"`
	Result       int16              `json:"result"`
	ScoreDelta   map[string]any     `json:"score_delta"`
	Steps        []BattleReplayStep `json:"steps"`
	FinishedAt   time.Time          `json:"finished_at"`
}

// LadderDTO 是排行榜输出。
type LadderDTO struct {
	TeamID      ids.ID    `json:"team_id"`
	TeamName    string    `json:"team_name"`
	Score       float64   `json:"score"`
	SolvedCount int32     `json:"solved_count"`
	LastSolveAt time.Time `json:"last_solve_at,omitempty"`
	Rank        int32     `json:"rank"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ResultSnapshotDTO 是竞赛归档后的最终榜单快照输出。
type ResultSnapshotDTO struct {
	ID           ids.ID      `json:"id"`
	TenantID     ids.ID      `json:"tenant_id,omitempty"`
	ContestID    ids.ID      `json:"contest_id"`
	FinalRanking []LadderDTO `json:"final_ranking"`
	GeneratedAt  time.Time   `json:"generated_at"`
}

// CheatRecordRequest 是违规判定请求。
type CheatRecordRequest struct {
	TeamID   ids.ID         `json:"team_id"`
	Type     int16          `json:"type"`
	Evidence map[string]any `json:"evidence"`
	Action   int16          `json:"action"`
}

// CheatRecordDTO 是违规记录输出。
type CheatRecordDTO struct {
	ID         ids.ID         `json:"id"`
	ContestID  ids.ID         `json:"contest_id"`
	TeamID     ids.ID         `json:"team_id"`
	Type       int16          `json:"type"`
	Evidence   map[string]any `json:"evidence"`
	Action     int16          `json:"action"`
	OperatorID ids.ID         `json:"operator_id,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// CheatSuspectDTO 是查重服务返回的疑似违规线索。
type CheatSuspectDTO struct {
	SourceRef   string  `json:"source_ref"`
	SubmitterID ids.ID  `json:"submitter_id"`
	Score       float64 `json:"score"`
	CodeHash    string  `json:"code_hash,omitempty"`
}

// VulnSourceRequest 是漏洞源配置请求。
type VulnSourceRequest struct {
	ID           ids.ID           `json:"id"`
	Type         int16            `json:"type"`
	Name         string           `json:"name"`
	Config       VulnSourceConfig `json:"config"`
	DefaultLevel int16            `json:"default_level"`
	Enabled      bool             `json:"enabled"`
}

// VulnSourceConfig 定义外部漏洞源的固定连接与映射配置。
type VulnSourceConfig struct {
	Endpoint       string            `json:"endpoint"`
	Method         string            `json:"method"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Body           map[string]any    `json:"body,omitempty"`
	CasesPath      string            `json:"cases_path,omitempty"`
	Mapping        VulnSourceMapping `json:"mapping"`
}

// VulnSourceMapping 定义外部案例字段到漏洞题草稿的固定映射。
type VulnSourceMapping struct {
	ExternalRef string `json:"external_ref"`
	Title       string `json:"title"`
	Level       string `json:"level,omitempty"`
	RuntimeMode string `json:"runtime_mode,omitempty"`
	DraftBody   string `json:"draft_body"`
}

// VulnSourceDTO 是漏洞源输出。
type VulnSourceDTO struct {
	ID           ids.ID           `json:"id"`
	Type         int16            `json:"type"`
	Name         string           `json:"name"`
	Config       VulnSourceConfig `json:"config"`
	DefaultLevel int16            `json:"default_level"`
	Enabled      bool             `json:"enabled"`
	LastSyncAt   time.Time        `json:"last_sync_at,omitempty"`
}

// ImportVulnProblemRequest 是手动导入漏洞案例请求。
type ImportVulnProblemRequest struct {
	SourceID    ids.ID         `json:"source_id"`
	ExternalRef string         `json:"external_ref"`
	Title       string         `json:"title"`
	Level       int16          `json:"level"`
	RuntimeMode int16          `json:"runtime_mode"`
	DraftBody   map[string]any `json:"draft_body"`
}

// PrevalidateRequest 是漏洞题预验证执行请求,实际结果由后端运行正反向验证生成。
type PrevalidateRequest struct {
	RuntimeCode         string   `json:"runtime_code"`
	RuntimeImageVersion string   `json:"runtime_image_version"`
	ToolCodes           []string `json:"tool_codes"`
	InitCodeRef         string   `json:"init_code_ref"`
	InitScriptRef       string   `json:"init_script_ref"`
}

// VulnProblemDTO 是漏洞题草稿输出。
type VulnProblemDTO struct {
	ID                 ids.ID         `json:"id"`
	SourceID           ids.ID         `json:"source_id,omitempty"`
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

// ContestRecordDTO 是学生个人竞赛战绩输出。
type ContestRecordDTO struct {
	ContestID     ids.ID  `json:"contest_id"`
	TeamID        ids.ID  `json:"team_id"`
	Score         float64 `json:"score"`
	Rank          int32   `json:"rank"`
	ContestName   string  `json:"contest_name"`
	ContestStatus int16   `json:"contest_status"`
}
