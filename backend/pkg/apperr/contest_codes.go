// apperr contest_codes 文件定义 M8 竞赛模块的稳定错误码和用户向文案。
package apperr

const (
	// CodeContestNotFound 表示竞赛不存在或已移除。
	CodeContestNotFound = "81001"
	// CodeContestInvalid 表示竞赛请求或赛程配置非法。
	CodeContestInvalid = "81002"
	// CodeContestStateInvalid 表示竞赛状态不允许当前操作。
	CodeContestStateInvalid = "81003"
	// CodeContestProblemInvalid 表示竞赛题目编排或引用非法。
	CodeContestProblemInvalid = "81004"
	// CodeContestContentUnavailable 表示题面或内容固化服务暂不可用。
	CodeContestContentUnavailable = "81005"
	// CodeContestEventSubscribeFailed 表示竞赛事件订阅失败。
	CodeContestEventSubscribeFailed = "81006"
	// CodeContestEventPayloadInvalid 表示竞赛事件载荷无法识别。
	CodeContestEventPayloadInvalid = "81007"
	// CodeContestEventSourceMismatch 表示判题事件来源与竞赛提交不匹配。
	CodeContestEventSourceMismatch = "81008"
	// CodeContestNotifyFailed 表示排行榜或参赛通知推送失败。
	CodeContestNotifyFailed = "81009"
)

const (
	// CodeContestTeamNotFound 表示队伍不存在。
	CodeContestTeamNotFound = "82001"
	// CodeContestTeamInvalid 表示报名或队伍请求非法。
	CodeContestTeamInvalid = "82002"
	// CodeContestTeamAccessDenied 表示当前账号无权访问该队伍。
	CodeContestTeamAccessDenied = "82003"
	// CodeContestSignupClosed 表示当前不在报名期。
	CodeContestSignupClosed = "82004"
)

const (
	// CodeContestSubmissionNotFound 表示解题提交不存在。
	CodeContestSubmissionNotFound = "83001"
	// CodeContestSubmissionInvalid 表示解题提交内容非法。
	CodeContestSubmissionInvalid = "83002"
	// CodeContestJudgeUnavailable 表示评测服务暂不可用。
	CodeContestJudgeUnavailable = "83003"
	// CodeContestSandboxUnavailable 表示竞赛环境暂不可用。
	CodeContestSandboxUnavailable = "83004"
	// CodeContestSubmitRateLimited 表示提交过于频繁或进入冷却。
	CodeContestSubmitRateLimited = "83005"
)

const (
	// CodeContestBattleEntryInvalid 表示对抗参战物请求非法。
	CodeContestBattleEntryInvalid = "84001"
	// CodeContestBattleMatchNotFound 表示对局不存在。
	CodeContestBattleMatchNotFound = "84002"
	// CodeContestBattleMatchFailed 表示对局执行或结算失败。
	CodeContestBattleMatchFailed = "84003"
	// CodeContestReplayUnavailable 表示对局回放不可用。
	CodeContestReplayUnavailable = "84004"
)

const (
	// CodeContestCheatInvalid 表示防作弊记录或证据非法。
	CodeContestCheatInvalid = "85001"
	// CodeContestVulnSourceInvalid 表示漏洞源配置非法。
	CodeContestVulnSourceInvalid = "85002"
	// CodeContestVulnSourceFetchFailed 表示漏洞源同步失败。
	CodeContestVulnSourceFetchFailed = "85003"
	// CodeContestVulnProblemInvalid 表示漏洞题草稿非法。
	CodeContestVulnProblemInvalid = "85004"
	// CodeContestVulnPrevalidateFailed 表示漏洞题预验证失败。
	CodeContestVulnPrevalidateFailed = "85005"
	// CodeContestVulnFinalizeFailed 表示漏洞题固化失败。
	CodeContestVulnFinalizeFailed = "85006"
)

