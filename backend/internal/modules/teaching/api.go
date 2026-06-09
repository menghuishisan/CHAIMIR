// M6 HTTP 接口层:注册 /api/v1/teaching 下课程、课时、作业、进度、互动与成绩接口。
package teaching

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// API 是 M6 的 HTTP 处理器。
type API struct {
	svc      *Service
	authMgr  *auth.Manager
	identity contracts.IdentityService
}

// NewAPI 构造 M6 HTTP 处理器,注入教学服务、鉴权管理器和身份只读契约。
func NewAPI(svc *Service, authMgr *auth.Manager, identity contracts.IdentityService) *API {
	return &API{svc: svc, authMgr: authMgr, identity: identity}
}

// Register 注册 M6 路由,教学用户路径走登录鉴权,聚合统计接口走服务鉴权。
func (a *API) Register(rg *gin.RouterGroup) {
	g := rg.Group("/teaching", a.authMgr.Middleware())
	{
		g.GET("/courses", a.listCourses)
		g.POST("/courses", a.createCourse)
		g.PATCH("/courses/:id", a.updateCourse)
		g.POST("/courses/:id/publish", a.publishCourse)
		g.POST("/courses/:id/end", a.endCourse)
		g.POST("/courses/:id/archive", a.archiveCourse)
		g.POST("/courses/:id/clone", a.cloneCourse)
		g.POST("/courses/:id/share", a.shareCourse)
		g.POST("/courses/:id/invite-code/refresh", a.refreshInviteCode)
		g.POST("/courses/join", a.joinCourse)
		g.GET("/courses/:id/members", a.listMembers)
		g.POST("/courses/:id/members/batch", a.addMembers)
		g.DELETE("/courses/:id/members/:sid", a.removeMember)
		g.GET("/courses/:id/chapters", a.listChapters)
		g.POST("/courses/:id/chapters", a.createChapter)
		g.PATCH("/courses/:id/chapters/:cid", a.updateChapter)
		g.DELETE("/courses/:id/chapters/:cid", a.deleteChapter)
		g.GET("/chapters/:id/lessons", a.listLessons)
		g.POST("/chapters/:id/lessons", a.createLesson)
		g.PATCH("/chapters/:id/lessons/:lid", a.updateLesson)
		g.DELETE("/chapters/:id/lessons/:lid", a.deleteLesson)
		g.POST("/lessons/:id/content", a.setLessonContent)
		g.GET("/courses/:id/outline", a.getOutline)
		g.GET("/lessons/:id", a.getLesson)
		g.POST("/lessons/:id/progress", a.upsertProgress)
		g.POST("/courses/:id/assignments", a.createAssignment)
		g.PATCH("/assignments/:id", a.updateAssignment)
		g.POST("/assignments/:id/publish", a.publishAssignment)
		g.GET("/assignments/:id", a.getAssignment)
		g.GET("/assignments/:id/draft", a.getDraft)
		g.POST("/assignments/:id/draft", a.saveDraft)
		g.POST("/assignments/:id/submit", a.submitAssignment)
		g.GET("/assignments/:id/submissions", a.listSubmissions)
		g.POST("/submissions/:id/grade", a.gradeSubmission)
		g.GET("/submissions/:id", a.getSubmission)
		g.GET("/courses/:id/posts", a.listPosts)
		g.POST("/courses/:id/posts", a.createPost)
		g.POST("/posts/:id/like", a.likePost)
		g.POST("/posts/:id/pin", a.pinPost)
		g.DELETE("/posts/:id", a.deletePost)
		g.GET("/courses/:id/announcements", a.listAnnouncements)
		g.POST("/courses/:id/announcements", a.createAnnouncement)
		g.POST("/announcements/:id/pin", a.pinAnnouncement)
		g.POST("/courses/:id/review", a.reviewCourse)
		g.GET("/courses/:id/progress-stats", a.progressStats)
		g.GET("/courses/:id/my-progress", a.myProgress)
		g.GET("/courses/:id/grade-weights", a.listGradeWeights)
		g.PUT("/courses/:id/grade-weights", a.setGradeWeights)
		g.POST("/courses/:id/grades/compute", a.computeGrades)
		g.GET("/courses/:id/grades", a.listGrades)
		g.GET("/courses/:id/grades/export", a.exportGrades)
		g.PATCH("/courses/:id/grades/:sid", a.overrideGrade)
	}
	internal := rg.Group("/teaching", a.authMgr.ServiceMiddleware())
	{
		internal.GET("/internal/stats", a.internalStats)
	}
}

