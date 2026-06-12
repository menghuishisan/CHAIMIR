// teaching api 文件负责注册 M6 HTTP 路由、绑定请求和组合鉴权,不承载教学业务逻辑。
package teaching

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册教学模块 HTTP API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles auth.RoleChecker) error {
	if r == nil {
		return apperr.ErrInternal.WithMessage("teaching routes 缺少 HTTP router")
	}
	if svc == nil {
		return apperr.ErrInternal.WithMessage("teaching routes 缺少 service")
	}
	if authn == nil {
		return apperr.ErrInternal.WithMessage("teaching routes 缺少 auth manager")
	}
	api := teachingAPI{svc: svc}
	g := r.Group("/api/v1/teaching")
	all := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	teacher := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	student := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent))
	internal := g.Group("/internal", authn.ServiceMiddleware())
	api.registerSharedRoutes(all)
	api.registerTeacherRoutes(teacher)
	api.registerStudentRoutes(student)
	api.registerInternalRoutes(internal)
	return nil
}

type teachingAPI struct {
	svc *Service
}

// registerSharedRoutes 注册师生都可访问的只读学习接口。
func (a teachingAPI) registerSharedRoutes(g gin.IRouter) {
	g.GET("/courses", a.listCourses)
	g.GET("/courses/:id/chapters", a.listChapters)
	g.GET("/chapters/:id/lessons", a.listLessons)
	g.GET("/courses/:id/outline", a.getOutline)
	g.GET("/lessons/:id", a.getLesson)
	g.GET("/assignments/:id", a.getAssignment)
	g.GET("/submissions/:id", a.getSubmission)
	g.GET("/courses/:id/posts", a.listPosts)
	g.POST("/courses/:id/posts", a.createPost)
	g.POST("/posts/:id/like", a.likePost)
	g.GET("/courses/:id/announcements", a.listAnnouncements)
	g.GET("/courses/:id/grade-weights", a.listGradeWeights)
}

// registerTeacherRoutes 注册授课教师和学校管理员管理接口。
func (a teachingAPI) registerTeacherRoutes(g gin.IRouter) {
	g.POST("/courses", a.createCourse)
	g.PATCH("/courses/:id", a.updateCourse)
	g.POST("/courses/:id/publish", a.publishCourse)
	g.POST("/courses/:id/end", a.endCourse)
	g.POST("/courses/:id/archive", a.archiveCourse)
	g.POST("/courses/:id/clone", a.cloneCourse)
	g.POST("/courses/:id/share", a.shareCourse)
	g.POST("/courses/:id/invite-code/refresh", a.refreshInviteCode)
	g.POST("/courses/:id/chapters", a.createChapter)
	g.PATCH("/courses/:id/chapters/:cid", a.updateChapter)
	g.DELETE("/courses/:id/chapters/:cid", a.deleteChapter)
	g.POST("/chapters/:id/lessons", a.createLesson)
	g.PATCH("/chapters/:id/lessons/:lid", a.updateLesson)
	g.DELETE("/chapters/:id/lessons/:lid", a.deleteLesson)
	g.POST("/lessons/:id/content", a.setLessonContent)
	g.GET("/courses/:id/members", a.listMembers)
	g.POST("/courses/:id/members/batch", a.addMembers)
	g.DELETE("/courses/:id/members/:sid", a.removeMember)
	g.POST("/courses/:id/assignments", a.createAssignment)
	g.PATCH("/assignments/:id", a.updateAssignment)
	g.POST("/assignments/:id/publish", a.publishAssignment)
	g.GET("/assignments/:id/submissions", a.listSubmissions)
	g.POST("/submissions/:id/grade", a.gradeSubmission)
	g.POST("/posts/:id/pin", a.pinPost)
	g.DELETE("/posts/:id", a.deletePost)
	g.POST("/courses/:id/announcements", a.createAnnouncement)
	g.POST("/announcements/:id/pin", a.pinAnnouncement)
	g.GET("/courses/:id/progress-stats", a.progressStats)
	g.PUT("/courses/:id/grade-weights", a.setGradeWeights)
	g.POST("/courses/:id/grades/compute", a.computeGrades)
	g.GET("/courses/:id/grades", a.listGrades)
	g.PATCH("/courses/:id/grades/:sid", a.overrideGrade)
	g.GET("/courses/:id/grades/export", a.exportGrades)
}

