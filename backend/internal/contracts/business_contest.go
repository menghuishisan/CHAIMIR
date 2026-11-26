// contracts 定义竞赛模块对聚合层开放的统计、成就与订阅授权契约。
package contracts

import "context"

// ContestStats 是 M8 输出给 M9 的竞赛统计摘要。
type ContestStats struct {
	TenantID           int64 `json:"tenant_id"`
	ContestCount       int64 `json:"contest_count"`
	ActiveContestCount int64 `json:"active_contest_count"`
	ParticipantCount   int64 `json:"participant_count"`
}

// ContestAchievement 是 M8 输出给 M11 展示的竞赛成就摘要。
type ContestAchievement struct {
	TenantID  int64   `json:"tenant_id"`
	StudentID int64   `json:"student_id"`
	ContestID int64   `json:"contest_id"`
	TeamID    int64   `json:"team_id"`
	Score     float64 `json:"score"`
	Rank      int32   `json:"rank"`
}

// ContestReadService 是 M8 对聚合层开放的唯一只读竞赛契约。
// 聚合模块不得直接访问 M8 表。
type ContestReadService interface {
	// Stats 读取租户级竞赛统计,供 M9 学校看板聚合。
	Stats(ctx context.Context, tenantID int64) (ContestStats, error)
	// ListStudentAchievements 读取学生竞赛成就,供 M11 与 GPA 分离展示。
	ListStudentAchievements(ctx context.Context, tenantID, studentID int64) ([]ContestAchievement, error)
	// AuthorizeLeaderboardSubscription 校验上下文账号的竞赛授权及指定主办方赛事的当前队员资格。
	AuthorizeLeaderboardSubscription(ctx context.Context, organizerTenantID, contestID int64) error
}
