import { ApiClient, encodePathSegment } from '../client'
import type { BoolFilter } from '../constants/filters'
import type {
  GradeAppealStatus,
  GradeReviewStatus,
  GradeWarningStatus,
  TranscriptScope,
} from '../constants/grade'
import type { PaginatedResponse, SnowflakeID } from '../types/common'
import type {
  GradeAppeal,
  GradeAppealRequest,
  GradeReviewRequest,
  GradeReview,
  GradeSummary,
  StudentGradePage,
  GradeTranscript,
  GradeWarning,
  LevelConfig,
  LevelConfigRequest,
  ReviewDecision,
  Semester,
  SemesterRequest,
  TranscriptDownloadGrant,
  TranscriptRequest,
  WarningRules,
  WarningScanResult,
} from '../types/grade'

export class GradeApi {
  constructor(private client: ApiClient) {}

  async listLevelConfigs(): Promise<LevelConfig[]> {
    return this.client.get('/grade-center/level-configs')
  }

  async createLevelConfig(data: LevelConfigRequest): Promise<LevelConfig> {
    return this.client.post('/grade-center/level-configs', data)
  }

  async updateLevelConfig(id: string, data: LevelConfigRequest): Promise<LevelConfig> {
    return this.client.put(`/grade-center/level-configs/${encodePathSegment(id)}`, data)
  }

  async listSemesters(): Promise<Semester[]> {
    return this.client.get('/grade-center/semesters')
  }

  async createSemester(data: SemesterRequest): Promise<Semester> {
    return this.client.post('/grade-center/semesters', data)
  }

  async submitReview(data: GradeReviewRequest): Promise<GradeReview> {
    return this.client.post('/grade-center/reviews', data)
  }

  async listReviews(params?: {
    status?: GradeReviewStatus
    /** 筛选锁定状态，返回的总数遵循同一筛选。 */
    is_locked?: BoolFilter
    semester_id?: string
    page?: number
    size?: number
  }): Promise<PaginatedResponse<GradeReview>> {
    return this.client.get('/grade-center/reviews', params)
  }

  async listOwnReviews(params?: {
    status?: GradeReviewStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<GradeReview>> {
    return this.client.get('/grade-center/reviews/mine', params)
  }

  async approveReview(id: string, data: ReviewDecision): Promise<GradeReview> {
    return this.client.post(`/grade-center/reviews/${encodePathSegment(id)}/approve`, data)
  }

  async rejectReview(id: string, data: ReviewDecision): Promise<GradeReview> {
    return this.client.post(`/grade-center/reviews/${encodePathSegment(id)}/reject`, data)
  }

  async unlockReview(id: string, data: ReviewDecision): Promise<GradeReview> {
    return this.client.post(`/grade-center/reviews/${encodePathSegment(id)}/unlock`, data)
  }

  /** 学生本人或校管读取已发布学业记录，教师通过 M6 任教课程成绩查看。 */
  async studentGrades(
    studentId: string,
    params: { semester?: string; page?: number; size?: number } = {}
  ): Promise<StudentGradePage> {
    return this.client.get(`/grade-center/students/${encodePathSegment(studentId)}/grades`, params)
  }

  async studentGPA(studentId: string): Promise<GradeSummary[]> {
    return this.client.get(`/grade-center/students/${encodePathSegment(studentId)}/gpa`)
  }

  async recomputeStudentGrade(
    studentId: string,
    data: { semester_id: string }
  ): Promise<GradeSummary> {
    return this.client.post(
      `/grade-center/students/${encodePathSegment(studentId)}/recompute`,
      data
    )
  }

  async submitAppeal(data: GradeAppealRequest): Promise<GradeAppeal> {
    return this.client.post('/grade-center/appeals', data)
  }

  /** 服务端按身份限制可见范围：本人、任教课程或本校。 */
  async listAppeals(params?: {
    status?: GradeAppealStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<GradeAppeal>> {
    return this.client.get('/grade-center/appeals', params)
  }

  async acceptAppeal(id: string, data: { comment?: string }): Promise<GradeAppeal> {
    return this.client.post(`/grade-center/appeals/${encodePathSegment(id)}/accept`, data)
  }

  async rejectAppeal(id: string, data: { comment?: string }): Promise<GradeAppeal> {
    return this.client.post(`/grade-center/appeals/${encodePathSegment(id)}/reject`, data)
  }

  async getWarningRules(): Promise<WarningRules> {
    return this.client.get('/grade-center/warning-rules')
  }

  async updateWarningRules(data: WarningRules): Promise<WarningRules> {
    return this.client.put('/grade-center/warning-rules', data)
  }

  async listWarnings(params?: {
    student_id?: string
    status?: GradeWarningStatus
    semester_id?: string
    page?: number
    size?: number
  }): Promise<PaginatedResponse<GradeWarning>> {
    return this.client.get('/grade-center/warnings', params)
  }

  async ackWarning(id: string): Promise<GradeWarning> {
    return this.client.post(`/grade-center/warnings/${encodePathSegment(id)}/ack`)
  }

  async scanWarnings(data: {
    student_id?: string
    semester_id?: string
  }): Promise<WarningScanResult> {
    return this.client.post('/grade-center/warnings/scan', data)
  }

  async generateTranscript(data: TranscriptRequest): Promise<GradeTranscript> {
    return this.client.post('/grade-center/transcripts', data)
  }

  async generateTranscriptBatch(data: {
    student_ids: SnowflakeID[]
    scope: TranscriptScope
    semester_id?: SnowflakeID
  }): Promise<GradeTranscript[]> {
    return this.client.post('/grade-center/transcripts/batch', data)
  }

  /** 签发短时下载授权，由文件接口取件。 */
  async transcriptDownloadGrant(id: string): Promise<TranscriptDownloadGrant> {
    return this.client.get(`/grade-center/transcripts/${encodePathSegment(id)}`)
  }
}
