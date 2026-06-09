// M11 HTTP 接口层:注册成绩中心等级、学期、审核、GPA、申诉、预警和成绩单路由。
package grade

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// gradeAPIService 是 M11 API 依赖的服务能力集合。
type gradeAPIService interface {
	ListLevelConfigs(context.Context) ([]LevelConfigDTO, error)
	CreateLevelConfig(context.Context, LevelConfigRequest) (LevelConfigDTO, error)
	UpdateLevelConfig(context.Context, int64, LevelConfigRequest) (LevelConfigDTO, error)
	ListSemesters(context.Context) ([]SemesterDTO, error)
	CreateSemester(context.Context, SemesterRequest) (SemesterDTO, error)
	WarningRules(context.Context) (WarningRuleDTO, error)
	UpdateWarningRules(context.Context, WarningRuleDTO) (WarningRuleDTO, error)
	SubmitReview(context.Context, ReviewCreateRequest) (ReviewDTO, error)
	ListReviews(context.Context, int16, int, int) ([]ReviewDTO, int64, error)
	CourseLockStatus(context.Context, int64) (ReviewDTO, error)
	ApproveReview(context.Context, int64, ReviewDecisionRequest) (ReviewDTO, error)
	RejectReview(context.Context, int64, ReviewDecisionRequest) (ReviewDTO, error)
	UnlockReview(context.Context, int64, ReviewDecisionRequest) (ReviewDTO, error)
	StudentGrades(context.Context, int64, int64) (StudentGradesDTO, error)
	StudentGPA(context.Context, int64) ([]SemesterGradeDTO, error)
	RecomputeStudent(context.Context, int64, RecomputeRequest) (SemesterGradeDTO, error)
	CreateAppeal(context.Context, AppealCreateRequest) (AppealDTO, error)
	AcceptAppeal(context.Context, int64, AppealHandleRequest) (AppealDTO, error)
	RejectAppeal(context.Context, int64, AppealHandleRequest) (AppealDTO, error)
	ListAppeals(context.Context, int16, int, int) ([]AppealDTO, int64, error)
	ScanWarnings(context.Context, WarningScanRequest) ([]WarningDTO, error)
	ListWarnings(context.Context, int64, int64, int16, int, int) ([]WarningDTO, int64, error)
	AcknowledgeWarning(context.Context, int64) (WarningDTO, error)
	GenerateTranscript(context.Context, TranscriptRequest) (TranscriptDTO, error)
	GetTranscript(context.Context, int64) (TranscriptDTO, error)
	DownloadTranscript(context.Context, int64) (TranscriptDTO, io.ReadCloser, error)
	BatchGenerateTranscripts(context.Context, TranscriptBatchRequest) ([]TranscriptDTO, error)
}

// API 是 M11 的 HTTP 处理器。
type API struct {
	svc      gradeAPIService
	authMgr  *auth.Manager
	identity contracts.IdentityService
}

// NewAPI 构造 M11 API。
func NewAPI(svc gradeAPIService, authMgr *auth.Manager, identity contracts.IdentityService) *API {
	return &API{svc: svc, authMgr: authMgr, identity: identity}
}

// Register 注册 M11 路由,所有成绩中心路径均需登录。
func (a *API) Register(rg *gin.RouterGroup) {
	g := rg.Group("/grade-center", a.authMgr.Middleware())
	{
		admin := g.Group("", a.requireSchoolAdmin())
		admin.GET("/level-configs", a.listLevelConfigs)
		admin.POST("/level-configs", a.createLevelConfig)
		admin.PUT("/level-configs/:id", a.updateLevelConfig)
		admin.GET("/semesters", a.listSemesters)
		admin.POST("/semesters", a.createSemester)
		admin.GET("/warning-rules", a.warningRules)
		admin.PUT("/warning-rules", a.updateWarningRules)
		admin.GET("/reviews", a.listReviews)
		admin.POST("/reviews/:id/approve", a.approveReview)
		admin.POST("/reviews/:id/reject", a.rejectReview)
		admin.POST("/reviews/:id/unlock", a.unlockReview)
		admin.POST("/transcripts/batch", a.batchGenerateTranscripts)

		teacher := g.Group("", a.requireTeacherOrAdmin())
		teacher.POST("/reviews", a.submitReview)
		teacher.GET("/appeals", a.listAppeals)
		teacher.POST("/appeals/:id/accept", a.acceptAppeal)
		teacher.POST("/appeals/:id/reject", a.rejectAppeal)

		g.GET("/students/:id/grades", a.studentGrades)
		g.GET("/students/:id/gpa", a.studentGPA)
		g.POST("/appeals", a.createAppeal)
		g.GET("/warnings", a.listWarnings)
		g.POST("/warnings/:id/ack", a.acknowledgeWarning)
		g.POST("/transcripts", a.generateTranscript)
		g.GET("/transcripts/:id", a.getTranscript)
	}
	recompute := rg.Group("/grade-center", a.requireSchoolAdminOrService())
	{
		recompute.POST("/students/:id/recompute", a.recomputeStudent)
	}
	internal := rg.Group("/grade-center", a.authMgr.ServiceMiddleware())
	{
		internal.GET("/courses/:course_id/lock-status", a.courseLockStatus)
		internal.POST("/warnings/scan", a.scanWarnings)
	}
}

