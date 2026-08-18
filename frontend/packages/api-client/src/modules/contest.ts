// Contest API：对齐后端 M8 竞赛模块唯一 HTTP 契约。

import { ApiClient, encodePathSegment } from '../client'
import type { ContestStatus, VulnProblemStatus } from '../constants/contest'
import type { PaginatedResponse, SnowflakeID } from '../types/common'
import type {
  BattleEntryRequest,
  BattleEntry,
  BattleMatch,
  BattleReplayRef,
  BattleReplayDownloadGrant,
  CheatRecord,
  CheatRecordRequest,
  CheatSuspect,
  Contest,
  ContestProblem,
  ContestProblemRequest,
  ContestRecord,
  ContestRequest,
  ContestSubmission,
  ContestSubmitRequest,
  EnvRequest,
  EnvSummary,
  JoinTeamRequest,
  LadderRank,
  ResultSnapshot,
  SignupRequest,
  ContestTeam,
  VulnProblem,
  VulnProblemImportRequest,
  VulnSource,
  VulnSourceRequest,
  VulnPrevalidateRequest,
} from '../types/contest'

/**
 * ContestApi 封装 M8 竞赛模块的前端 HTTP 契约。
 */
export class ContestApi {
  /**
   * constructor 注入统一 ApiClient,确保竞赛接口共用鉴权、trace_id 和错误处理。
   */
  constructor(private client: ApiClient) {}

  /**
   * 获取竞赛列表。
   */
  async getContests(params?: { status?: ContestStatus; page?: number; size?: number }): Promise<PaginatedResponse<Contest>> {
    return this.client.get('/contest/contests', params)
  }

  /**
   * getStudentContests 查询学生可发现的非草稿竞赛。
   * status 筛选由服务端执行(草稿态仍不可见),total 与筛选同口径。
   */
  async getStudentContests(params?: {
    status?: ContestStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Contest>> {
    return this.client.get('/contest/student/contests', params)
  }

  /**
   * 读取单个学生可发现的竞赛（门槛与列表一致：草稿态不可见）。
   * 竞赛详情页深链、刷新以及退出竞赛答题/对局回放后的回落都用这条。
   */
  async getStudentContest(contestId: string): Promise<Contest> {
    return this.client.get(`/contest/student/contests/${encodePathSegment(contestId)}`)
  }

  /**
   * 创建竞赛。
   */
  async createContest(data: ContestRequest): Promise<Contest> {
    return this.client.post('/contest/contests', data)
  }

  /**
   * 更新草稿竞赛。
   */
  async updateContest(contestId: string, data: ContestRequest): Promise<Contest> {
    return this.client.patch(`/contest/contests/${encodePathSegment(contestId)}`, data)
  }

  /**
   * 发布竞赛。
   */
  async publishContest(contestId: string): Promise<Contest> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/publish`)
  }

  /**
   * 开始竞赛。
   */
  async startContest(contestId: string): Promise<Contest> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/start`)
  }

