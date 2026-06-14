// Contest API：竞赛管理
// 对应后端 M8 模块

import { ApiClient } from '../client'
import type {
  Contest,
  ContestRequest,
  ContestProblem,
  ContestProblemRequest,
  ContestTeam,
  SignupRequest,
  JoinTeamRequest,
  ContestSubmission,
  ContestSubmitRequest,
  Leaderboard,
  BattleReplay,
  PaginatedResponse,
} from '../types'

export class ContestApi {
  constructor(private client: ApiClient) {}

  // ===== 竞赛管理 =====

  /**
   * 获取竞赛列表
   */
  async getContests(params?: {
    mode?: number
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Contest>> {
    return this.client.get('/contest/contests', params)
  }

  /**
   * 获取竞赛详情
   */
  async getContest(contestId: string): Promise<Contest> {
    return this.client.get(`/contest/contests/${contestId}`)
  }

  /**
   * 创建竞赛
   */
  async createContest(data: ContestRequest): Promise<Contest> {
    return this.client.post('/contest/contests', data)
  }

  /**
   * 更新竞赛
   */
  async updateContest(contestId: string, data: ContestRequest): Promise<Contest> {
    return this.client.put(`/contest/contests/${contestId}`, data)
  }

  /**
   * 删除竞赛
   */
  async deleteContest(contestId: string): Promise<void> {
    return this.client.delete(`/contest/contests/${contestId}`)
  }

  /**
   * 发布竞赛
   */
  async publishContest(contestId: string): Promise<void> {
    return this.client.post(`/contest/contests/${contestId}/publish`)
  }

  // ===== 题目编排 =====

  /**
   * 获取竞赛题目列表
   */
  async getProblems(contestId: string): Promise<ContestProblem[]> {
    return this.client.get(`/contest/contests/${contestId}/problems`)
  }

  /**
   * 添加题目到竞赛
   */
  async addProblem(contestId: string, data: ContestProblemRequest): Promise<ContestProblem> {
    return this.client.post(`/contest/contests/${contestId}/problems`, data)
  }

  /**
   * 更新竞赛题目
   */
  async updateProblem(problemId: string, data: ContestProblemRequest): Promise<ContestProblem> {
    return this.client.put(`/contest/problems/${problemId}`, data)
  }

  /**
   * 删除竞赛题目
   */
  async deleteProblem(problemId: string): Promise<void> {
    return this.client.delete(`/contest/problems/${problemId}`)
  }

  // ===== 报名与组队 =====

  /**
   * 学生报名（个人或创建队伍）
   */
  async signup(contestId: string, data: SignupRequest): Promise<ContestTeam> {
    return this.client.post(`/contest/contests/${contestId}/signup`, data)
  }

  /**
   * 加入队伍
   */
  async joinTeam(data: JoinTeamRequest): Promise<void> {
    return this.client.post('/contest/teams/join', data)
  }

  /**
   * 获取我的队伍
   */
  async getMyTeam(contestId: string): Promise<ContestTeam> {
    return this.client.get(`/contest/contests/${contestId}/my-team`)
  }

  /**
   * 退出队伍
   */
  async leaveTeam(teamId: string): Promise<void> {
    return this.client.post(`/contest/teams/${teamId}/leave`)
  }

  // ===== 答题与提交 =====

  /**
   * 提交答案（解题赛）
   */
  async submitAnswer(
    contestId: string,
    problemId: string,
    data: ContestSubmitRequest
  ): Promise<ContestSubmission> {
    return this.client.post(`/contest/contests/${contestId}/problems/${problemId}/submit`, data)
  }

  /**
   * 获取提交列表
   */
  async getSubmissions(contestId: string, params?: {
    problem_id?: string
    team_id?: string
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<ContestSubmission>> {
    return this.client.get(`/contest/contests/${contestId}/submissions`, params)
  }

  /**
   * 获取提交详情
   */
  async getSubmission(submissionId: string): Promise<ContestSubmission> {
    return this.client.get(`/contest/submissions/${submissionId}`)
  }

  // ===== 排行榜 =====

  /**
   * 获取排行榜
   */
  async getLeaderboard(contestId: string): Promise<Leaderboard> {
    return this.client.get(`/contest/contests/${contestId}/leaderboard`)
  }

  /**
   * 订阅排行榜实时更新 topic
   */
  getLeaderboardTopic(contestId: string): string {
    return `contest:${contestId}:leaderboard`
  }

  // ===== 对抗赛 =====

  /**
   * 启动对抗环境
   */
  async startBattle(contestId: string, problemId: string): Promise<{ battle_id: string }> {
    return this.client.post(`/contest/contests/${contestId}/problems/${problemId}/battle/start`)
  }

  /**
   * 获取对抗回放
   */
  async getBattleReplay(battleId: string): Promise<BattleReplay> {
    return this.client.get(`/contest/battles/${battleId}/replay`)
  }

  /**
   * 订阅对抗状态 topic
   */
  getBattleTopic(battleId: string): string {
    return `contest:battle:${battleId}`
  }
}
