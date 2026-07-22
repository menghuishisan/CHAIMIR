// grade api 文件负责注册 M11 HTTP 路由、绑定请求和组合鉴权。
package grade

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册成绩中心 HTTP API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles contracts.IdentityService) error {
	if r == nil || svc == nil || authn == nil {
		return apperr.ErrHTTPServiceMissing
	}
	api := gradeAPI{svc: svc}
	g := r.Group("/api/v1/grade-center", authn.Middleware())
	all := g.Group("", auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	student := g.Group("", auth.RequireTenantAnyRole(roles, contracts.RoleStudent))
	studentAdmin := g.Group("", auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleSchoolAdmin))
	teacher := g.Group("", auth.RequireTenantAnyRole(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	admin := g.Group("", auth.RequireTenantAnyRole(roles, contracts.RoleSchoolAdmin))
	all.GET("/level-configs", api.listLevelConfigs)
	admin.POST("/level-configs", api.createLevelConfig)
	admin.PUT("/level-configs/:id", api.updateLevelConfig)
	all.GET("/semesters", api.listSemesters)
	admin.POST("/semesters", api.createSemester)
	teacher.POST("/reviews", api.submitReview)
	teacher.GET("/reviews/mine", api.listOwnReviews)
	admin.GET("/reviews", api.listReviews)
	admin.POST("/reviews/:id/approve", api.approveReview)
	admin.POST("/reviews/:id/reject", api.rejectReview)
	admin.POST("/reviews/:id/unlock", api.unlockReview)
	all.GET("/students/:id/grades", api.studentGrades)
	all.GET("/students/:id/gpa", api.studentGPA)
	admin.POST("/students/:id/recompute", api.recomputeStudentGrade)
	student.POST("/appeals", api.createAppeal)
	teacher.GET("/appeals", api.listAppeals)
	teacher.POST("/appeals/:id/accept", api.acceptAppeal)
	teacher.POST("/appeals/:id/reject", api.rejectAppeal)
	admin.GET("/warning-rules", api.getWarningRules)
	admin.PUT("/warning-rules", api.updateWarningRules)
	studentAdmin.GET("/warnings", api.listWarnings)
	student.POST("/warnings/:id/ack", api.ackWarning)
	admin.POST("/warnings/scan", api.scanWarnings)
	all.POST("/transcripts", api.generateTranscript)
	admin.POST("/transcripts/batch", api.generateTranscriptBatch)
	all.GET("/transcripts/:id", api.downloadTranscript)
	return nil
}

// listOwnReviews 查询当前教师本人提交的成绩审核记录。
func (a gradeAPI) listOwnReviews(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListOwnReviews(c.Request.Context(), int16(status), page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

type gradeAPI struct{ svc *Service }

// createLevelConfig 绑定等级映射配置创建请求。
func (a gradeAPI) createLevelConfig(c *gin.Context) {
	var req LevelConfigRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeConfigInvalid) {
		return
	}
	out1, err := a.svc.CreateLevelConfig(c.Request.Context(), req)
	httpx.Write(c, out1, err)
}

// listLevelConfigs 查询当前租户可用的等级映射配置。
func (a gradeAPI) listLevelConfigs(c *gin.Context) {
	out, err := a.svc.ListLevelConfigs(c.Request.Context())
	httpx.Write(c, out, err)
}

// updateLevelConfig 绑定等级映射配置更新请求。
func (a gradeAPI) updateLevelConfig(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req LevelConfigRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeConfigInvalid) {
		return
	}
	out2, err := a.svc.UpdateLevelConfig(c.Request.Context(), id, req)
	httpx.Write(c, out2, err)
}

// createSemester 绑定学期创建请求。
func (a gradeAPI) createSemester(c *gin.Context) {
	var req SemesterRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeConfigInvalid) {
		return
	}
	out3, err := a.svc.CreateSemester(c.Request.Context(), req)
	httpx.Write(c, out3, err)
}

// listSemesters 查询学期列表。
func (a gradeAPI) listSemesters(c *gin.Context) {
	out, err := a.svc.ListSemesters(c.Request.Context())
	httpx.Write(c, out, err)
}

// submitReview 绑定课程成绩审核提交请求。
func (a gradeAPI) submitReview(c *gin.Context) {
	var req ReviewRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeReviewInvalid) {
		return
	}
	out4, err := a.svc.SubmitReview(c.Request.Context(), req)
	httpx.Write(c, out4, err)
}

// listReviews 查询成绩审核列表。
func (a gradeAPI) listReviews(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return
	}
	out5, total, p, s, err := a.svc.ListReviews(c.Request.Context(), int16(status), page, size)
	httpx.WritePage(c, out5, total, p, s, err)
}

// approveReview 绑定审核通过操作。
func (a gradeAPI) approveReview(c *gin.Context) { a.reviewDecision(c, a.svc.ApproveReview) }