  /**
   * 进入封榜期。
   */
  async freezeContest(contestId: string): Promise<Contest> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/freeze`)
  }

  /**
   * 结束竞赛。
   */
  async endContest(contestId: string): Promise<Contest> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/end`)
  }

  /**
   * 归档竞赛并生成最终榜单快照。
   */
  async archiveContest(contestId: string): Promise<ResultSnapshot> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/archive`)
  }

  /**
   * 获取竞赛最终榜单快照。
   */
  async getResultSnapshot(contestId: string): Promise<ResultSnapshot> {
    return this.client.get(`/contest/contests/${encodePathSegment(contestId)}/result-snapshot`)
  }

  /**
   * 获取竞赛题面列表。
   */
  async getProblems(contestId: string): Promise<ContestProblem[]> {
    return this.client.get(`/contest/contests/${encodePathSegment(contestId)}/problems`)
  }

  /**
   * 添加或更新竞赛题目。
   */
  async addProblem(contestId: string, data: ContestProblemRequest): Promise<ContestProblem> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/problems`, data)
  }

  /**
   * 学生报名或创建队伍。
   */
  async signup(contestId: string, data: SignupRequest): Promise<ContestTeam> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/signup`, data)
  }

  /**
   * 用队长分享的邀请码加入本赛事队伍。
   * 只按邀请码定位队伍:队伍编号对学生是不可知的内部标识,不要求页面先取到它。
   */
  async joinTeam(contestId: string, data: JoinTeamRequest): Promise<ContestTeam> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/join-team`, data)
  }

  /**
   * 获取队伍信息。
   */
  async getTeam(teamId: string): Promise<ContestTeam> {
    return this.client.get(`/contest/teams/${encodePathSegment(teamId)}`)
  }

  /**
   * 锁定队伍名单。
   */
  async lockTeam(teamId: string): Promise<ContestTeam> {
    return this.client.post(`/contest/teams/${encodePathSegment(teamId)}/lock`)
  }

  /**
   * 创建解题赛实操环境。
   */
  async createEnv(contestId: string, problemId: string, data: EnvRequest): Promise<EnvSummary> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/problems/${encodePathSegment(problemId)}/env`, data)
  }

  /**
   * 提交解题赛答案或代码引用。
   */
  async submitSolve(contestId: string, problemId: string, data: ContestSubmitRequest): Promise<ContestSubmission> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/problems/${encodePathSegment(problemId)}/submit`, data)
  }

  /**
   * 获取提交详情。
   */
  async getSubmission(submissionId: string): Promise<ContestSubmission> {
    return this.client.get(`/contest/submissions/${encodePathSegment(submissionId)}`)
  }

  /**
   * 提交对抗赛参战物。
   */
  async submitBattleEntry(contestId: string, data: BattleEntryRequest): Promise<BattleEntry> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/battle/entry`, data)
  }

  /**
   * 查询当前队伍参战历史。
   */
  async listBattleEntries(contestId: string): Promise<BattleEntry[]> {
    return this.client.get(`/contest/contests/${encodePathSegment(contestId)}/battle/entries`)
  }

  /**
   * 查询当前队伍对局列表。
   */
  async listBattleMatches(contestId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<BattleMatch>> {
    return this.client.get(`/contest/contests/${encodePathSegment(contestId)}/battle/matches`, params)
  }

  /**
   * 获取对局回放引用。
   */
  async getBattleReplay(matchId: string): Promise<BattleReplayRef> {
    return this.client.get(`/contest/matches/${encodePathSegment(matchId)}/replay`)
  }

  /** 为已授权的对局回放签发统一文件服务取件授权。 */
  async issueBattleReplayDownloadGrant(matchId: string): Promise<BattleReplayDownloadGrant> {
    return this.client.post(`/contest/matches/${encodePathSegment(matchId)}/replay/download-grant`)
  }

  /**
   * 获取排行榜。
   */
  async getLadder(contestId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<LadderRank>> {
    return this.client.get(`/contest/contests/${encodePathSegment(contestId)}/ladder`, params)
  }

  /**
   * 订阅排行榜实时更新 topic。
   */
  getLeaderboardTopic(tenantId: string, contestId: string): string {
    return `tenant:${tenantId}:contest:${contestId}:leaderboard`
  }

  /**
   * 查询我的竞赛战绩。
   */
  async getMyContestRecords(): Promise<ContestRecord[]> {
    return this.client.get('/contest/my/contest-records')
  }

  /**
   * 查询防作弊疑似线索。
   */
  async listCheatSuspects(
    contestId: string,
    params: { problem_id: SnowflakeID; code_hash?: string; exclude_source_ref?: string; threshold?: number }
  ): Promise<CheatSuspect[]> {
    return this.client.get(`/contest/contests/${encodePathSegment(contestId)}/cheat-suspects`, params)
  }

  /**
   * 查询违规处理记录。
   */
  async listCheatRecords(contestId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<CheatRecord>> {
    return this.client.get(`/contest/contests/${encodePathSegment(contestId)}/cheat-records`, params)
  }

  /**
   * 创建违规处理记录。
   */
  async createCheatRecord(contestId: string, data: CheatRecordRequest): Promise<CheatRecord> {
    return this.client.post(`/contest/contests/${encodePathSegment(contestId)}/cheat-records`, data)
  }

  /**
   * 查询漏洞源配置。
   */
  async listVulnSources(): Promise<VulnSource[]> {
    return this.client.get('/contest/vuln-sources')
  }

  /** listPlatformVulnSources 查询平台维护的全局漏洞源。 */
  async listPlatformVulnSources(): Promise<VulnSource[]> {
    return this.client.get('/contest/platform/vuln-sources')
  }

  /**
   * 创建或更新漏洞源配置。
   */
  async upsertVulnSource(data: VulnSourceRequest): Promise<VulnSource> {
    return this.client.post('/contest/vuln-sources', data)
  }

  /** upsertPlatformVulnSource 创建或更新平台全局漏洞源。 */
  async upsertPlatformVulnSource(data: VulnSourceRequest): Promise<VulnSource> {
    return this.client.post('/contest/platform/vuln-sources', data)
  }

  /**
   * 同步漏洞源案例。
   */
  async syncVulnSource(sourceId: string): Promise<VulnProblem[]> {
    return this.client.post(`/contest/vuln-sources/${encodePathSegment(sourceId)}/sync`)
  }

  /**
   * 查询漏洞题草稿。
   */
  async listVulnProblems(params?: { source_id?: SnowflakeID; status?: VulnProblemStatus; page?: number; size?: number }): Promise<PaginatedResponse<VulnProblem>> {
    return this.client.get('/contest/vuln-problems', params)
  }

  /**
   * 导入漏洞题草稿。
   * 手工录入与按来源案例录入是同一条路径:请求体的可选 source_id 表达来源归属
   * (对齐清单 §6.17 已收敛掉同义的 /vuln-sources/import)。
   */
  async importVulnProblem(data: VulnProblemImportRequest): Promise<VulnProblem> {
    return this.client.post('/contest/vuln-problems', data)
  }

  /**
   * 执行漏洞题预验证。
   */
  async prevalidateVulnProblem(problemId: string, data: VulnPrevalidateRequest): Promise<VulnProblem> {
    return this.client.post(`/contest/vuln-problems/${encodePathSegment(problemId)}/prevalidate`, data)
  }

  /**
   * 固化漏洞题到题库。
   */
  async finalizeVulnProblem(problemId: string): Promise<VulnProblem> {
    return this.client.post(`/contest/vuln-problems/${encodePathSegment(problemId)}/finalize`)
  }
}