// requireSchoolAdminOrService 允许学校管理员 JWT 或内部服务 HMAC 触发文档标注的重算入口。
func (a *API) requireSchoolAdminOrService() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(c.GetHeader(auth.ServiceNameHeader)) != "" || strings.TrimSpace(c.GetHeader(auth.ServiceSignatureHeader)) != "" {
			a.authMgr.ServiceMiddleware()(c)
			return
		}
		a.authMgr.Middleware()(c)
		if c.IsAborted() {
			return
		}
		a.requireSchoolAdmin()(c)
	}
}

// requireSchoolAdmin 要求学校管理员角色。
func (a *API) requireSchoolAdmin() gin.HandlerFunc {
	return auth.RequireTenantAnyRole(a.identity, contracts.RoleSchoolAdmin)
}

// requireTeacherOrAdmin 要求教师或学校管理员角色。
func (a *API) requireTeacherOrAdmin() gin.HandlerFunc {
	return auth.RequireTenantAnyRole(a.identity, contracts.RoleTeacher, contracts.RoleSchoolAdmin)
}

// listLevelConfigs 查询当前租户等级映射配置,供 GPA 聚合和成绩展示复用。
func (a *API) listLevelConfigs(c *gin.Context) {
	out, err := a.svc.ListLevelConfigs(c.Request.Context())
	httpx.Write(c, out, err)
}

