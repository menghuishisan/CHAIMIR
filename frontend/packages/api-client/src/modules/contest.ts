// Contest API：对齐后端 M8 竞赛模块唯一 HTTP 契约。

import { ApiClient, encodePathSegment } from '../client'
import { API_BASE_PATH } from '../constants'
import type { IdentityApi } from './identity'
import type { ContestStatus, VulnPrevalidateStatus, VulnProblemStatus } from '../constants/contest'
import type { PaginatedResponse, SnowflakeID } from '../types/common'
import type {
  BattleEntryRequest,
  BattleEntry,
  BattleMatch,
  BattleReplayRef,
  BattleReplayDownloadGrant,
  BattleReplayWindow,
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
  ContestSandboxResponse,
  ContestSandboxFileReadResponse,
  ContestSandboxFileListResponse,
  ContestSandboxFileWriteRequest,
  ContestSandboxFileWriteResponse,
  ContestSandboxFileSaveResponse,
  ContestSandboxCommandToolRunRequest,
  ContestSandboxCommandToolRunResponse,
  ContestSandboxChainRequest,
  ContestSandboxChainResponse,
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
  constructor(
    private client: ApiClient,
    private identity: IdentityApi
  ) {}

  /**
   * 获取竞赛列表。
   */
  async getContests(params?: {
    status?: ContestStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Contest>> {
    return this.client.get('/contest/contests', params)
  }

  /**
   * getContest 读取单场赛事的教师视图。
   * 与学生单读的区别是**没有非草稿门槛** —— 草稿态赛事正是教师要编排的那些,
   * 所以教师侧必须走这条,不能借学生接口(它会把草稿判成不存在)。
   * 详情、发布与运营动作都以这条返回的对象为当前版本。
   */
  async getContest(contestId: string): Promise<Contest> {
    return this.client.get(`/contest/contests/${encodePathSegment(contestId)}`)
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
    return this.client.post(
      `/contest/contests/${encodePathSegment(contestId)}/problems/${encodePathSegment(problemId)}/env`,
      data
    )
  }

  /** 读取竞赛授权网关中的沙箱摘要。跨校竞赛工作区只允许走 M8 网关。 */
  async getSandboxInstance(sandboxId: string): Promise<ContestSandboxResponse> {
    return this.client.get(`/contest/sandboxes/${encodePathSegment(sandboxId)}`)
  }

  /** 获取竞赛授权沙箱终端 WebSocket 地址。 */
  getSandboxTerminalWsUrl(sandboxId: string, container?: string): string {
    return this.client.wsURL(
      `/contest/sandboxes/${encodePathSegment(sandboxId)}/terminal`,
      container ? { container } : undefined
    )
  }

  /** 获取竞赛授权沙箱准备进度 WebSocket 地址。 */
  getSandboxProgressWsUrl(sandboxId: string): string {
    return this.client.wsURL(`/contest/sandboxes/${encodePathSegment(sandboxId)}/progress`)
  }

  /** 读取竞赛授权沙箱工作区文件。 */
  async readSandboxFile(sandboxId: string, path: string): Promise<ContestSandboxFileReadResponse> {
    return this.client.get(`/contest/sandboxes/${encodePathSegment(sandboxId)}/files`, { path })
  }

  /** 列出竞赛授权沙箱工作区目录。 */
  async listSandboxFiles(sandboxId: string, path = '.'): Promise<ContestSandboxFileListResponse> {
    return this.client.get(`/contest/sandboxes/${encodePathSegment(sandboxId)}/files`, {
      mode: 'list',
      path,
    })
  }

  /** 写入竞赛授权沙箱工作区文件。 */
  async writeSandboxFile(
    sandboxId: string,
    data: ContestSandboxFileWriteRequest
  ): Promise<ContestSandboxFileWriteResponse> {
    return this.client.put(`/contest/sandboxes/${encodePathSegment(sandboxId)}/files`, data)
  }

  /** 持久化竞赛授权沙箱工作区。 */
  async saveSandboxFiles(sandboxId: string): Promise<ContestSandboxFileSaveResponse> {
    return this.client.post(`/contest/sandboxes/${encodePathSegment(sandboxId)}/files/save`)
  }

  /** 执行竞赛授权沙箱命令工具。 */
  async runSandboxCommandTool(
    sandboxId: string,
    toolCode: string,
    data: ContestSandboxCommandToolRunRequest
  ): Promise<ContestSandboxCommandToolRunResponse> {
    return this.client.post(
      `/contest/sandboxes/${encodePathSegment(sandboxId)}/command-tools/${encodePathSegment(toolCode)}/run`,
      data
    )
  }

  /** 执行竞赛授权沙箱链部署。 */
  async sandboxChainDeploy(
    sandboxId: string,
    data: ContestSandboxChainRequest
  ): Promise<ContestSandboxChainResponse> {
    return this.client.post(`/contest/sandboxes/${encodePathSegment(sandboxId)}/chain/deploy`, data)
  }

  /** 执行竞赛授权沙箱链交易。 */
  async sandboxChainSendTx(
    sandboxId: string,
    data: ContestSandboxChainRequest
  ): Promise<ContestSandboxChainResponse> {
    return this.client.post(`/contest/sandboxes/${encodePathSegment(sandboxId)}/chain/tx`, data)
  }

  /** 查询竞赛授权沙箱链状态。 */
  async sandboxChainQuery(
    sandboxId: string,
    runtimeInstance: string,
    target: string
  ): Promise<ContestSandboxChainResponse> {
    return this.client.get(`/contest/sandboxes/${encodePathSegment(sandboxId)}/chain/query`, { runtime_instance: runtimeInstance, target })
  }

  /** 获取竞赛授权沙箱网页工具代理地址。 */
  async getSandboxToolProxyUrl(
    sandboxId: string,
    toolCode: string,
    proxyPath = '',
    toolOrigin: string
  ): Promise<string> {
    const normalizedPath = normalizeProxyPath(proxyPath)
    const encodedSandbox = encodePathSegment(sandboxId)
    const encodedTool = encodePathSegment(toolCode)
    const pathPrefix = `${API_BASE_PATH}/contest/sandboxes/${encodedSandbox}/tools/${encodedTool}`
    const path = `${pathPrefix}/${normalizedPath}`
    const { ticket } = await this.identity.issueBrowserAccessTicket(pathPrefix)
    return this.client.browserURLAtOrigin(toolOrigin, path, { ticket })
  }

  /**
   * 提交解题赛答案或代码引用。
   */
  async submitSolve(
    contestId: string,
    problemId: string,
    data: ContestSubmitRequest
  ): Promise<ContestSubmission> {
    return this.client.post(
      `/contest/contests/${encodePathSegment(contestId)}/problems/${encodePathSegment(problemId)}/submit`,
      data
    )
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
  async listBattleEntries(
    contestId: string,
    params?: { page?: number; size?: number }
  ): Promise<PaginatedResponse<BattleEntry>> {
    return this.client.get(
      `/contest/contests/${encodePathSegment(contestId)}/battle/entries`,
      params
    )
  }

  /**
   * 查询当前队伍对局列表。
   */
  async listBattleMatches(
    contestId: string,
    params?: { page?: number; size?: number }
  ): Promise<PaginatedResponse<BattleMatch>> {
    return this.client.get(
      `/contest/contests/${encodePathSegment(contestId)}/battle/matches`,
      params
    )
  }

  /** 获取回放专用的已完成对局时间窗和服务端检查点。 */
  async getBattleReplayWindow(
    contestId: string,
    params?: { page?: number; size?: number }
  ): Promise<BattleReplayWindow> {
    return this.client.get(
      `/contest/contests/${encodePathSegment(contestId)}/battle/replay-window`,
      params
    )
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
  async getLadder(
    contestId: string,
    params?: { page?: number; size?: number }
  ): Promise<PaginatedResponse<LadderRank>> {
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
    params: {
      problem_id: SnowflakeID
      code_hash?: string
      exclude_source_ref?: string
      threshold?: number
    }
  ): Promise<CheatSuspect[]> {
    return this.client.get(
      `/contest/contests/${encodePathSegment(contestId)}/cheat-suspects`,
      params
    )
  }

  /**
   * 查询违规处理记录。
   */
  async listCheatRecords(
    contestId: string,
    params?: { page?: number; size?: number }
  ): Promise<PaginatedResponse<CheatRecord>> {
    return this.client.get(
      `/contest/contests/${encodePathSegment(contestId)}/cheat-records`,
      params
    )
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
   * `status` 过滤的是草稿/固化状态,`prevalidate_status` 过滤的是预验证结果 ——
   * 两者是不同的列,出题人最常用的分堆是后者(验过 / 没验过)。
   * 两个参数都是 `0=不限`,由服务端过滤,`total` 与筛选同口径。
   */
  async listVulnProblems(params?: {
    source_id?: SnowflakeID
    status?: VulnProblemStatus
    prevalidate_status?: VulnPrevalidateStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<VulnProblem>> {
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
  async prevalidateVulnProblem(
    problemId: string,
    data: VulnPrevalidateRequest
  ): Promise<VulnProblem> {
    return this.client.post(
      `/contest/vuln-problems/${encodePathSegment(problemId)}/prevalidate`,
      data
    )
  }

  /**
   * 固化漏洞题到题库。
   */
  async finalizeVulnProblem(problemId: string): Promise<VulnProblem> {
    return this.client.post(`/contest/vuln-problems/${encodePathSegment(problemId)}/finalize`)
  }
}

/** 将浏览器代理路径约束为不含查询、片段或路径逃逸的编码路径。 */
function normalizeProxyPath(proxyPath: string): string {
  const normalized = proxyPath.trim().replace(/^\/+/, '')
  if (/[?#\\]/.test(normalized) || hasControlCharacter(normalized)) {
    throw new Error('工具代理路径包含不允许的字符')
  }
  const segments = normalized
    .split('/')
    .filter(Boolean)
    .map((segment) => {
      let decoded: string
      try {
        decoded = decodeURIComponent(segment)
      } catch {
        throw new Error('工具代理路径包含无效编码')
      }
      if (
        decoded === '.' ||
        decoded === '..' ||
        /[\\/?#]/.test(decoded) ||
        hasControlCharacter(decoded)
      ) {
        throw new Error('工具代理路径包含不允许的路径段')
      }
      return encodeURIComponent(decoded)
    })
  return segments.join('/')
}

function hasControlCharacter(value: string): boolean {
  return [...value].some((char) => char.charCodeAt(0) < 0x20 || char.charCodeAt(0) === 0x7f)
}
