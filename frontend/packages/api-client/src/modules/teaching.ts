// Teaching API：课程、作业、提交
// 对应后端 M6 模块

import { ApiClient } from '../client'
import type {
  Course,
  CourseRequest,
  CourseOutline,
  Chapter,
  ChapterRequest,
  Lesson,
  LessonRequest,
  JoinCourseRequest,
  Assignment,
  AssignmentRequest,
  AssignmentDetail,
  Submission,
  SubmitRequest,
  PaginatedResponse,
} from '../types'

export class TeachingApi {
  constructor(private client: ApiClient) {}

  // ===== 课程 =====

  /**
   * 获取课程列表
   */
  async getCourses(params?: {
    teacher_id?: string
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Course>> {
    return this.client.get('/teaching/courses', params)
  }

  /**
   * 获取课程详情
   */
  async getCourse(courseId: string): Promise<Course> {
    return this.client.get(`/teaching/courses/${courseId}`)
  }

  /**
   * 创建课程
   */
  async createCourse(data: CourseRequest): Promise<Course> {
    return this.client.post('/teaching/courses', data)
  }

  /**
   * 更新课程
   */
  async updateCourse(courseId: string, data: CourseRequest): Promise<Course> {
    return this.client.put(`/teaching/courses/${courseId}`, data)
  }

  /**
   * 删除课程
   */
  async deleteCourse(courseId: string): Promise<void> {
    return this.client.delete(`/teaching/courses/${courseId}`)
  }

  /**
   * 克隆课程
   */
  async cloneCourse(courseId: string, data: { name: string }): Promise<Course> {
    return this.client.post(`/teaching/courses/${courseId}/clone`, data)
  }

  /**
   * 获取课程大纲（含章节、课时、进度）
   */
  async getCourseOutline(courseId: string): Promise<CourseOutline> {
    return this.client.get(`/teaching/courses/${courseId}/outline`)
  }

  /**
   * 学生加入课程
   */
  async joinCourse(data: JoinCourseRequest): Promise<void> {
    return this.client.post('/teaching/courses/join', data)
  }

  // ===== 章节 =====

  /**
   * 创建章节
   */
  async createChapter(courseId: string, data: ChapterRequest): Promise<Chapter> {
    return this.client.post(`/teaching/courses/${courseId}/chapters`, data)
  }

  /**
   * 更新章节
   */
  async updateChapter(chapterId: string, data: ChapterRequest): Promise<Chapter> {
    return this.client.put(`/teaching/chapters/${chapterId}`, data)
  }

  /**
   * 删除章节
   */
  async deleteChapter(chapterId: string): Promise<void> {
    return this.client.delete(`/teaching/chapters/${chapterId}`)
  }

  // ===== 课时 =====

  /**
   * 创建课时
   */
  async createLesson(chapterId: string, data: LessonRequest): Promise<Lesson> {
    return this.client.post(`/teaching/chapters/${chapterId}/lessons`, data)
  }

  /**
   * 更新课时
   */
  async updateLesson(lessonId: string, data: LessonRequest): Promise<Lesson> {
    return this.client.put(`/teaching/lessons/${lessonId}`, data)
  }

  /**
   * 删除课时
   */
  async deleteLesson(lessonId: string): Promise<void> {
    return this.client.delete(`/teaching/lessons/${lessonId}`)
  }

  /**
   * 标记课时完成
   */
  async completeLesson(lessonId: string): Promise<void> {
    return this.client.post(`/teaching/lessons/${lessonId}/complete`)
  }

  // ===== 作业 =====

  /**
   * 获取作业列表
   */
  async getAssignments(courseId: string, params?: {
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Assignment>> {
    return this.client.get(`/teaching/courses/${courseId}/assignments`, params)
  }

  /**
   * 获取作业详情（含题目列表）
   */
  async getAssignment(assignmentId: string): Promise<AssignmentDetail> {
    return this.client.get(`/teaching/assignments/${assignmentId}`)
  }

  /**
   * 创建作业
   */
  async createAssignment(courseId: string, data: AssignmentRequest): Promise<Assignment> {
    return this.client.post(`/teaching/courses/${courseId}/assignments`, data)
  }

  /**
   * 更新作业
   */
  async updateAssignment(assignmentId: string, data: AssignmentRequest): Promise<Assignment> {
    return this.client.put(`/teaching/assignments/${assignmentId}`, data)
  }

  /**
   * 删除作业
   */
  async deleteAssignment(assignmentId: string): Promise<void> {
    return this.client.delete(`/teaching/assignments/${assignmentId}`)
  }

  /**
   * 发布作业
   */
  async publishAssignment(assignmentId: string): Promise<void> {
    return this.client.post(`/teaching/assignments/${assignmentId}/publish`)
  }

  // ===== 提交 =====

  /**
   * 获取学生提交列表
   */
  async getSubmissions(assignmentId: string, params?: {
    student_id?: string
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Submission>> {
    return this.client.get(`/teaching/assignments/${assignmentId}/submissions`, params)
  }

  /**
   * 获取提交详情
   */
  async getSubmission(submissionId: string): Promise<Submission> {
    return this.client.get(`/teaching/submissions/${submissionId}`)
  }

  /**
   * 提交作业
   */
  async submitAssignment(assignmentId: string, data: SubmitRequest): Promise<Submission> {
    return this.client.post(`/teaching/assignments/${assignmentId}/submit`, data)
  }

  /**
   * 保存草稿
   */
  async saveDraft(assignmentId: string, data: { content: Record<string, any> }): Promise<void> {
    return this.client.post(`/teaching/assignments/${assignmentId}/draft`, data)
  }

  /**
   * 获取草稿
   */
  async getDraft(assignmentId: string): Promise<{ content: Record<string, any> }> {
    return this.client.get(`/teaching/assignments/${assignmentId}/draft`)
  }
}
