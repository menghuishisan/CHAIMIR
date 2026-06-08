// M8 竞赛模块错误码(81xxx 赛事 / 82xxx 报名 / 83xxx 解题 / 84xxx 对抗 / 85xxx 漏洞源)。
// 文案面向用户,内部技术原因通过 WithCause 进入日志。
package apperr

var (
	ErrContestNotFound          = New("81001", "竞赛不存在或已被移除")
	ErrContestInvalid           = New("81002", "竞赛信息不完整,请检查后重试")
	ErrContestState             = New("81003", "竞赛当前状态不支持该操作")
	ErrContestForbidden         = New("81004", "你不能访问或管理该竞赛")
	ErrContestProblem           = New("81005", "竞赛题目信息不正确")
	ErrContestStatsQueryInvalid = New("81006", "竞赛统计查询条件不正确,请检查后重试")
	ErrContestQueryFailed       = New("81007", "竞赛数据暂时无法读取,请稍后重试")
	ErrContestAuditFailed       = New("81008", "竞赛操作暂时无法记录,请稍后重试")

	ErrContestSignupClosed = New("82001", "当前不在报名时间内")
	ErrContestTeamNotFound = New("82002", "队伍不存在或已被移除")
	ErrContestTeamInvalid  = New("82003", "队伍信息不正确")
	ErrContestTeamLocked   = New("82004", "队伍已锁定,不能再变更成员")

	ErrContestSubmissionNotFound = New("83001", "提交记录不存在或已被移除")
	ErrContestSubmissionInvalid  = New("83002", "提交内容不正确")
	ErrContestJudgeFailed        = New("83003", "提交判定失败,请稍后重试")
	ErrContestEventFailed        = New("83004", "竞赛结果暂时无法同步,请稍后重试")
	ErrContestEventInvalid       = New("83005", "竞赛事件数据不正确,请检查后重试")

	ErrContestBattleEntryNotFound = New("84001", "参战物不存在或已被移除")
	ErrContestBattleInvalid       = New("84002", "对抗赛信息不正确")
	ErrContestBattleFailed        = New("84003", "对局结算失败,请稍后重试")

	ErrContestVulnSourceInvalid  = New("85001", "漏洞源配置不正确")
	ErrContestVulnProblemInvalid = New("85002", "漏洞题草稿不正确")
	ErrContestVulnPrevalidate    = New("85003", "漏洞题预验证未通过")
	ErrContestVulnFinalize       = New("85004", "漏洞题固化失败,请稍后重试")
	ErrContestVulnSourceTooLarge = New("85005", "漏洞源响应过大,请调整漏洞源或联系管理员")
)
