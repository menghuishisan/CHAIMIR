// contest enum 文件定义 M8 竞赛、队伍、提交、对局、漏洞源和内容固化枚举。
package contest

const (
	// ContestModeSolve 表示解题赛。
	ContestModeSolve int16 = 1
	// ContestModeBattle 表示对抗赛。
	ContestModeBattle int16 = 2
)

var (
	// contestModeRegistry 注册平台内置和后续扩展的竞赛类型。
	contestModeRegistry = map[int16]string{ContestModeSolve: "solve", ContestModeBattle: "battle"}
	// battleRuleRegistry 注册平台内置和后续扩展的对局规则。
	battleRuleRegistry = map[int16]string{BattleRuleAttackDefense: "attack-defense", BattleRuleGame: "game"}
)

// registeredContestMode 判断竞赛类型是否已注册。
func registeredContestMode(mode int16) bool {
	_, ok := contestModeRegistry[mode]
	return ok
}

// registeredBattleRule 判断对局规则是否已注册。
func registeredBattleRule(rule int16) bool {
	_, ok := battleRuleRegistry[rule]
	return ok
}

const (
	// MatchModeRoundRobin 表示循环赛撮合。
	MatchModeRoundRobin int16 = 1
	// MatchModeELO 表示天梯 ELO 撮合。
	MatchModeELO int16 = 2
)

const (
	// TeamModeSolo 表示个人赛,实现上使用单人队。
	TeamModeSolo int16 = 1
	// TeamModeGroup 表示团队赛。
	TeamModeGroup int16 = 2
)

const (
	// ContestStatusDraft 表示竞赛草稿。
	ContestStatusDraft int16 = 1
	// ContestStatusSignup 表示报名中。
	ContestStatusSignup int16 = 2
	// ContestStatusRunning 表示竞赛进行中。
	ContestStatusRunning int16 = 3
	// ContestStatusFrozen 表示封榜期。
	ContestStatusFrozen int16 = 4
	// ContestStatusEnded 表示竞赛已结束。
	ContestStatusEnded int16 = 5
	// ContestStatusArchived 表示竞赛已归档。
	ContestStatusArchived int16 = 6
)

const (
	// TeamStatusBuilding 表示队伍组建中。
	TeamStatusBuilding int16 = 1
	// TeamStatusLocked 表示队伍已锁定。
	TeamStatusLocked int16 = 2
)

const (
	// BattleRuleAttackDefense 表示攻防型对局规则。
	BattleRuleAttackDefense int16 = 1
	// BattleRuleGame 表示博弈型对局规则。
	BattleRuleGame int16 = 2
)

const (
	// BattleRoleStrategy 表示博弈策略方。
	BattleRoleStrategy int16 = 0
	// BattleRoleDefense 表示攻防守方。
	BattleRoleDefense int16 = 1
	// BattleRoleAttack 表示攻防攻方。
	BattleRoleAttack int16 = 2
)

const (
	// BattleMatchStatusPending 表示对局待执行。
	BattleMatchStatusPending int16 = 1
	// BattleMatchStatusRunning 表示对局执行中。
	BattleMatchStatusRunning int16 = 2
	// BattleMatchStatusDone 表示对局完成。
	BattleMatchStatusDone int16 = 3
	// BattleMatchStatusFailed 表示对局失败。
	BattleMatchStatusFailed int16 = 4
)

const (
	// BattleResultAWin 表示 A 方获胜。
	BattleResultAWin int16 = 1
	// BattleResultBWin 表示 B 方获胜。
	BattleResultBWin int16 = 2
	// BattleResultDraw 表示平局。
	BattleResultDraw int16 = 3
)

const (
	// CheatTypeSimilarity 表示代码查重类违规。
	CheatTypeSimilarity int16 = 1
	// CheatTypeBehavior 表示行为异常类违规。
	CheatTypeBehavior int16 = 2
	// CheatTypeEnvironment 表示环境违规。
	CheatTypeEnvironment int16 = 3
)

const (
	// CheatActionWarn 表示警告。
	CheatActionWarn int16 = 1
	// CheatActionPenalty 表示扣分。
	CheatActionPenalty int16 = 2
	// CheatActionDisqualify 表示取消资格。
	CheatActionDisqualify int16 = 3
)

const (
	// VulnLevelA 表示可自动转链上验证题。
	VulnLevelA int16 = 1
	// VulnLevelB 表示待人工补全草稿。
	VulnLevelB int16 = 2
	// VulnLevelC 表示静态或理论素材。
	VulnLevelC int16 = 3
)

const (
	// VulnRuntimeIsolated 表示平台隔离链复现。
	VulnRuntimeIsolated int16 = 1
	// VulnRuntimeForked 表示历史区块 fork 复现。
	VulnRuntimeForked int16 = 2
)

const (
	// VulnPrevalidatePending 表示尚未预验证。
	VulnPrevalidatePending int16 = 1
	// VulnPrevalidatePassed 表示正反向预验证通过。
	VulnPrevalidatePassed int16 = 2
	// VulnPrevalidateFailed 表示预验证失败。
	VulnPrevalidateFailed int16 = 3
)

const (
	// VulnProblemStatusDraft 表示漏洞题草稿。
	VulnProblemStatusDraft int16 = 1
	// VulnProblemStatusFinalized 表示已固化入 M5。
	VulnProblemStatusFinalized int16 = 2
	// VulnProblemStatusDiscarded 表示已废弃。
	VulnProblemStatusDiscarded int16 = 3
)

const (
	// contentTypeContestProblem 是 M5 竞赛题内容类型的稳定取值。
	contentTypeContestProblem int16 = 2
	// contentAuthorExternal 是 M5 外部来源导入作者类型的稳定取值。
	contentAuthorExternal int16 = 3
	// contentVisibilityTenant 是 M5 本租户教师可见的稳定取值。
	contentVisibilityTenant int16 = 2
	// contentDifficultyBasic 是 M5 基础难度的稳定取值。
	contentDifficultyBasic int16 = 2
)
