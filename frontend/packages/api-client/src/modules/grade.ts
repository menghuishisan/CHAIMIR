// Grade API：成绩中心。
// 对应后端 M11 模块的当前接口。

import { ApiClient } from '../client'
import type {
  GradeAppeal,
  GradeAppealRequest,
  GradeReviewRequest,
  GradeReview,
  GradeSummary,
  GradeTranscript,
  GradeWarning,
  LevelConfig,
  LevelConfigRequest,
  PaginatedResponse,
  ReviewDecision,
  Semester,
  SemesterRequest,
  TranscriptDownloadGrant,
  TranscriptRequest,
  WarningRules,
  WarningScanResult,
} from '../types'

/**
 * GradeApi 封装 M11 成绩中心的前端 HTTP 契约。
 */
export class GradeApi {
  /**
   * constructor 注入统一 ApiClient,确保成绩接口共用鉴权、trace_id 和错误处理。
   */
  constructor(private client: ApiClient) {}

  /**
   * 查询当前租户的等级映射配置。
   */
  async listLevelConfigs(): Promise<LevelConfig[]> {
    return this.client.get('/grade-center/level-configs')
  }

  /**
   * 创建等级映射配置。
   */
  async createLevelConfig(data: LevelConfigRequest): Promise<LevelConfig> {
    return this.client.post('/grade-center/level-configs', data)
  }

  /**
   * 更新指定等级映射配置。
   */
  async updateLevelConfig(id: string, data: LevelConfigRequest): Promise<LevelConfig> {
    return this.client.put(`/grade-center/level-configs/${id}`, data)
  }

  /**
   * 查询当前租户的学期列表。
   */
  async listSemesters(): Promise<Semester[]> {
    return this.client.get('/grade-center/semesters')
  }

  /**
   * 创建学期配置。
   */
  async createSemester(data: SemesterRequest): Promise<Semester> {
    return this.client.post('/grade-center/semesters', data)
  }

  /**
   * 提交课程成绩审核。
   */
  async submitReview(data: GradeReviewRequest): Promise<GradeReview> {
    return this.client.post('/grade-center/reviews', data)
  }

  /**
   * 查询成绩审核列表。
   */
  async listReviews(params?: { status?: number; page?: number; size?: number }): Promise<PaginatedResponse<GradeReview>> {
    return this.client.get('/grade-center/reviews', params)
  }

  /**
   * 学校管理员通过成绩审核。
   */
  async approveReview(id: string, data: ReviewDecision): Promise<GradeReview> {
    return this.client.post(`/grade-center/reviews/${id}/approve`, data)
  }

  /**
   * 学校管理员驳回成绩审核。
   */
  async rejectReview(id: string, data: ReviewDecision): Promise<GradeReview> {
    return this.client.post(`/grade-center/reviews/${id}/reject`, data)
  }

  /**
   * 学校管理员解锁已通过的成绩审核。
   */
  async unlockReview(id: string, data: ReviewDecision): Promise<GradeReview> {
    return this.client.post(`/grade-center/reviews/${id}/unlock`, data)
  }

  /**
   * 查询学生课程成绩明细与 GPA 汇总。
   */
  async studentGrades(studentId: string, semester?: string): Promise<GradeSummary> {
    return this.client.get(`/grade-center/students/${studentId}/grades`, semester ? { semester } : undefined)
  }

  /**
   * 查询学生已落库的学期 GPA。
   */
  async studentGPA(studentId: string): Promise<GradeSummary[]> {
    return this.client.get(`/grade-center/students/${studentId}/gpa`)
  }

  /**
   * 管理员触发指定学生的 GPA 重算。
   */
  async recomputeStudentGrade(studentId: string, data: { semester_id: string }): Promise<GradeSummary> {
    return this.client.post(`/grade-center/students/${studentId}/recompute`, data)
  }

  /**
   * 学生提交成绩申诉。
   */
  async submitAppeal(data: GradeAppealRequest): Promise<GradeAppeal> {
    return this.client.post('/grade-center/appeals', data)
  }

  /**
   * 教师或管理员查询成绩申诉列表。
   */
  async listAppeals(params?: { status?: number; page?: number; size?: number }): Promise<PaginatedResponse<GradeAppeal>> {
    return this.client.get('/grade-center/appeals', params)
  }

  /**
   * 教师或管理员受理成绩申诉。
   */
  async acceptAppeal(id: string, data: ReviewDecision): Promise<GradeAppeal> {
    return this.client.post(`/grade-center/appeals/${id}/accept`, data)
  }

  /**
   * 教师或管理员驳回成绩申诉。
   */
  async rejectAppeal(id: string, data: ReviewDecision): Promise<GradeAppeal> {
    return this.client.post(`/grade-center/appeals/${id}/reject`, data)
  }

  /**
   * 查询学业预警规则。
   */
  async getWarningRules(): Promise<WarningRules> {
    return this.client.get('/grade-center/warning-rules')
  }

  /**
   * 更新学业预警规则。
   */
  async updateWarningRules(data: WarningRules): Promise<WarningRules> {
    return this.client.put('/grade-center/warning-rules', data)
  }

  /**
   * 查询当前用户可见的学业预警。
   */
  async listWarnings(params?: { student_id?: string; page?: number; size?: number }): Promise<PaginatedResponse<GradeWarning>> {
    return this.client.get('/grade-center/warnings', params)
  }

  /**
   * 学生确认本人学业预警。
   */
  async ackWarning(id: string): Promise<GradeWarning> {
    return this.client.post(`/grade-center/warnings/${id}/ack`)
  }

  /**
   * 管理员触发学业预警扫描。
   */
  async scanWarnings(data: { student_id?: string; semester_id?: string }): Promise<WarningScanResult> {
    return this.client.post('/grade-center/warnings/scan', data)
  }

  /**
   * 生成单人成绩单记录。
   */
  async generateTranscript(data: TranscriptRequest): Promise<GradeTranscript> {
    return this.client.post('/grade-center/transcripts', data)
  }

  /**
   * 批量生成成绩单记录。
   */
  async generateTranscriptBatch(data: { student_ids: number[]; scope: number; semester_id?: string }): Promise<GradeTranscript[]> {
    return this.client.post('/grade-center/transcripts/batch', data)
  }

  /**
   * 下载成绩单文件。
   */
  async downloadTranscript(id: string): Promise<TranscriptDownloadGrant> {
    return this.client.get(`/grade-center/transcripts/${id}`)
  }
}