// createLevelConfig 创建等级映射配置,请求体校验后交由服务层维护默认规则。
func (a *API) createLevelConfig(c *gin.Context) {
	var req LevelConfigRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeConfigInvalid) {
		out, err := a.svc.CreateLevelConfig(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// updateLevelConfig 更新等级映射配置,路径 ID 和版本内容在服务层统一校验。
func (a *API) updateLevelConfig(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req LevelConfigRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeConfigInvalid) {
		out, err := a.svc.UpdateLevelConfig(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// listSemesters 查询学期列表,作为成绩聚合和预警扫描的时间范围来源。
func (a *API) listSemesters(c *gin.Context) {
	out, err := a.svc.ListSemesters(c.Request.Context())
	httpx.Write(c, out, err)
}

// createSemester 创建学期边界,服务层负责校验日期区间和当前学期唯一性。
func (a *API) createSemester(c *gin.Context) {
	var req SemesterRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeSemesterInvalid) {
		out, err := a.svc.CreateSemester(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// warningRules 查询学业预警规则。
func (a *API) warningRules(c *gin.Context) {
	out, err := a.svc.WarningRules(c.Request.Context())
	httpx.Write(c, out, err)
}

// updateWarningRules 更新学业预警规则。
func (a *API) updateWarningRules(c *gin.Context) {
	var req WarningRuleDTO
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeWarningInvalid) {
		out, err := a.svc.UpdateWarningRules(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// submitReview 提交课程成绩审核申请,M11 只记录审核流程不重算单课程成绩。
func (a *API) submitReview(c *gin.Context) {
	var req ReviewCreateRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeReviewInvalid) {
		out, err := a.svc.SubmitReview(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// listReviews 分页查询成绩审核列表,按状态过滤供教师和管理员处理。
func (a *API) listReviews(c *gin.Context) {
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	items, total, err := a.svc.ListReviews(c.Request.Context(), int16(httpx.Int(c.Query("status"))), page, size)
	httpx.WritePage(c, items, total, page, size, err)
}

// courseLockStatus 查询课程成绩锁定状态。
func (a *API) courseLockStatus(c *gin.Context) {
	id, ok := httpx.PathID(c, "course_id")
	if ok {
		out, err := a.svc.CourseLockStatus(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// approveReview 审核通过课程成绩并锁定,防止后续单课程成绩被无审计修改。
func (a *API) approveReview(c *gin.Context) {
	a.reviewDecision(c, a.svc.ApproveReview)
}

// rejectReview 驳回课程成绩审核,驳回原因通过统一请求体进入服务层。
func (a *API) rejectReview(c *gin.Context) {
	a.reviewDecision(c, a.svc.RejectReview)
}

// unlockReview 解锁已审核成绩,仅受控角色可触发并写入审计。
func (a *API) unlockReview(c *gin.Context) {
	a.reviewDecision(c, a.svc.UnlockReview)
}

// reviewDecision 处理审核类动作的公共绑定。
func (a *API) reviewDecision(c *gin.Context, fn func(context.Context, int64, ReviewDecisionRequest) (ReviewDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ReviewDecisionRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeReviewInvalid) {
		out, err := fn(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// studentGrades 查询学生跨课程成绩明细,只读 M6 提供的单课程成绩结果。
func (a *API) studentGrades(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.StudentGrades(c.Request.Context(), id, ids.ParseOrZero(c.Query("semester")))
		httpx.Write(c, out, err)
	}
}

// studentGPA 查询学生 GPA 聚合结果,不在 HTTP 层重复成绩计算规则。
func (a *API) studentGPA(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.StudentGPA(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// recomputeStudent 触发学生 GPA 重算,由服务层从 M6 只读成绩并写入 M11 聚合表。
func (a *API) recomputeStudent(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req RecomputeRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeAggregateInvalid) {
		out, err := a.svc.RecomputeStudent(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// createAppeal 创建成绩申诉,学生身份和课程归属由服务端校验。
func (a *API) createAppeal(c *gin.Context) {
	var req AppealCreateRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeAppealInvalid) {
		out, err := a.svc.CreateAppeal(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// acceptAppeal 受理成绩申诉,处理意见经统一请求体进入服务层。
func (a *API) acceptAppeal(c *gin.Context) {
	a.appealDecision(c, a.svc.AcceptAppeal)
}

// rejectAppeal 驳回成绩申诉,保留处理人和处理时间用于追溯。
func (a *API) rejectAppeal(c *gin.Context) {
	a.appealDecision(c, a.svc.RejectAppeal)
}

// listAppeals 查询成绩申诉列表。
func (a *API) listAppeals(c *gin.Context) {
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	items, total, err := a.svc.ListAppeals(c.Request.Context(), int16(httpx.Int(c.Query("status"))), page, size)
	httpx.WritePage(c, items, total, page, size, err)
}

// appealDecision 处理申诉动作公共绑定。
func (a *API) appealDecision(c *gin.Context, fn func(context.Context, int64, AppealHandleRequest) (AppealDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AppealHandleRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeAppealInvalid) {
		out, err := fn(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// scanWarnings 执行学业预警扫描,按服务层规则生成或更新预警记录。
func (a *API) scanWarnings(c *gin.Context) {
	var req WarningScanRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeWarningInvalid) {
		out, err := a.svc.ScanWarnings(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// listWarnings 按学生、学期和状态筛选预警列表,返回分页结果。
func (a *API) listWarnings(c *gin.Context) {
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	items, total, err := a.svc.ListWarnings(c.Request.Context(), ids.ParseOrZero(c.Query("student_id")), ids.ParseOrZero(c.Query("semester_id")), int16(httpx.Int(c.Query("status"))), page, size)
	httpx.WritePage(c, items, total, page, size, err)
}

// acknowledgeWarning 标记预警已知悉,由服务层校验当前账号可处理该预警。
func (a *API) acknowledgeWarning(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.AcknowledgeWarning(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// generateTranscript 生成正式成绩单记录,PDF 内容和验证码由服务端产生。
func (a *API) generateTranscript(c *gin.Context) {
	var req TranscriptRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeTranscriptInvalid) {
		out, err := a.svc.GenerateTranscript(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// getTranscript 下载成绩单 PDF,以流式响应返回并显式记录关闭错误。
func (a *API) getTranscript(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		meta, reader, err := a.svc.DownloadTranscript(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, err)
			return
		}
		defer func() {
			if closeErr := reader.Close(); closeErr != nil {
				logging.ErrorContext(c.Request.Context(), "close transcript reader failed", closeErr.Error(), slog.String("transcript_id", meta.ID))
			}
		}()
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="transcript-%s.pdf"`, meta.ID))
		c.DataFromReader(http.StatusOK, -1, "application/pdf", reader, nil)
	}
}

// batchGenerateTranscripts 批量生成成绩单任务,避免客户端逐个拼接生成逻辑。
func (a *API) batchGenerateTranscripts(c *gin.Context) {
	var req TranscriptBatchRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrGradeTranscriptInvalid) {
		out, err := a.svc.BatchGenerateTranscripts(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}