var (
	// ErrContestNotFound 表示竞赛不存在或已移除。
	ErrContestNotFound = New(CodeContestNotFound, "竞赛不存在或已移除")
	// ErrContestInvalid 表示竞赛信息不完整。
	ErrContestInvalid = New(CodeContestInvalid, "竞赛信息不完整,请检查后重试")
	// ErrContestStateInvalid 表示当前竞赛状态不支持操作。
	ErrContestStateInvalid = New(CodeContestStateInvalid, "当前竞赛状态不支持该操作")
	// ErrContestProblemInvalid 表示竞赛题目信息不正确。
	ErrContestProblemInvalid = New(CodeContestProblemInvalid, "竞赛题目信息不正确")
	// ErrContestContentUnavailable 表示题面或内容服务不可用。
	ErrContestContentUnavailable = New(CodeContestContentUnavailable, "竞赛题目暂时无法读取")
	// ErrContestEventSubscribeFailed 表示事件订阅失败。
	ErrContestEventSubscribeFailed = New(CodeContestEventSubscribeFailed, "竞赛事件暂时无法订阅")
	// ErrContestEventPayloadInvalid 表示事件载荷非法。
	ErrContestEventPayloadInvalid = New(CodeContestEventPayloadInvalid, "竞赛事件内容不正确")
	// ErrContestEventSourceMismatch 表示事件来源不匹配。
	ErrContestEventSourceMismatch = New(CodeContestEventSourceMismatch, "竞赛提交来源校验失败")
	// ErrContestNotifyFailed 表示通知推送失败。
	ErrContestNotifyFailed = New(CodeContestNotifyFailed, "竞赛动态暂时无法推送")
	// ErrContestTeamNotFound 表示队伍不存在。
	ErrContestTeamNotFound = New(CodeContestTeamNotFound, "参赛队伍不存在")
	// ErrContestTeamInvalid 表示队伍信息非法。
	ErrContestTeamInvalid = New(CodeContestTeamInvalid, "参赛队伍信息不完整,请检查后重试")
	// ErrContestTeamAccessDenied 表示队伍访问被拒绝。
	ErrContestTeamAccessDenied = New(CodeContestTeamAccessDenied, "无法访问该参赛队伍")
	// ErrContestSignupClosed 表示报名未开放。
	ErrContestSignupClosed = New(CodeContestSignupClosed, "当前不在竞赛报名时间内")
	// ErrContestSubmissionNotFound 表示提交不存在。
	ErrContestSubmissionNotFound = New(CodeContestSubmissionNotFound, "竞赛提交不存在")
	// ErrContestSubmissionInvalid 表示提交内容非法。
	ErrContestSubmissionInvalid = New(CodeContestSubmissionInvalid, "提交内容不完整,请检查后重试")
	// ErrContestJudgeUnavailable 表示判题服务不可用。
	ErrContestJudgeUnavailable = New(CodeContestJudgeUnavailable, "提交暂时无法判定,请稍后重试")
	// ErrContestSandboxUnavailable 表示竞赛环境不可用。
	ErrContestSandboxUnavailable = New(CodeContestSandboxUnavailable, "竞赛环境暂时无法准备,请稍后重试")
	// ErrContestSubmitRateLimited 表示提交限频或冷却。
	ErrContestSubmitRateLimited = New(CodeContestSubmitRateLimited, "提交过于频繁,请稍后再试")
	// ErrContestBattleEntryInvalid 表示参战物非法。
	ErrContestBattleEntryInvalid = New(CodeContestBattleEntryInvalid, "参战物信息不正确")
	// ErrContestBattleMatchNotFound 表示对局不存在。
	ErrContestBattleMatchNotFound = New(CodeContestBattleMatchNotFound, "竞赛对局不存在")
	// ErrContestBattleMatchFailed 表示对局失败。
	ErrContestBattleMatchFailed = New(CodeContestBattleMatchFailed, "竞赛对局暂时无法完成")
	// ErrContestReplayUnavailable 表示回放不可用。
	ErrContestReplayUnavailable = New(CodeContestReplayUnavailable, "对局回放暂时无法查看")
	// ErrContestCheatInvalid 表示防作弊记录非法。
	ErrContestCheatInvalid = New(CodeContestCheatInvalid, "违规处理信息不完整")
	// ErrContestVulnSourceInvalid 表示漏洞源配置非法。
	ErrContestVulnSourceInvalid = New(CodeContestVulnSourceInvalid, "漏洞源配置不正确")
	// ErrContestVulnSourceFetchFailed 表示漏洞源同步失败。
	ErrContestVulnSourceFetchFailed = New(CodeContestVulnSourceFetchFailed, "漏洞源暂时无法同步")
	// ErrContestVulnProblemInvalid 表示漏洞题草稿非法。
	ErrContestVulnProblemInvalid = New(CodeContestVulnProblemInvalid, "漏洞题草稿信息不完整")
	// ErrContestVulnPrevalidateFailed 表示预验证失败。
	ErrContestVulnPrevalidateFailed = New(CodeContestVulnPrevalidateFailed, "漏洞题预验证未通过")
	// ErrContestVulnFinalizeFailed 表示固化失败。
	ErrContestVulnFinalizeFailed = New(CodeContestVulnFinalizeFailed, "漏洞题暂时无法固化入库")
)