// registerStudentRoutes 注册学生学习、提交和评价接口。
func (a teachingAPI) registerStudentRoutes(g gin.IRouter) {
	g.POST("/courses/join", a.joinCourse)
	g.POST("/lessons/:id/progress", a.reportProgress)
	g.GET("/courses/:id/my-progress", a.myProgress)
	g.GET("/assignments/:id/draft", a.getDraft)
	g.POST("/assignments/:id/draft", a.saveDraft)
	g.POST("/assignments/:id/submit", a.submitAssignment)
	g.POST("/courses/:id/review", a.reviewCourse)
}

// registerInternalRoutes 注册内部服务只读契约 HTTP 入口。
func (a teachingAPI) registerInternalRoutes(g gin.IRouter) {
	g.GET("/stats", a.internalStats)
	g.GET("/courses/:id/grades", a.internalCourseGrades)
}

// listCourses 绑定课程列表过滤参数。
func (a teachingAPI) listCourses(c *gin.Context) {
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 5, HasMax: true})
	if !ok {
		return
	}
	page, size, ok := teachingPage(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListCourses(c.Request.Context(), CourseListFilter{Role: c.Query("role"), Status: int16(status), Page: page, Size: size})
	httpx.WritePage(c, out, total, p, s, err)
}

// createCourse 绑定创建课程请求。
func (a teachingAPI) createCourse(c *gin.Context) {
	var req CourseRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingCourseInvalid) {
		return
	}
	out, err := a.svc.CreateCourse(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// updateCourse 绑定课程编辑请求。
func (a teachingAPI) updateCourse(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req CourseRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingCourseInvalid) {
		return
	}
	out, err := a.svc.UpdateCourse(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// publishCourse 发布课程。
func (a teachingAPI) publishCourse(c *gin.Context) {
	a.writeCourseAction(c, a.svc.PublishCourse)
}

// endCourse 结束课程。
func (a teachingAPI) endCourse(c *gin.Context) {
	a.writeCourseAction(c, a.svc.EndCourse)
}

// archiveCourse 归档课程。
func (a teachingAPI) archiveCourse(c *gin.Context) {
	a.writeCourseAction(c, a.svc.ArchiveCourse)
}

// cloneCourse 绑定课程克隆请求。
func (a teachingAPI) cloneCourse(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req CloneCourseRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingCourseInvalid) {
		return
	}
	out, err := a.svc.CloneCourse(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// shareCourse 共享课程到课程库。
func (a teachingAPI) shareCourse(c *gin.Context) {
	a.writeCourseAction(c, a.svc.ShareCourse)
}

// refreshInviteCode 刷新课程邀请码。
func (a teachingAPI) refreshInviteCode(c *gin.Context) {
	a.writeCourseAction(c, a.svc.RefreshInviteCode)
}

// writeCourseAction 统一处理课程状态类动作。
func (a teachingAPI) writeCourseAction(c *gin.Context, fn func(context.Context, int64) (CourseDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := fn(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// createChapter 绑定章节创建请求。
func (a teachingAPI) createChapter(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ChapterRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingChapterInvalid) {
		return
	}
	out, err := a.svc.CreateChapter(c.Request.Context(), courseID, req)
	httpx.Write(c, out, err)
}

// listChapters 查询课程章节。
func (a teachingAPI) listChapters(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ListChapters(c.Request.Context(), courseID)
	httpx.Write(c, out, err)
}

// updateChapter 绑定章节编辑请求。
func (a teachingAPI) updateChapter(c *gin.Context) {
	chapterID, ok := httpx.PathID(c, "cid")
	if !ok {
		return
	}
	var req ChapterRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingChapterInvalid) {
		return
	}
	out, err := a.svc.UpdateChapter(c.Request.Context(), chapterID, req)
	httpx.Write(c, out, err)
}

// deleteChapter 删除章节。
func (a teachingAPI) deleteChapter(c *gin.Context) {
	id, ok := httpx.PathID(c, "cid")
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.DeleteChapter(c.Request.Context(), id))
}

// createLesson 绑定课时创建请求。
func (a teachingAPI) createLesson(c *gin.Context) {
	chapterID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req LessonRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingLessonInvalid) {
		return
	}
	out, err := a.svc.CreateLesson(c.Request.Context(), chapterID, req)
	httpx.Write(c, out, err)
}

// getLesson 读取课时内容。
func (a teachingAPI) getLesson(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetLessonForUser(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// listLessons 查询章节课时列表。
func (a teachingAPI) listLessons(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ListLessonsByChapter(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// updateLesson 绑定课时编辑请求。
func (a teachingAPI) updateLesson(c *gin.Context) {
	id, ok := httpx.PathID(c, "lid")
	if !ok {
		return
	}
	var req LessonRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingLessonInvalid) {
		return
	}
	out, err := a.svc.UpdateLesson(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// setLessonContent 设置课时内容引用。
func (a teachingAPI) setLessonContent(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req LessonRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingLessonInvalid) {
		return
	}
	out, err := a.svc.SetLessonContent(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// deleteLesson 删除课时。
func (a teachingAPI) deleteLesson(c *gin.Context) {
	id, ok := httpx.PathID(c, "lid")
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.DeleteLesson(c.Request.Context(), id))
}

// joinCourse 绑定学生邀请码入课请求。
func (a teachingAPI) joinCourse(c *gin.Context) {
	var req JoinCourseRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingInviteInvalid) {
		return
	}
	out, err := a.svc.JoinCourseByInvite(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// listMembers 查询课程成员分页。
func (a teachingAPI) listMembers(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := teachingPage(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListCourseMembers(c.Request.Context(), courseID, page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// addMembers 绑定批量加课成员请求。
func (a teachingAPI) addMembers(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req BatchMembersRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingMemberInvalid) {
		return
	}
	out, err := a.svc.AddCourseMembers(c.Request.Context(), courseID, req)
	httpx.Write(c, out, err)
}

// removeMember 移除课程成员。
func (a teachingAPI) removeMember(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	studentID, ok := httpx.PathID(c, "sid")
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.RemoveCourseMember(c.Request.Context(), courseID, studentID))
}

// getOutline 读取课程目录和本人进度。
func (a teachingAPI) getOutline(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetCourseOutline(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// createAssignment 绑定作业创建请求。
func (a teachingAPI) createAssignment(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AssignmentRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingAssignmentInvalid) {
		return
	}
	out, err := a.svc.CreateAssignment(c.Request.Context(), courseID, req)
	httpx.Write(c, out, err)
}

// updateAssignment 绑定作业编辑请求。
func (a teachingAPI) updateAssignment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AssignmentRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingAssignmentInvalid) {
		return
	}
	out, err := a.svc.UpdateAssignment(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// publishAssignment 发布作业。
func (a teachingAPI) publishAssignment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.PublishAssignment(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// getAssignment 读取作业详情。
func (a teachingAPI) getAssignment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetAssignmentForStudent(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// saveDraft 保存作业草稿。
func (a teachingAPI) saveDraft(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req DraftRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingDraftInvalid) {
		return
	}
	out, err := a.svc.SaveDraft(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// getDraft 读取服务端权威作答草稿。
func (a teachingAPI) getDraft(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetDraft(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// submitAssignment 提交作业。
func (a teachingAPI) submitAssignment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req SubmitAssignmentRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingSubmissionInvalid) {
		return
	}
	out, err := a.svc.SubmitAssignment(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// listSubmissions 查询作业提交分页。
func (a teachingAPI) listSubmissions(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := teachingPage(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListSubmissions(c.Request.Context(), id, page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// gradeSubmission 绑定教师批改请求。
func (a teachingAPI) gradeSubmission(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req GradeSubmissionRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingSubmissionInvalid) {
		return
	}
	out, err := a.svc.GradeSubmission(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// getSubmission 读取提交反馈。
func (a teachingAPI) getSubmission(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetSubmissionForUser(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// reportProgress 上报学习进度。
func (a teachingAPI) reportProgress(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ProgressRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingProgressInvalid) {
		return
	}
	out, err := a.svc.ReportProgress(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// progressStats 查询班级进度统计。
func (a teachingAPI) progressStats(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.CourseProgressStats(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// myProgress 查询本人课程进度。
func (a teachingAPI) myProgress(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.MyProgress(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// createPost 创建讨论帖或回复。
func (a teachingAPI) createPost(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req PostRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingDiscussionInvalid) {
		return
	}
	out, err := a.svc.CreatePost(c.Request.Context(), courseID, req)
	httpx.Write(c, out, err)
}

// listPosts 查询课程讨论分页。
func (a teachingAPI) listPosts(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := teachingPage(c)
	if !ok {
		return
	}
	out, p, s, err := a.svc.ListPosts(c.Request.Context(), courseID, page, size)
	httpx.Write(c, gin.H{"items": out, "page": p, "size": s}, err)
}

// likePost 点赞讨论。
func (a teachingAPI) likePost(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.LikePost(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// pinPost 设置讨论置顶。
func (a teachingAPI) pinPost(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.PinPost(c.Request.Context(), id, true)
	httpx.Write(c, out, err)
}

// deletePost 删除讨论帖。
func (a teachingAPI) deletePost(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.DeletePost(c.Request.Context(), id))
}

// createAnnouncement 创建课程公告。
func (a teachingAPI) createAnnouncement(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AnnouncementRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingDiscussionInvalid) {
		return
	}
	out, err := a.svc.CreateAnnouncement(c.Request.Context(), courseID, req)
	httpx.Write(c, out, err)
}

// listAnnouncements 查询课程公告。
func (a teachingAPI) listAnnouncements(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ListAnnouncements(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// pinAnnouncement 设置公告置顶。
func (a teachingAPI) pinAnnouncement(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.PinAnnouncement(c.Request.Context(), id, true)
	httpx.Write(c, out, err)
}

// reviewCourse 绑定课程评价请求。
func (a teachingAPI) reviewCourse(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ReviewRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingDiscussionInvalid) {
		return
	}
	out, err := a.svc.ReviewCourse(c.Request.Context(), courseID, req)
	httpx.Write(c, out, err)
}

// setGradeWeights 保存课程成绩权重。
func (a teachingAPI) setGradeWeights(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req GradeWeightRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingGradeWeightInvalid) {
		return
	}
	out, err := a.svc.SetGradeWeights(c.Request.Context(), courseID, req)
	httpx.Write(c, out, err)
}

// listGradeWeights 查询课程成绩权重。
func (a teachingAPI) listGradeWeights(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ListGradeWeights(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// computeGrades 触发单课程成绩重算。
func (a teachingAPI) computeGrades(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ComputeCourseGrades(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// listGrades 查询单课程成绩。
func (a teachingAPI) listGrades(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ListGrades(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// overrideGrade 绑定教师手动调分请求。
func (a teachingAPI) overrideGrade(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	studentID, ok := httpx.PathID(c, "sid")
	if !ok {
		return
	}
	var req OverrideGradeRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTeachingGradeInvalid) {
		return
	}
	out, err := a.svc.OverrideGrade(c.Request.Context(), courseID, studentID, req)
	httpx.Write(c, out, err)
}

// exportGrades 输出课程成绩 Excel 文件。
func (a teachingAPI) exportGrades(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ExportGrades(c.Request.Context(), id)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	data, err := base64.StdEncoding.DecodeString(out.DataBase64)
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrTeachingGradeExportFailed.WithCause(err))
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", out.FileName))
	c.Data(http.StatusOK, out.ContentType, data)
}

// internalStats 读取租户级教学统计。
func (a teachingAPI) internalStats(c *gin.Context) {
	tenantID, ok := httpx.QueryInt(c, "tenant_id", httpx.QueryIntRule{Min: 1})
	if !ok {
		return
	}
	out, err := a.svc.Stats(c.Request.Context(), tenantID)
	httpx.Write(c, out, err)
}

// internalCourseGrades 读取单课程成绩供 M11 只读聚合。
func (a teachingAPI) internalCourseGrades(c *gin.Context) {
	courseID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := httpx.QueryInt(c, "tenant_id", httpx.QueryIntRule{Min: 1})
	if !ok {
		return
	}
	out, err := a.svc.ListCourseGrades(c.Request.Context(), tenantID, courseID)
	httpx.Write(c, out, err)
}

// teachingPage 统一解析教学模块分页参数。
func teachingPage(c *gin.Context) (int, int, bool) {
	page, ok := httpx.QueryInt(c, "page", httpx.QueryIntRule{Default: 1, Min: 1})
	if !ok {
		return 0, 0, false
	}
	size, ok := httpx.QueryInt(c, "size", httpx.QueryIntRule{Default: 20, Min: 1, Max: 100, HasMax: true})
	if !ok {
		return 0, 0, false
	}
	return int(page), int(size), true
}
