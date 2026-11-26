// 教学模块的 HTTP 请求与响应契约，鉴权和传输统一交给 ApiClient。
import { ApiClient, encodePathSegment } from '../client'
import type { BoolFilter } from '../constants/filters'
import type { CourseStatus, SubmissionStatus } from '../constants/teaching'
import type { PaginatedResponse } from '../types/common'
import type {
  Course,
  SharedCourse,
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
  CourseGradeSource,
  GradeWeightRequest,
  OverrideGradeRequest,
  ProgressStats,
  TeachingAnnouncement,
  TeachingAnnouncementRequest,
  TeachingCourseGrade,
  TeachingPost,
  TeachingPostRequest,
  TeachingReview,
  MyCourseReview,
  TeachingReviewRequest,
} from '../types/teaching'
import type { TransferTask } from '../types/transfer'

export class TeachingApi {
  constructor(private client: ApiClient) {}

  async getCourses(params?: {
    role?: 'teacher' | 'student'
    status?: CourseStatus
    /** 共享筛选在服务端执行，总数与筛选一致。 */
    is_shared?: BoolFilter
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Course>> {
    return this.client.get('/teaching/courses', params)
  }

  /** 仅返回外校共享课程目录；克隆时由服务端重新检查权限。 */
  async getSharedCourses(params?: {
    keyword?: string
    page?: number
    size?: number
  }): Promise<PaginatedResponse<SharedCourse>> {
    return this.client.get('/teaching/courses/shared', params)
  }

  async createCourse(data: CourseRequest): Promise<Course> {
    return this.client.post('/teaching/courses', data)
  }

  async updateCourse(courseId: string, data: CourseRequest): Promise<Course> {
    return this.client.patch(`/teaching/courses/${encodePathSegment(courseId)}`, data)
  }

  async publishCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/publish`)
  }

  /** 结束已发布或进行中的课程，实验资源由后台回收。 */
  async endCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/end`)
  }

