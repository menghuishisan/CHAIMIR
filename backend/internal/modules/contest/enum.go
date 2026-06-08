// M8 枚举常量:集中定义竞赛、队伍、提交、对抗与漏洞题状态。
package contest

const (
	ContestModeSolve  int16 = 1
	ContestModeBattle int16 = 2

	MatchModeRoundRobin int16 = 1
	MatchModeElo        int16 = 2

	TeamModeSolo int16 = 1
	TeamModeTeam int16 = 2

	ContestStatusDraft    int16 = 1
	ContestStatusSignup   int16 = 2
	ContestStatusRunning  int16 = 3
	ContestStatusFrozen   int16 = 4
	ContestStatusEnded    int16 = 5
	ContestStatusArchived int16 = 6

	TeamStatusBuilding int16 = 1
	TeamStatusLocked   int16 = 2

	BattleRoleStrategy int16 = 0
	BattleRoleDefense  int16 = 1
	BattleRoleAttack   int16 = 2

	BattleResultAWin int16 = 1
	BattleResultBWin int16 = 2
	BattleResultDraw int16 = 3

	CheatTypeSimilarity int16 = 1
	CheatTypeBehavior   int16 = 2
	CheatTypeEnvAbuse   int16 = 3

	CheatActionWarn       int16 = 1
	CheatActionDeduct     int16 = 2
	CheatActionDisqualify int16 = 3

	VulnLevelA int16 = 1
	VulnLevelB int16 = 2
	VulnLevelC int16 = 3

	VulnRuntimeIsolated int16 = 1
	VulnRuntimeForked   int16 = 2

	VulnPrevalidatePending int16 = 1
	VulnPrevalidatePassed  int16 = 2
	VulnPrevalidateFailed  int16 = 3

	VulnProblemDraft     int16 = 1
	VulnProblemFinalized int16 = 2
	VulnProblemDiscarded int16 = 3
)
