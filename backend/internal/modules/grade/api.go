// grade api 文件负责注册 M11 HTTP 路由、绑定请求和组合鉴权。
package grade

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册成绩中心 HTTP API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles auth.RoleChecker) error {
	if r == nil || svc == nil || authn == nil {
		return apperr.ErrInternal.WithMessage("grade routes 依赖不完整")
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

type gradeAPI struct{ svc *Service }

func (a gradeAPI) createLevelConfig(c *gin.Context) {
	var req LevelConfigRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeConfigInvalid) {
		return
	}
	out1, err := a.svc.CreateLevelConfig(c.Request.Context(), req)
	httpx.Write(c, out1, err)
}

func (a gradeAPI) listLevelConfigs(c *gin.Context) {
	out, err := a.svc.ListLevelConfigs(c.Request.Context())
	httpx.Write(c, out, err)
}

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

func (a gradeAPI) createSemester(c *gin.Context) {
	var req SemesterRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeConfigInvalid) {
		return
	}
	out3, err := a.svc.CreateSemester(c.Request.Context(), req)
	httpx.Write(c, out3, err)
}

func (a gradeAPI) listSemesters(c *gin.Context) {
	out, err := a.svc.ListSemesters(c.Request.Context())
	httpx.Write(c, out, err)
}

func (a gradeAPI) submitReview(c *gin.Context) {
	var req ReviewRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeReviewInvalid) {
		return
	}
	out4, err := a.svc.SubmitReview(c.Request.Context(), req)
	httpx.Write(c, out4, err)
}

func (a gradeAPI) listReviews(c *gin.Context) {
	page, size, ok := gradePage(c)
	if !ok {
		return
	}
	status, _ := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	out5, err := a.svc.ListReviews(c.Request.Context(), int16(status), page, size)
	httpx.Write(c, out5, err)
}

func (a gradeAPI) approveReview(c *gin.Context) { a.reviewDecision(c, a.svc.ApproveReview) }
func (a gradeAPI) rejectReview(c *gin.Context)  { a.reviewDecision(c, a.svc.RejectReview) }
func (a gradeAPI) unlockReview(c *gin.Context)  { a.reviewDecision(c, a.svc.UnlockReview) }

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

func (a gradeAPI) studentGPA(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.StudentGPA(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

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

func (a gradeAPI) createAppeal(c *gin.Context) {
	var req AppealRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeAppealInvalid) {
		return
	}
	out8, err := a.svc.CreateAppeal(c.Request.Context(), req)
	httpx.Write(c, out8, err)
}

func (a gradeAPI) listAppeals(c *gin.Context) {
	page, size, ok := gradePage(c)
	if !ok {
		return
	}
	status, _ := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 4, HasMax: true})
	out9, err := a.svc.ListAppeals(c.Request.Context(), int16(status), page, size)
	httpx.Write(c, out9, err)
}

func (a gradeAPI) acceptAppeal(c *gin.Context) { a.appealDecision(c, a.svc.AcceptAppeal) }
func (a gradeAPI) rejectAppeal(c *gin.Context) { a.appealDecision(c, a.svc.RejectAppeal) }

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

func (a gradeAPI) getWarningRules(c *gin.Context) {
	out, err := a.svc.GetWarningRules(c.Request.Context())
	httpx.Write(c, out, err)
}

func (a gradeAPI) updateWarningRules(c *gin.Context) {
	var req WarningRules
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeConfigInvalid) {
		return
	}
	out, err := a.svc.UpdateWarningRules(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

func (a gradeAPI) listWarnings(c *gin.Context) {
	page, size, ok := gradePage(c)
	if !ok {
		return
	}
	studentID, _ := httpx.QueryInt(c, "student_id", httpx.QueryIntRule{Default: 0, Min: 0})
	out11, err := a.svc.ListWarnings(c.Request.Context(), studentID, page, size)
	httpx.Write(c, out11, err)
}

func (a gradeAPI) ackWarning(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out12, err := a.svc.AckWarning(c.Request.Context(), id)
		httpx.Write(c, out12, err)
	}
}

func (a gradeAPI) scanWarnings(c *gin.Context) {
	var req WarningScanRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeWarningInvalid) {
		return
	}
	out, err := a.svc.ScanWarnings(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

func (a gradeAPI) generateTranscript(c *gin.Context) {
	var req TranscriptRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeTranscriptFailed) {
		return
	}
	out13, err := a.svc.GenerateTranscript(c.Request.Context(), req)
	httpx.Write(c, out13, err)
}

func (a gradeAPI) generateTranscriptBatch(c *gin.Context) {
	var req TranscriptBatchRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrGradeTranscriptFailed) {
		return
	}
	out, err := a.svc.GenerateTranscriptBatch(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

func (a gradeAPI) downloadTranscript(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	record, reader, err := a.svc.DownloadTranscript(c.Request.Context(), id)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"transcript-%d.pdf\"", record.ID))
	c.Status(http.StatusOK)
	c.Header("Content-Type", "application/pdf")
	if _, copyErr := io.Copy(c.Writer, reader); copyErr != nil {
		if reportErr := c.Error(apperr.ErrGradeTranscriptFailed.WithCause(copyErr)); reportErr != nil {
			logging.ErrorContext(c.Request.Context(), "记录成绩单下载错误失败", reportErr.Error())
		}
	}
	if closeErr := reader.Close(); closeErr != nil {
		logging.ErrorContext(c.Request.Context(), "关闭成绩单下载流失败", closeErr.Error())
	}
}

func gradePage(c *gin.Context) (int, int, bool) {
	p, ok := httpx.QueryInt(c, "page", httpx.QueryIntRule{Default: 1, Min: 1})
	if !ok {
		return 0, 0, false
	}
	s, ok := httpx.QueryInt(c, "size", httpx.QueryIntRule{Default: 20, Min: 1, Max: 100, HasMax: true})
	if !ok {
		return 0, 0, false
	}
	return int(p), int(s), true
}