  /** 归档已结束的课程。 */
  async archiveCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/archive`)
  }

  async cloneCourse(courseId: string, data: { name: string }): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/clone`, data)
  }

  async shareCourse(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/share`)
  }

  /** 撤回共享，保留原课程、学生记录和已建立的副本。 */
  async unshareCourse(courseId: string): Promise<Course> {
    return this.client.delete(`/teaching/courses/${encodePathSegment(courseId)}/share`)
  }

  async refreshInviteCode(courseId: string): Promise<Course> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/invite-code/refresh`)
  }

  /** 一次取得课程、章节、课时与本人进度。 */
  async getCourseOutline(courseId: string): Promise<CourseOutline> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/outline`)
  }

  async joinCourse(data: JoinCourseRequest): Promise<CourseMember> {
    return this.client.post('/teaching/courses/join', data)
  }

  async createChapter(courseId: string, data: ChapterRequest): Promise<Chapter> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/chapters`, data)
  }

  async updateChapter(courseId: string, chapterId: string, data: ChapterRequest): Promise<Chapter> {
    return this.client.patch(
      `/teaching/courses/${encodePathSegment(courseId)}/chapters/${encodePathSegment(chapterId)}`,
      data
    )
  }

  async deleteChapter(courseId: string, chapterId: string): Promise<void> {
    return this.client.delete(
      `/teaching/courses/${encodePathSegment(courseId)}/chapters/${encodePathSegment(chapterId)}`
    )
  }

  async getLesson(lessonId: string): Promise<Lesson> {
    return this.client.get(`/teaching/lessons/${encodePathSegment(lessonId)}`)
  }

  async createLesson(chapterId: string, data: LessonRequest): Promise<Lesson> {
    return this.client.post(`/teaching/chapters/${encodePathSegment(chapterId)}/lessons`, data)
  }

  async updateLesson(chapterId: string, lessonId: string, data: LessonRequest): Promise<Lesson> {
    return this.client.patch(
      `/teaching/chapters/${encodePathSegment(chapterId)}/lessons/${encodePathSegment(lessonId)}`,
      data
    )
  }

  async deleteLesson(chapterId: string, lessonId: string): Promise<void> {
    return this.client.delete(
      `/teaching/chapters/${encodePathSegment(chapterId)}/lessons/${encodePathSegment(lessonId)}`
    )
  }

  /** 替换课时材料；服务端按实际文件类型更新课时形态和对象引用。 */
  async uploadLessonMaterial(
    lessonId: string,
    file: File,
    onProgress?: (progress: number) => void
  ): Promise<Lesson> {
    const formData = new FormData()
    formData.append('file', file)
    return this.client.sendFormData(
      'POST',
      `/teaching/lessons/${encodePathSegment(lessonId)}/material`,
      formData,
      onProgress
    )
  }

  /** 视频授权用于 streamUrl 续播，附件授权用于 consumeGrant 一次性取件。 */
  async issueLessonMaterialAccess(lessonId: string): Promise<LessonMaterialAccess> {
    return this.client.post(`/teaching/lessons/${encodePathSegment(lessonId)}/material/access`)
  }

  /** 上传仅返回暂存引用，随创建或编辑课程提交后才生效。 */
  async uploadCourseCover(
    file: File,
    onProgress?: (progress: number) => void
  ): Promise<CourseCoverUpload> {
    const formData = new FormData()
    formData.append('file', file)
    return this.client.sendFormData('POST', '/teaching/courses/cover', formData, onProgress)
  }

  /** 按课程读取权限换取可重复读取的封面授权，交给 streamUrl 显示。 */
  async issueCourseCoverAccess(courseId: string): Promise<CourseCoverAccess> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/cover/access`)
  }

  async reportProgress(lessonId: string, data: ProgressRequest): Promise<Progress> {
    return this.client.post(`/teaching/lessons/${encodePathSegment(lessonId)}/progress`, data)
  }

  async getMyProgress(courseId: string): Promise<Progress[]> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/my-progress`)
  }

  async listMembers(
    courseId: string,
    params?: { page?: number; size?: number }
  ): Promise<PaginatedResponse<CourseMember>> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/members`, params)
  }

  async addMembers(courseId: string, data: BatchMembersRequest): Promise<CourseMember[]> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/members/batch`, data)
  }

  async removeMember(courseId: string, studentId: string): Promise<void> {
    return this.client.delete(
      `/teaching/courses/${encodePathSegment(courseId)}/members/${encodePathSegment(studentId)}`
    )
  }

  /** 教师可见草稿，学生仅见已发布作业；响应不含题面。 */
  async listCourseAssignments(
    courseId: string,
    params?: { page?: number; size?: number }
  ): Promise<PaginatedResponse<Assignment>> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/assignments`, params)
  }

  async getAssignment(assignmentId: string): Promise<AssignmentDetail> {
    return this.client.get(`/teaching/assignments/${encodePathSegment(assignmentId)}`)
  }

  async createAssignment(courseId: string, data: AssignmentRequest): Promise<AssignmentDetail> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/assignments`, data)
  }

  async updateAssignment(assignmentId: string, data: AssignmentRequest): Promise<AssignmentDetail> {
    return this.client.patch(`/teaching/assignments/${encodePathSegment(assignmentId)}`, data)
  }

  async publishAssignment(assignmentId: string): Promise<Assignment> {
    return this.client.post(`/teaching/assignments/${encodePathSegment(assignmentId)}/publish`)
  }

  /** 教师可见全班提交，学生仅见本人历次提交；范围由服务端身份确定。 */
  async getSubmissions(
    assignmentId: string,
    params?: {
      status?: SubmissionStatus
      /** 迟交筛选在服务端执行，总数与筛选一致。 */
      is_late?: BoolFilter
      page?: number
      size?: number
    }
  ): Promise<PaginatedResponse<Submission>> {
    return this.client.get(
      `/teaching/assignments/${encodePathSegment(assignmentId)}/submissions`,
      params
    )
  }

  async getSubmission(submissionId: string): Promise<Submission> {
    return this.client.get(`/teaching/submissions/${encodePathSegment(submissionId)}`)
  }

  async submitAssignment(assignmentId: string, data: SubmitRequest): Promise<Submission> {
    return this.client.post(`/teaching/assignments/${encodePathSegment(assignmentId)}/submit`, data)
  }

  async saveDraft(
    assignmentId: string,
    data: { content: Record<string, unknown> }
  ): Promise<DraftSave> {
    return this.client.post(`/teaching/assignments/${encodePathSegment(assignmentId)}/draft`, data)
  }

  async getDraft(assignmentId: string): Promise<Draft> {
    return this.client.get(`/teaching/assignments/${encodePathSegment(assignmentId)}/draft`)
  }

  async gradeSubmission(
    submissionId: string,
    data: { score: number; comment: string }
  ): Promise<Submission> {
    return this.client.post(`/teaching/submissions/${encodePathSegment(submissionId)}/grade`, data)
  }

  async listPosts(
    courseId: string,
    params?: { page?: number; size?: number }
  ): Promise<PaginatedResponse<TeachingPost>> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/posts`, params)
  }

  async createPost(courseId: string, data: TeachingPostRequest): Promise<TeachingPost> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/posts`, data)
  }

  async likePost(postId: string): Promise<TeachingPost> {
    return this.client.post(`/teaching/posts/${encodePathSegment(postId)}/like`)
  }

  async pinPost(postId: string, isPinned: boolean): Promise<TeachingPost> {
    return this.client.post(`/teaching/posts/${encodePathSegment(postId)}/pin`, {
      is_pinned: isPinned,
    })
  }

  async deletePost(postId: string): Promise<void> {
    return this.client.delete(`/teaching/posts/${encodePathSegment(postId)}`)
  }

  async listAnnouncements(
    courseId: string,
    params?: { page?: number; size?: number }
  ): Promise<PaginatedResponse<TeachingAnnouncement>> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/announcements`, params)
  }

  async createAnnouncement(
    courseId: string,
    data: TeachingAnnouncementRequest
  ): Promise<TeachingAnnouncement> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/announcements`, data)
  }

  async pinAnnouncement(announcementId: string, isPinned: boolean): Promise<TeachingAnnouncement> {
    return this.client.post(`/teaching/announcements/${encodePathSegment(announcementId)}/pin`, {
      is_pinned: isPinned,
    })
  }

  async reviewCourse(courseId: string, data: TeachingReviewRequest): Promise<TeachingReview> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/review`, data)
  }

  /** 未评价时返回明确空值。 */
  async getMyCourseReview(courseId: string): Promise<MyCourseReview> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/review`)
  }

  async getProgressStats(courseId: string): Promise<ProgressStats> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/progress-stats`)
  }

  async listGradeSources(courseId: string): Promise<CourseGradeSource[]> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/grade-sources`)
  }

  async listGradeWeights(courseId: string): Promise<GradeWeight[]> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/grade-weights`)
  }

  async setGradeWeights(courseId: string, data: GradeWeightRequest): Promise<GradeWeight[]> {
    return this.client.put(`/teaching/courses/${encodePathSegment(courseId)}/grade-weights`, data)
  }

  async computeGrades(courseId: string): Promise<TeachingCourseGrade[]> {
    return this.client.post(`/teaching/courses/${encodePathSegment(courseId)}/grades/compute`)
  }

  async listGrades(
    courseId: string,
    params?: { page?: number; size?: number }
  ): Promise<PaginatedResponse<TeachingCourseGrade>> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/grades`, params)
  }

  async overrideGrade(
    courseId: string,
    studentId: string,
    data: OverrideGradeRequest
  ): Promise<TeachingCourseGrade> {
    return this.client.patch(
      `/teaching/courses/${encodePathSegment(courseId)}/grades/${encodePathSegment(studentId)}`,
      data
    )
  }

  async exportGrades(courseId: string): Promise<TransferTask> {
    return this.client.get(`/teaching/courses/${encodePathSegment(courseId)}/grades/export`)
  }
}
