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
  Progress,
  ProgressRequest,
  JoinCourseRequest,
  Assignment,
  AssignmentRequest,
  AssignmentDetail,
  Draft,
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
    role?: 'teacher' | 'student'
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Course>> {
    return this.client.get('/teaching/courses', params)
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
    return this.client.patch(`/teaching/courses/${courseId}`, data)
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
  async updateChapter(courseId: string, chapterId: string, data: ChapterRequest): Promise<Chapter> {
    return this.client.patch(`/teaching/courses/${courseId}/chapters/${chapterId}`, data)
  }

  /**
   * 删除章节
   */
  async deleteChapter(courseId: string, chapterId: string): Promise<void> {
    return this.client.delete(`/teaching/courses/${courseId}/chapters/${chapterId}`)
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
  async updateLesson(chapterId: string, lessonId: string, data: LessonRequest): Promise<Lesson> {
    return this.client.patch(`/teaching/chapters/${chapterId}/lessons/${lessonId}`, data)
  }

  /**
   * 删除课时
   */
  async deleteLesson(chapterId: string, lessonId: string): Promise<void> {
    return this.client.delete(`/teaching/chapters/${chapterId}/lessons/${lessonId}`)
  }

  /**
   * 上报课时学习进度
   */
  async reportProgress(lessonId: string, data: ProgressRequest): Promise<Progress> {
    return this.client.post(`/teaching/lessons/${lessonId}/progress`, data)
  }

  // ===== 作业 =====

  /**
   * 获取作业详情（含题目列表）
   */
  async getAssignment(assignmentId: string): Promise<AssignmentDetail> {
    return this.client.get(`/teaching/assignments/${assignmentId}`)
  }

  /**
   * 创建作业
   */
  async createAssignment(courseId: string, data: AssignmentRequest): Promise<AssignmentDetail> {
    return this.client.post(`/teaching/courses/${courseId}/assignments`, data)
  }

  /**
   * 更新作业
   */
  async updateAssignment(assignmentId: string, data: AssignmentRequest): Promise<AssignmentDetail> {
    return this.client.patch(`/teaching/assignments/${assignmentId}`, data)
  }

  /**
   * 发布作业
   */
  async publishAssignment(assignmentId: string): Promise<Assignment> {
    return this.client.post(`/teaching/assignments/${assignmentId}/publish`)
  }

  // ===== 提交 =====

  /**
   * 获取学生提交列表
   */
  async getSubmissions(assignmentId: string, params?: {
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
  async saveDraft(assignmentId: string, data: { content: Record<string, any> }): Promise<{ updated_at: string }> {
    return this.client.post(`/teaching/assignments/${assignmentId}/draft`, data)
  }

  /**
   * 获取草稿
   */
  async getDraft(assignmentId: string): Promise<Draft> {
    return this.client.get(`/teaching/assignments/${assignmentId}/draft`)
  }
}
