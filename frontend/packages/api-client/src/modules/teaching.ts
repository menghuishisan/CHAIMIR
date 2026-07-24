// Teaching API：课程、作业、提交
// 对应后端 M6 模块

import { ApiClient } from '../client'
import type { CourseStatus, LessonContentType } from '../constants/teaching'
import type { PaginatedResponse } from '../types/common'
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
  BatchMembersRequest,
  CourseMember,
  GradeWeight,
  GradeWeightRequest,
  OverrideGradeRequest,
  ProgressStats,
  TeachingAnnouncement,
  TeachingAnnouncementRequest,
  TeachingCourseGrade,
  TeachingPost,
  TeachingPostRequest,
  TeachingReview,
  TeachingReviewRequest,
  LessonContentRef,
} from '../types/teaching'
import type { TransferTask } from '../types/transfer'

/**
 * TeachingApi 封装后端 M6 课程、课时、成员、作业和成绩接口。
 */
export class TeachingApi {
  /**
   * constructor 注入统一 API 客户端，避免教学模块自行处理鉴权和错误格式。
   */
  constructor(private client: ApiClient) {}

  // ===== 课程 =====

  /**
   * 获取课程列表
   */
  async getCourses(params?: {
    role?: 'teacher' | 'student'
    status?: CourseStatus
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
   * 发布课程。
   */
  async publishCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${courseId}/publish`)
  }

  /**
   * 结束进行中的课程。
   */
  async endCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${courseId}/end`)
  }

