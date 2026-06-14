// Grade API：成绩中心
// 对应后端 M11 模块

import { ApiClient } from '../client'
import type {
  GradeTranscript,
  GradeApplication,
  GradeApplicationRequest,
  GradeReviewRequest,
  GradeAppeal,
  GradeAppealRequest,
  GradeWarning,
  PaginatedResponse,
} from '../types'

export class GradeApi {
  constructor(private client: ApiClient) {}

  // ===== 成绩单 =====

  /**
   * 获取学生成绩单
   */
  async getMyTranscript(): Promise<GradeTranscript> {
    return this.client.get('/grade/transcript/my')
  }

  /**
   * 获取指定学生成绩单（教师/管理员）
   */
  async getStudentTranscript(studentId: string): Promise<GradeTranscript> {
    return this.client.get(`/grade/transcript/student/${studentId}`)
  }

  /**
   * 导出成绩单 PDF
   */
  async exportTranscriptPDF(studentId: string): Promise<void> {
    return this.client.download(`/grade/transcript/${studentId}/pdf`, `transcript_${studentId}.pdf`)
  }

  // ===== 成绩上报与审核 =====

  /**
   * 教师提交成绩上报申请
   */
  async submitGradeApplication(data: GradeApplicationRequest): Promise<GradeApplication> {
    return this.client.post('/grade/applications', data)
  }

  /**
   * 获取成绩上报申请列表
   */
  async getGradeApplications(params?: {
    course_id?: string
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<GradeApplication>> {
    return this.client.get('/grade/applications', params)
  }

  /**
   * 获取成绩上报申请详情
   */
  async getGradeApplication(applicationId: string): Promise<GradeApplication> {
    return this.client.get(`/grade/applications/${applicationId}`)
  }

  /**
   * 学校管理员审核成绩
   */
  async reviewGradeApplication(applicationId: string, data: GradeReviewRequest): Promise<void> {
    return this.client.post(`/grade/applications/${applicationId}/review`, data)
  }

  /**
   * 撤回成绩上报申请
   */
  async withdrawGradeApplication(applicationId: string): Promise<void> {
    return this.client.post(`/grade/applications/${applicationId}/withdraw`)
  }

  // ===== 成绩申诉 =====

  /**
   * 学生提交成绩申诉
   */
  async submitAppeal(data: GradeAppealRequest): Promise<GradeAppeal> {
    return this.client.post('/grade/appeals', data)
  }

  /**
   * 获取成绩申诉列表
   */
  async getAppeals(params?: {
    course_id?: string
    student_id?: string
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<GradeAppeal>> {
    return this.client.get('/grade/appeals', params)
  }

  /**
   * 获取成绩申诉详情
   */
  async getAppeal(appealId: string): Promise<GradeAppeal> {
    return this.client.get(`/grade/appeals/${appealId}`)
  }

  /**
   * 教师处理成绩申诉
   */
  async handleAppeal(
    appealId: string,
    data: { status: number; reply: string; new_score?: number }
  ): Promise<void> {
    return this.client.post(`/grade/appeals/${appealId}/handle`, data)
  }

  // ===== 学业预警 =====

  /**
   * 获取学业预警列表
   */
  async getWarnings(params?: {
    student_id?: string
    level?: number
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<GradeWarning>> {
    return this.client.get('/grade/warnings', params)
  }

  /**
   * 获取我的学业预警
   */
  async getMyWarnings(): Promise<GradeWarning[]> {
    return this.client.get('/grade/warnings/my')
  }

  /**
   * 确认已阅学业预警
   */
  async acknowledgeWarning(warningId: string): Promise<void> {
    return this.client.post(`/grade/warnings/${warningId}/acknowledge`)
  }

  /**
   * 处理学业预警（辅导员/管理员）
   */
  async handleWarning(warningId: string, data: { status: number; note: string }): Promise<void> {
    return this.client.post(`/grade/warnings/${warningId}/handle`, data)
  }

  // ===== 成绩配置 =====

  /**
   * 获取 GPA 计算规则
   */
  async getGPAConfig(): Promise<Record<string, any>> {
    return this.client.get('/grade/config/gpa')
  }

  /**
   * 更新 GPA 计算规则（管理员）
   */
  async updateGPAConfig(data: Record<string, any>): Promise<void> {
    return this.client.put('/grade/config/gpa', data)
  }

  /**
   * 获取预警规则
   */
  async getWarningRules(): Promise<Record<string, any>> {
    return this.client.get('/grade/config/warning-rules')
  }

  /**
   * 更新预警规则（管理员）
   */
  async updateWarningRules(data: Record<string, any>): Promise<void> {
    return this.client.put('/grade/config/warning-rules', data)
  }
}