// rejectReview 绑定审核驳回操作。
func (a gradeAPI) rejectReview(c *gin.Context) { a.reviewDecision(c, a.svc.RejectReview) }

// unlockReview 绑定审核解锁操作。
func (a gradeAPI) unlockReview(c *gin.Context) { a.reviewDecision(c, a.svc.UnlockReview) }

// reviewDecision 复用审核决策请求绑定和响应封装。
func (a gradeAPI) reviewDecision(c *gin.Context, fn func(context.Context, int64, ReviewDecisionRequest) (ReviewDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ReviewDecisionRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeReviewInvalid) {
		return
	}
	out6, err := fn(c.Request.Context(), id, req)
	httpx.Write(c, out6, err)
}

// studentGrades 查询学生课程成绩明细。
func (a gradeAPI) studentGrades(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	semesterID, ok := httpx.QueryInt(c, "semester", httpx.QueryIntRule{Default: 0, Min: 0})
	if !ok {
		return
	}
	out, err := a.svc.StudentGrades(c.Request.Context(), id, semesterID)
	httpx.Write(c, out, err)
}

// studentGPA 查询学生 GPA 摘要。
func (a gradeAPI) studentGPA(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.StudentGPA(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// recomputeStudentGrade 绑定学生成绩重算请求。
func (a gradeAPI) recomputeStudentGrade(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req RecomputeRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeAggregationFailed) {
		return
	}
	out, err := a.svc.RecomputeStudentGrade(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// createAppeal 绑定学生成绩申诉创建请求。
func (a gradeAPI) createAppeal(c *gin.Context) {
	var req AppealRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeAppealInvalid) {
		return
	}
	out8, err := a.svc.CreateAppeal(c.Request.Context(), req)
	httpx.Write(c, out8, err)
}

// listAppeals 查询成绩申诉列表。
func (a gradeAPI) listAppeals(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 4, HasMax: true})
	if !ok {
		return
	}
	out9, total, p, s, err := a.svc.ListAppeals(c.Request.Context(), int16(status), page, size)
	httpx.WritePage(c, out9, total, p, s, err)
}

// acceptAppeal 绑定申诉受理操作。
func (a gradeAPI) acceptAppeal(c *gin.Context) { a.appealDecision(c, a.svc.AcceptAppeal) }

// rejectAppeal 绑定申诉驳回操作。
func (a gradeAPI) rejectAppeal(c *gin.Context) { a.appealDecision(c, a.svc.RejectAppeal) }

// appealDecision 复用申诉处理请求绑定和响应封装。
func (a gradeAPI) appealDecision(c *gin.Context, fn func(context.Context, int64, AppealDecisionRequest) (AppealDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AppealDecisionRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeAppealInvalid) {
		return
	}
	out10, err := fn(c.Request.Context(), id, req)
	httpx.Write(c, out10, err)
}

// getWarningRules 查询学业预警规则。
func (a gradeAPI) getWarningRules(c *gin.Context) {
	out, err := a.svc.GetWarningRules(c.Request.Context())
	httpx.Write(c, out, err)
}

// updateWarningRules 绑定学业预警规则更新请求。
func (a gradeAPI) updateWarningRules(c *gin.Context) {
	var req WarningRules
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeConfigInvalid) {
		return
	}
	out, err := a.svc.UpdateWarningRules(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// listWarnings 查询当前用户可见的学业预警。
func (a gradeAPI) listWarnings(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	studentID, ok := httpx.QueryInt(c, "student_id", httpx.QueryIntRule{Default: 0, Min: 0})
	if !ok {
		return
	}
	out11, total, p, s, err := a.svc.ListWarnings(c.Request.Context(), studentID, page, size)
	httpx.WritePage(c, out11, total, p, s, err)
}

// ackWarning 绑定学生确认预警操作。
func (a gradeAPI) ackWarning(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out12, err := a.svc.AckWarning(c.Request.Context(), id)
		httpx.Write(c, out12, err)
	}
}

// scanWarnings 绑定学业预警扫描请求。
func (a gradeAPI) scanWarnings(c *gin.Context) {
	var req WarningScanRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeWarningInvalid) {
		return
	}
	out, err := a.svc.ScanWarnings(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// generateTranscript 绑定单人成绩单生成请求。
func (a gradeAPI) generateTranscript(c *gin.Context) {
	var req TranscriptRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeTranscriptFailed) {
		return
	}
	out13, err := a.svc.GenerateTranscript(c.Request.Context(), req)
	httpx.Write(c, out13, err)
}

// generateTranscriptBatch 绑定批量成绩单生成请求。
func (a gradeAPI) generateTranscriptBatch(c *gin.Context) {
	var req TranscriptBatchRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeTranscriptFailed) {
		return
	}
	out, err := a.svc.GenerateTranscriptBatch(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// downloadTranscript 创建成绩单下载授权。
func (a gradeAPI) downloadTranscript(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.DownloadTranscript(c.Request.Context(), id)
	httpx.Write(c, out, err)
}