// listCourses 按当前账号角色查询课程列表,支持教师和学生两种视角。
func (a *API) listCourses(c *gin.Context) {
	out, err := a.svc.ListCourses(c.Request.Context(), c.Query("role"), httpx.Int16(c.Query("status")), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.Write(c, out, err)
}

// createCourse 绑定课程创建请求,由服务层写入草稿并生成邀请码等服务端字段。
func (a *API) createCourse(c *gin.Context) {
	var req CourseRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrCourseInvalid) {
		out, err := a.svc.CreateCourse(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// updateCourse 更新课程基础信息,路径课程 ID 和请求体在 HTTP 边界分别校验。
func (a *API) updateCourse(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req CourseRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrCourseInvalid) {
		out, err := a.svc.UpdateCourse(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// publishCourse 触发课程发布状态流转,具体可发布条件由服务层校验。
func (a *API) publishCourse(c *gin.Context) { a.courseAction(c, a.svc.PublishCourse) }

// endCourse 触发课程结束状态流转,防止 handler 层复制状态机规则。
func (a *API) endCourse(c *gin.Context) { a.courseAction(c, a.svc.EndCourse) }

// archiveCourse 触发课程归档状态流转,统一复用课程状态动作入口。
func (a *API) archiveCourse(c *gin.Context) { a.courseAction(c, a.svc.ArchiveCourse) }

// cloneCourse 克隆课程结构和可复用内容引用,不在 HTTP 层复制课程组装逻辑。
func (a *API) cloneCourse(c *gin.Context) { a.courseAction(c, a.svc.CloneCourse) }

// shareCourse 将课程加入共享范围,共享规则和审计由服务层处理。
func (a *API) shareCourse(c *gin.Context) { a.courseAction(c, a.svc.ShareCourse) }

// refreshInviteCode 重新生成课程邀请码,旧码失效逻辑集中在服务层。
func (a *API) refreshInviteCode(c *gin.Context) { a.courseAction(c, a.svc.RefreshInviteCode) }

// joinCourse 按邀请码加入课程。
func (a *API) joinCourse(c *gin.Context) {
	var req JoinCourseRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrCourseJoinInvalid) {
		out, err := a.svc.JoinCourseByInvite(c.Request.Context(), req.InviteCode)
		httpx.Write(c, out, err)
	}
}

// listMembers 查询课程成员分页列表,用于教师管理选课学生。
func (a *API) listMembers(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ListMembers(c.Request.Context(), id, httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.Write(c, out, err)
}

// addMembers 绑定批量成员请求,由服务层校验课程教师权限和学生身份。
func (a *API) addMembers(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req MemberBatchRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrCourseMemberInvalid) {
		out, err := a.svc.AddMembers(c.Request.Context(), id, req.StudentIDs)
		httpx.Write(c, out, err)
	}
}

// removeMember 移除课程成员关系,路径中的课程和学生 ID 都必须有效。
func (a *API) removeMember(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	sid, ok := httpx.PathID(c, "sid")
	if !ok {
		return
	}
	err := a.svc.RemoveMember(c.Request.Context(), id, sid)
	httpx.Write(c, map[string]any{"removed": true}, err)
}

// listChapters 查询课程章节列表,只负责路径绑定和响应写回。
func (a *API) listChapters(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListChapters(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// createChapter 在指定课程下创建章节,排序和教师权限由服务层控制。
func (a *API) createChapter(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ChapterRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrCourseInvalid) {
		out, err := a.svc.CreateChapter(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// updateChapter 更新章节标题或顺序,同时校验章节归属课程。
func (a *API) updateChapter(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	cid, ok := httpx.PathID(c, "cid")
	if !ok {
		return
	}
	var req ChapterRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrCourseInvalid) {
		out, err := a.svc.UpdateChapter(c.Request.Context(), id, cid, req)
		httpx.Write(c, out, err)
	}
}

// deleteChapter 软删章节,由服务层保护包含课时的章节不被误删。
func (a *API) deleteChapter(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	cid, ok := httpx.PathID(c, "cid")
	if !ok {
		return
	}
	httpx.Write(c, map[string]any{"deleted": true}, a.svc.DeleteChapter(c.Request.Context(), id, cid))
}

// listLessons 查询章节课时列表,供课程目录和编辑页复用。
func (a *API) listLessons(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListLessons(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// createLesson 在章节下创建课时,内容引用后续通过专门接口绑定。
func (a *API) createLesson(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req LessonRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrCourseInvalid) {
		out, err := a.svc.CreateLesson(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// updateLesson 更新课时基础信息,章节归属和教师权限由服务层校验。
func (a *API) updateLesson(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	lid, ok := httpx.PathID(c, "lid")
	if !ok {
		return
	}
	var req LessonRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrCourseInvalid) {
		out, err := a.svc.UpdateLesson(c.Request.Context(), id, lid, req)
		httpx.Write(c, out, err)
	}
}

// deleteLesson 软删课时,保持学习进度等历史数据可追溯。
func (a *API) deleteLesson(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	lid, ok := httpx.PathID(c, "lid")
	if !ok {
		return
	}
	httpx.Write(c, map[string]any{"deleted": true}, a.svc.DeleteLesson(c.Request.Context(), id, lid))
}

// setLessonContent 绑定课时内容引用,按内容类型校验必要字段后交给服务层保存。
func (a *API) setLessonContent(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req LessonContentRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrCourseInvalid) {
		out, err := a.svc.SetLessonContent(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// getOutline 查询课程目录。
func (a *API) getOutline(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetCourseOutline(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// getLesson 查询课时。
func (a *API) getLesson(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetLesson(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// upsertProgress 上报或更新学生学习进度,服务端记录是跨设备权威状态。
func (a *API) upsertProgress(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ProgressRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrProgressInvalid) {
		out, err := a.svc.UpsertProgress(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// createAssignment 创建作业草稿并锁定内容版本引用,避免发布后题目漂移。
func (a *API) createAssignment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AssignmentRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAssignmentInvalid) {
		out, err := a.svc.CreateAssignment(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// updateAssignment 更新作业草稿配置,只允许服务层认可的状态修改。
func (a *API) updateAssignment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AssignmentRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAssignmentInvalid) {
		out, err := a.svc.UpdateAssignment(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// publishAssignment 发布作业并开启学生提交入口,状态机仍由服务层控制。
func (a *API) publishAssignment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.PublishAssignment(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// getAssignment 查询作业。
func (a *API) getAssignment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetAssignment(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// saveDraft 保存作答草稿。
func (a *API) saveDraft(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req DraftRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrSubmissionInvalid) {
		out, err := a.svc.SaveDraft(c.Request.Context(), id, req.Content)
		httpx.Write(c, out, err)
	}
}

// getDraft 读取当前学生的服务端作答草稿。
func (a *API) getDraft(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetDraft(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// submitAssignment 提交学生作业内容,包含迟交策略和自动判题出队逻辑。
func (a *API) submitAssignment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req SubmitRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrSubmissionInvalid) {
		out, err := a.svc.SubmitAssignment(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// listSubmissions 查询提交列表。
func (a *API) listSubmissions(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListSubmissions(c.Request.Context(), id, httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
		httpx.Write(c, out, err)
	}
}

// gradeSubmission 绑定教师批改请求,由服务层写入手动分和最终分。
func (a *API) gradeSubmission(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req GradeSubmissionRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeInvalid) {
		out, err := a.svc.GradeSubmission(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// getSubmission 查询提交。
func (a *API) getSubmission(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetSubmission(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// listPosts 查询课程讨论帖分页列表,用于课堂互动区展示。
func (a *API) listPosts(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListPosts(c.Request.Context(), id, httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
		httpx.Write(c, out, err)
	}
}

// createPost 创建课程讨论帖,作者身份来自服务端会话而非客户端参数。
func (a *API) createPost(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req PostRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrDiscussionInvalid) {
		out, err := a.svc.CreatePost(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// likePost 点赞讨论帖。
func (a *API) likePost(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.LikePost(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// pinPost 置顶讨论帖。
func (a *API) pinPost(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.PinPost(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// deletePost 删除讨论帖。
func (a *API) deletePost(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		httpx.Write(c, map[string]any{"deleted": true}, a.svc.DeletePost(c.Request.Context(), id))
	}
}

// listAnnouncements 查询公告。
func (a *API) listAnnouncements(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListAnnouncements(c.Request.Context(), id, httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
		httpx.Write(c, out, err)
	}
}

// createAnnouncement 创建公告。
func (a *API) createAnnouncement(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AnnouncementRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAnnouncementInvalid) {
		out, err := a.svc.CreateAnnouncement(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// pinAnnouncement 置顶公告。
func (a *API) pinAnnouncement(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.PinAnnouncement(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// reviewCourse 提交课程评价,服务层负责防止越权和重复评价。
func (a *API) reviewCourse(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ReviewRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrReviewInvalid) {
		out, err := a.svc.ReviewCourse(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// progressStats 查询课程进度统计。
func (a *API) progressStats(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ProgressStats(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// myProgress 查询我的课程进度。
func (a *API) myProgress(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.MyProgress(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// listGradeWeights 查询成绩权重。
func (a *API) listGradeWeights(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListGradeWeights(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// setGradeWeights 保存成绩权重。
func (a *API) setGradeWeights(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req []GradeWeightInput
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeWeightInvalid) {
		out, err := a.svc.SetGradeWeights(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// computeGrades 计算课程成绩。
func (a *API) computeGrades(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ComputeGrades(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// listGrades 查询课程成绩。
func (a *API) listGrades(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListGrades(c.Request.Context(), id, httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
		httpx.Write(c, out, err)
	}
}

// exportGrades 导出课程完整成绩;文件生成细节由服务层统一封装。
func (a *API) exportGrades(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	file, err := a.svc.ExportGrades(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	c.Data(200, file.ContentType, file.Content)
}

// overrideGrade 手动覆盖课程成绩。
func (a *API) overrideGrade(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	sid, ok := httpx.PathID(c, "sid")
	if !ok {
		return
	}
	var req GradeOverrideRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeInvalid) {
		out, err := a.svc.OverrideGrade(c.Request.Context(), id, sid, req)
		httpx.Write(c, out, err)
	}
}

// internalStats 返回教学内部统计。
func (a *API) internalStats(c *gin.Context) {
	tenantID, ok := ids.Parse(c.Query("tenant_id"))
	if !ok {
		response.Fail(c, apperr.ErrTeachingStatsQueryInvalid)
		return
	}
	out, err := a.svc.StatsDTO(c.Request.Context(), tenantID)
	httpx.Write(c, out, err)
}

// courseAction 执行只需课程 ID 的课程操作。
func (a *API) courseAction(c *gin.Context, fn func(context.Context, int64) (CourseDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := fn(c.Request.Context(), id)
	httpx.Write(c, out, err)
}