  /**
   * 归档课程。
   */
  async archiveCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${courseId}/archive`)
  }

  /**
   * 克隆课程
   */
  async cloneCourse(courseId: string, data: { name: string }): Promise<Course> {
    return this.client.post(`/teaching/courses/${courseId}/clone`, data)
  }

  /**
   * 将课程设为共享库可见。
   */
  async shareCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${courseId}/share`)
  }

  /**
   * 刷新课程邀请码。
   */
  async refreshInviteCode(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${courseId}/invite-code/refresh`)
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
   * 查询课程章节列表。
   */
  async listChapters(courseId: string): Promise<Chapter[]> {
    return this.client.get(`/teaching/courses/${courseId}/chapters`)
  }

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
   * 查询章节课时列表。
   */
  async listLessons(chapterId: string): Promise<Lesson[]> {
    return this.client.get(`/teaching/chapters/${chapterId}/lessons`)
  }

  /**
   * 查询单个课时详情。
   */
  async getLesson(lessonId: string): Promise<Lesson> {
    return this.client.get(`/teaching/lessons/${lessonId}`)
  }

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
   * 设置课时内容引用。
   */
  async setLessonContent(
    lessonId: string,
    data: { content_type: LessonContentType; content_ref: LessonContentRef }
  ): Promise<Lesson> {
    return this.client.post(`/teaching/lessons/${lessonId}/content`, data)
  }

  /**
   * 上报课时学习进度
   */
  async reportProgress(lessonId: string, data: ProgressRequest): Promise<Progress> {
    return this.client.post(`/teaching/lessons/${lessonId}/progress`, data)
  }

  /**
   * 查询当前学生在课程内的学习进度。
   */
  async getMyProgress(courseId: string): Promise<Progress[]> {
    return this.client.get(`/teaching/courses/${courseId}/my-progress`)
  }

  // ===== 成员 =====

  /**
   * 查询课程成员列表。
   */
  async listMembers(courseId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<CourseMember>> {
    return this.client.get(`/teaching/courses/${courseId}/members`, params)
  }

  /**
   * 批量添加课程成员。
   */
  async addMembers(courseId: string, data: BatchMembersRequest): Promise<CourseMember[]> {
    return this.client.post(`/teaching/courses/${courseId}/members/batch`, data)
  }

  /**
   * 移除课程成员。
   */
  async removeMember(courseId: string, studentId: string): Promise<void> {
    return this.client.delete(`/teaching/courses/${courseId}/members/${studentId}`)
  }

  // ===== 作业 =====

  /**
   * 获取当前账号可见的课程作业目录。
   */
  async listAssignments(courseId: string): Promise<Assignment[]> {
    return this.client.get(`/teaching/courses/${courseId}/assignments`)
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
  async saveDraft(assignmentId: string, data: { content: Record<string, unknown> }): Promise<{ updated_at: string }> {
    return this.client.post(`/teaching/assignments/${assignmentId}/draft`, data)
  }

  /**
   * 获取草稿
   */
  async getDraft(assignmentId: string): Promise<Draft> {
    return this.client.get(`/teaching/assignments/${assignmentId}/draft`)
  }

  /**
   * 教师批改作业提交。
   */
  async gradeSubmission(submissionId: string, data: { score: number; comment: string }): Promise<Submission> {
    return this.client.post(`/teaching/submissions/${submissionId}/grade`, data)
  }

  // ===== 讨论与公告 =====

  /**
   * 查询课程讨论帖。
   */
  async listPosts(courseId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<TeachingPost>> {
    return this.client.get(`/teaching/courses/${courseId}/posts`, params)
  }

  /**
   * 发布课程讨论帖。
   */
  async createPost(courseId: string, data: TeachingPostRequest): Promise<TeachingPost> {
    return this.client.post(`/teaching/courses/${courseId}/posts`, data)
  }

  /**
   * 点赞讨论帖。
   */
  async likePost(postId: string): Promise<TeachingPost> {
    return this.client.post(`/teaching/posts/${postId}/like`)
  }

  /**
   * 置顶讨论帖。
   */
  async pinPost(postId: string): Promise<TeachingPost> {
    return this.client.post(`/teaching/posts/${postId}/pin`)
  }

  /**
   * 删除讨论帖。
   */
  async deletePost(postId: string): Promise<void> {
    return this.client.delete(`/teaching/posts/${postId}`)
  }

  /**
   * 查询课程公告。
   */
  async listAnnouncements(courseId: string): Promise<TeachingAnnouncement[]> {
    return this.client.get(`/teaching/courses/${courseId}/announcements`)
  }

  /**
   * 创建课程公告。
   */
  async createAnnouncement(courseId: string, data: TeachingAnnouncementRequest): Promise<TeachingAnnouncement> {
    return this.client.post(`/teaching/courses/${courseId}/announcements`, data)
  }

  /**
   * 置顶课程公告。
   */
  async pinAnnouncement(announcementId: string): Promise<TeachingAnnouncement> {
    return this.client.post(`/teaching/announcements/${announcementId}/pin`)
  }

  /**
   * 提交课程评价。
   */
  async reviewCourse(courseId: string, data: TeachingReviewRequest): Promise<TeachingReview> {
    return this.client.post(`/teaching/courses/${courseId}/review`, data)
  }

  // ===== 进度与成绩 =====

  /**
   * 查询课程学习进度统计。
   */
  async getProgressStats(courseId: string): Promise<ProgressStats> {
    return this.client.get(`/teaching/courses/${courseId}/progress-stats`)
  }

  /**
   * 查询课程成绩权重。
   */
  async listGradeWeights(courseId: string): Promise<GradeWeight[]> {
    return this.client.get(`/teaching/courses/${courseId}/grade-weights`)
  }

  /**
   * 设置课程成绩权重。
   */
  async setGradeWeights(courseId: string, data: GradeWeightRequest): Promise<GradeWeight[]> {
    return this.client.put(`/teaching/courses/${courseId}/grade-weights`, data)
  }

  /**
   * 触发课程成绩计算。
   */
  async computeGrades(courseId: string): Promise<TeachingCourseGrade[]> {
    return this.client.post(`/teaching/courses/${courseId}/grades/compute`)
  }

  /**
   * 查询课程成绩列表。
   */
  async listGrades(courseId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<TeachingCourseGrade>> {
    return this.client.get(`/teaching/courses/${courseId}/grades`, params)
  }

  /**
   * 人工调整单个学生课程总评。
   */
  async overrideGrade(courseId: string, studentId: string, data: OverrideGradeRequest): Promise<TeachingCourseGrade> {
    return this.client.patch(`/teaching/courses/${courseId}/grades/${studentId}`, data)
  }

  /**
   * 创建课程成绩导出任务。
   */
  async exportGrades(courseId: string): Promise<TransferTask> {
    return this.client.get(`/teaching/courses/${courseId}/grades/export`)
  }
}
