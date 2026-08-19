// Teaching API：课程、作业、提交
// 对应后端 M6 模块

import { ApiClient, encodePathSegment } from '../client'
import type { CourseStatus, SubmissionStatus } from '../constants/teaching'
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
  DraftSave,
  LessonMaterialAccess,
  CourseCoverUpload,
  CourseCoverAccess,
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
    return this.client.patch(`/teaching/courses/${encodePathSegment(courseId)}`, data)
  }

  /**
   * 发布课程。
   */
  async publishCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/publish`)
  }

  /**
   * 结束进行中的课程。
   */
  async endCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/end`)
  }

  /**
   * 归档课程。
   */
  async archiveCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/archive`)
  }

  /**
   * 克隆课程
   */
  async cloneCourse(courseId: string, data: { name: string }): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/clone`, data)
  }

  /**
   * 将课程设为共享库可见。
   */
  async shareCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/share`)
  }

  /**
   * 刷新课程邀请码。
   */
  async refreshInviteCode(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/invite-code/refresh`)
  }

  /**
   * 获取课程大纲（含章节、课时、进度）
   */
  async getCourseOutline(courseId: string): Promise<CourseOutline> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/outline`)
  }

  /**
   * 学生加入课程
   */
  async joinCourse(data: JoinCourseRequest): Promise<void> {
    return this.client.post('/teaching/courses/join', data)
  }

  // ===== 章节 =====
  // 章节与课时的只读取用统一走 getCourseOutline:它一次返回课程 + 章节 + 课时 + 本人进度。
  // 不另开「只取章节」「只取某章课时」两条方法 —— 那是同一份数据的子集形态,会变成第二套读路径。

  /**
   * 创建章节
   */
  async createChapter(courseId: string, data: ChapterRequest): Promise<Chapter> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/chapters`, data)
  }

  /**
   * 更新章节
   */
  async updateChapter(courseId: string, chapterId: string, data: ChapterRequest): Promise<Chapter> {
    return this.client.patch(`/teaching/courses/${encodePathSegment(courseId)}/chapters/${encodePathSegment(chapterId)}`, data)
  }

  /**
   * 删除章节
   */
  async deleteChapter(courseId: string, chapterId: string): Promise<void> {
    return this.client.delete(`/teaching/courses/${encodePathSegment(courseId)}/chapters/${encodePathSegment(chapterId)}`)
  }

  // ===== 课时 =====

  /**
   * 查询单个课时详情。
   */
  async getLesson(lessonId: string): Promise<Lesson> {
    return this.client.get(`/teaching/lessons/${encodePathSegment(lessonId)}`)
  }

  /**
   * 创建课时
   */
  async createLesson(chapterId: string, data: LessonRequest): Promise<Lesson> {
    return this.client.post(`/teaching/chapters/${encodePathSegment(chapterId)}/lessons`, data)
  }

  /**
   * 更新课时
   */
  async updateLesson(chapterId: string, lessonId: string, data: LessonRequest): Promise<Lesson> {
    return this.client.patch(`/teaching/chapters/${encodePathSegment(chapterId)}/lessons/${encodePathSegment(lessonId)}`, data)
  }

  /**
   * 删除课时
   */
  async deleteLesson(chapterId: string, lessonId: string): Promise<void> {
    return this.client.delete(`/teaching/chapters/${encodePathSegment(chapterId)}/lessons/${encodePathSegment(lessonId)}`)
  }

  /**
   * 上传课时视频或附件。
   * 服务端按文件类型置 content_type 并把对象引用写进 content_ref;
   * 同一课时重复上传即替换(content_type 单值,课时只有一种形态)。
   */
  async uploadLessonMaterial(
    lessonId: string,
    file: File,
    onProgress?: (progress: number) => void
  ): Promise<Lesson> {
    const formData = new FormData()
    formData.append('file', file)
    return this.client.postFormData(`/teaching/lessons/${encodePathSegment(lessonId)}/material`, formData, onProgress)
  }

  /**
   * 换取课时材料投放授权。
   * 视频回 mode=stream(可续播),附件回 mode=download(一次性取件),由课时形态决定。
   * 拿到 token 后:视频交给 storage.streamUrl 作播放地址,附件交给 storage.consumeGrant 取件。
   */
  async issueLessonMaterialAccess(lessonId: string): Promise<LessonMaterialAccess> {
    return this.client.post(`/teaching/lessons/${encodePathSegment(lessonId)}/material/access`)
  }

  /**
   * 上传课程封面,返回对象引用。
   * 引用要随 createCourse 或 updateCourse 一起提交才会生效 —— 上传不绑课程 id,
   * 所以「新建课程时先传图」与「后续改封面」共用这一条路径。
   */
  async uploadCourseCover(
    file: File,
    onProgress?: (progress: number) => void
  ): Promise<CourseCoverUpload> {
    const formData = new FormData()
    formData.append('file', file)
    return this.client.postFormData('/teaching/courses/cover', formData, onProgress)
  }

  /**
   * 换取课程封面投放授权,拿到 token 后交给 storage.streamUrl 作 img 地址。
   * 封面只对能看到该课程的人可见,故没有公开路径,每次显示都要重新换取授权。
   */
  async issueCourseCoverAccess(courseId: string): Promise<CourseCoverAccess> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/cover/access`)
  }

  /**
   * 上报课时学习进度
   */
  async reportProgress(lessonId: string, data: ProgressRequest): Promise<Progress> {
    return this.client.post(`/teaching/lessons/${encodePathSegment(lessonId)}/progress`, data)
  }

  /**
   * 查询当前学生在课程内的学习进度。
   */
  async getMyProgress(courseId: string): Promise<Progress[]> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/my-progress`)
  }

  // ===== 成员 =====

  /**
   * 查询课程成员列表。
   */
  async listMembers(courseId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<CourseMember>> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/members`, params)
  }

  /**
   * 批量添加课程成员。
   */
  async addMembers(courseId: string, data: BatchMembersRequest): Promise<CourseMember[]> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/members/batch`, data)
  }

  /**
   * 移除课程成员。
   */
  async removeMember(courseId: string, studentId: string): Promise<void> {
    return this.client.delete(`/teaching/courses/${encodePathSegment(courseId)}/members/${encodePathSegment(studentId)}`)
  }

  // ===== 作业 =====

  /**
   * 查询课程作业清单。
   * 服务端按身份分视角：授课教师得到含草稿的全量，课程内学生只得到已发布作业。
   * 这是学生取得作业编号的唯一入口（课程大纲只含章节课时），响应为作业外壳不含题面。
   */
  async listCourseAssignments(courseId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<Assignment>> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/assignments`, params)
  }

  /**
   * 获取作业详情（含题目列表）
   */
  async getAssignment(assignmentId: string): Promise<AssignmentDetail> {
    return this.client.get(`/teaching/assignments/${encodePathSegment(assignmentId)}`)
  }

  /**
   * 创建作业
   */
  async createAssignment(courseId: string, data: AssignmentRequest): Promise<AssignmentDetail> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/assignments`, data)
  }

  /**
   * 更新作业
   */
  async updateAssignment(assignmentId: string, data: AssignmentRequest): Promise<AssignmentDetail> {
    return this.client.patch(`/teaching/assignments/${encodePathSegment(assignmentId)}`, data)
  }

  /**
   * 发布作业
   */
  async publishAssignment(assignmentId: string): Promise<Assignment> {
    return this.client.post(`/teaching/assignments/${encodePathSegment(assignmentId)}/publish`)
  }

  // ===== 提交 =====

  /**
   * 查询作业提交列表。
   * 服务端按身份分视角：授课教师得到全班提交（批改列表），课程内学生只得到本人历次提交
   * （学生刷新页面后据此取回提交编号）。学生编号由服务端会话决定，不接受客户端传参。
   */
  async getSubmissions(assignmentId: string, params?: {
    status?: SubmissionStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Submission>> {
    return this.client.get(`/teaching/assignments/${encodePathSegment(assignmentId)}/submissions`, params)
  }

  /**
   * 获取提交详情
   */
  async getSubmission(submissionId: string): Promise<Submission> {
    return this.client.get(`/teaching/submissions/${encodePathSegment(submissionId)}`)
  }

  /**
   * 提交作业
   */
  async submitAssignment(assignmentId: string, data: SubmitRequest): Promise<Submission> {
    return this.client.post(`/teaching/assignments/${encodePathSegment(assignmentId)}/submit`, data)
  }

  /**
   * 保存草稿
   */
  async saveDraft(assignmentId: string, data: { content: Record<string, unknown> }): Promise<DraftSave> {
    return this.client.post(`/teaching/assignments/${encodePathSegment(assignmentId)}/draft`, data)
  }

  /**
   * 获取草稿
   */
  async getDraft(assignmentId: string): Promise<Draft> {
    return this.client.get(`/teaching/assignments/${encodePathSegment(assignmentId)}/draft`)
  }

  /**
   * 教师批改作业提交。
   */
  async gradeSubmission(submissionId: string, data: { score: number; comment: string }): Promise<Submission> {
    return this.client.post(`/teaching/submissions/${encodePathSegment(submissionId)}/grade`, data)
  }

  // ===== 讨论与公告 =====

  /**
   * 查询课程讨论帖。
   */
  async listPosts(courseId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<TeachingPost>> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/posts`, params)
  }

  /**
   * 发布课程讨论帖。
   */
  async createPost(courseId: string, data: TeachingPostRequest): Promise<TeachingPost> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/posts`, data)
  }

  /**
   * 点赞讨论帖。
   */
  async likePost(postId: string): Promise<TeachingPost> {
    return this.client.post(`/teaching/posts/${encodePathSegment(postId)}/like`)
  }

  /**
   * 置顶讨论帖。
   */
  async pinPost(postId: string): Promise<TeachingPost> {
    return this.client.post(`/teaching/posts/${encodePathSegment(postId)}/pin`)
  }

  /**
   * 删除讨论帖。
   */
  async deletePost(postId: string): Promise<void> {
    return this.client.delete(`/teaching/posts/${encodePathSegment(postId)}`)
  }

  /**
   * 查询课程公告。
   */
  async listAnnouncements(courseId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<TeachingAnnouncement>> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/announcements`, params)
  }

  /**
   * 创建课程公告。
   */
  async createAnnouncement(courseId: string, data: TeachingAnnouncementRequest): Promise<TeachingAnnouncement> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/announcements`, data)
  }

  /**
   * 置顶课程公告。
   */
  async pinAnnouncement(announcementId: string): Promise<TeachingAnnouncement> {
    return this.client.post(`/teaching/announcements/${encodePathSegment(announcementId)}/pin`)
  }

  /**
   * 提交课程评价。
   */
  async reviewCourse(courseId: string, data: TeachingReviewRequest): Promise<TeachingReview> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/review`, data)
  }

  // ===== 进度与成绩 =====

  /**
   * 查询课程学习进度统计。
   */
  async getProgressStats(courseId: string): Promise<ProgressStats> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/progress-stats`)
  }

  /**
   * 查询课程成绩权重。
   */
  async listGradeWeights(courseId: string): Promise<GradeWeight[]> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/grade-weights`)
  }

  /**
   * 设置课程成绩权重。
   */
  async setGradeWeights(courseId: string, data: GradeWeightRequest): Promise<GradeWeight[]> {
    return this.client.put(`/teaching/courses/${encodePathSegment(courseId)}/grade-weights`, data)
  }

  /**
   * 触发课程成绩计算。
   */
  async computeGrades(courseId: string): Promise<TeachingCourseGrade[]> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/grades/compute`)
  }

  /**
   * 查询课程成绩汇总。后端按课程成绩契约返回完整数组,不分页。
   */
  async listGrades(courseId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<TeachingCourseGrade>> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/grades`, params)
  }

  /**
   * 人工调整单个学生课程总评。
   */
  async overrideGrade(courseId: string, studentId: string, data: OverrideGradeRequest): Promise<TeachingCourseGrade> {
    return this.client.patch(`/teaching/courses/${encodePathSegment(courseId)}/grades/${encodePathSegment(studentId)}`, data)
  }

  /**
   * 创建课程成绩导出任务。
   */
  async exportGrades(courseId: string): Promise<TransferTask> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/grades/export`)
  }
}
