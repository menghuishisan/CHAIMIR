// M6 服务公共 helper:加载实体、权限校验、邀请码和 DTO 批量转换。
package teaching

import (
	"context"
	"errors"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// updateCourseStatus 执行课程状态机流转。
func (s *Service) updateCourseStatus(ctx context.Context, courseID int64, status int16) (CourseDTO, error) {
	current, err := s.loadCourse(ctx, courseID)
	if err != nil {
		return CourseDTO{}, err
	}
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return CourseDTO{}, err
	}
	if err := validateCourseTransition(current.Status, status); err != nil {
		return CourseDTO{}, err
	}
	row, err := s.repo.updateCourseStatusIfAllowed(ctx, courseID, status)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return CourseDTO{}, ae
		}
		return CourseDTO{}, apperr.ErrCourseInvalidState.WithCause(err)
	}
	id, _ := tenantFromContext(ctx)
	if err := s.writeAudit(ctx, id.TenantID, auditActionCourseStatus, auditTargetCourse, courseID, map[string]any{"status": status}); err != nil {
		return CourseDTO{}, err
	}
	return row, nil
}

// updateCourseVisibility 执行课程共享状态变更。
func (s *Service) updateCourseVisibility(ctx context.Context, courseID int64, visibility int16) (CourseDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return CourseDTO{}, err
	}
	row, err := s.repo.updateCourseVisibility(ctx, courseID, visibility)
	if err != nil {
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	return row, nil
}

// loadCourse 读取课程访问投影并转换未命中错误。
func (s *Service) loadCourse(ctx context.Context, courseID int64) (CourseAccessSnapshot, error) {
	row, err := s.repo.getCourse(ctx, courseID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return CourseAccessSnapshot{}, ae
		}
		return CourseAccessSnapshot{}, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return row, nil
}

// loadChapter 读取章节位置投影。
func (s *Service) loadChapter(ctx context.Context, chapterID int64) (ChapterLocation, error) {
	row, err := s.repo.getChapter(ctx, chapterID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ChapterLocation{}, ae
		}
		return ChapterLocation{}, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return row, nil
}

// loadLesson 读取课时内容投影。
func (s *Service) loadLesson(ctx context.Context, lessonID int64) (LessonContentSnapshot, error) {
	row, err := s.repo.getLesson(ctx, lessonID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return LessonContentSnapshot{}, ae
		}
		return LessonContentSnapshot{}, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return row, nil
}

// loadAssignment 读取作业策略投影。
func (s *Service) loadAssignment(ctx context.Context, assignmentID int64) (AssignmentPolicySnapshot, error) {
	row, err := s.repo.getAssignment(ctx, assignmentID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return AssignmentPolicySnapshot{}, ae
		}
		return AssignmentPolicySnapshot{}, apperr.ErrAssignmentQueryFailed.WithCause(err)
	}
	return row, nil
}

// loadSubmission 读取提交评分投影。
func (s *Service) loadSubmission(ctx context.Context, submissionID int64) (SubmissionScoreSnapshot, error) {
	row, err := s.repo.getSubmission(ctx, submissionID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return SubmissionScoreSnapshot{}, ae
		}
		return SubmissionScoreSnapshot{}, apperr.ErrSubmissionQueryFailed.WithCause(err)
	}
	return row, nil
}

// ensureTeacherOfCourse 校验当前账号是课程教师或平台上下文。
func (s *Service) ensureTeacherOfCourse(ctx context.Context, courseID int64) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return nil
	}
	course, err := s.loadCourse(ctx, courseID)
	if err != nil {
		return err
	}
	if course.TeacherID != id.AccountID {
		return apperr.ErrCourseForbidden
	}
	return nil
}

// ensureTeacherRole 校验当前账号具备教师侧课程管理角色。
func (s *Service) ensureTeacherRole(ctx context.Context) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return nil
	}
	if s.identity == nil {
		return apperr.ErrCourseForbidden
	}
	account, err := s.identity.GetAccount(ctx, id.AccountID)
	if err != nil {
		return apperr.ErrCourseForbidden.WithCause(err)
	}
	if !contracts.HasAnyRole(account.Roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin) {
		return apperr.ErrCourseForbidden
	}
	return nil
}

// ensureStudentRole 使用服务端身份契约确认当前账号具备学生角色。
func (s *Service) ensureStudentRole(ctx context.Context, forbidden *apperr.Error) (int64, int64, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return 0, 0, apperr.ErrUnauthorized
	}
	if s.identity == nil {
		return 0, 0, forbidden.WithCause(errors.New("identity contract unavailable"))
	}
	account, err := s.identity.GetAccount(ctx, id.AccountID)
	if err != nil {
		return 0, 0, forbidden.WithCause(err)
	}
	if !contracts.HasAnyRole(account.Roles, contracts.RoleStudent) {
		return 0, 0, forbidden
	}
	return id.TenantID, id.AccountID, nil
}

// ensureStudentCourseMember 校验当前学生已加入课程;用于调用下游服务前阻断越权副作用。
func (s *Service) ensureStudentCourseMember(ctx context.Context, courseID int64, forbidden *apperr.Error) (int64, int64, error) {
	tenantID, accountID, err := s.ensureStudentRole(ctx, forbidden)
	if err != nil {
		return 0, 0, err
	}
	if err := s.repo.ensureCourseMember(ctx, courseID, accountID, forbidden); err != nil {
		if ae, ok := apperr.As(err); ok {
			return 0, 0, ae
		}
		return 0, 0, forbidden.WithCause(err)
	}
	return tenantID, accountID, nil
}

// ensureAccountStudent 校验教师批量添加的目标账号具备学生角色。
func (s *Service) ensureAccountStudent(ctx context.Context, accountID int64) error {
	if s.identity == nil {
		return apperr.ErrCourseMemberInvalid
	}
	ok, err := s.identity.HasRole(ctx, accountID, contracts.RoleStudent)
	if err != nil {
		return apperr.ErrCourseMemberInvalid.WithCause(err)
	}
	if !ok {
		return apperr.ErrCourseMemberInvalid
	}
	return nil
}

// ensureCourseAccessible 校验当前账号可访问课程。
func (s *Service) ensureCourseAccessible(ctx context.Context, courseID int64) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	course, err := s.loadCourse(ctx, courseID)
	if err != nil {
		return err
	}
	if canAccessCourseContent(id.IsPlatform, course.TeacherID, id.AccountID, course.Visibility, false) {
		return nil
	}
	member, err := s.repo.isCourseMember(ctx, courseID, id.AccountID)
	if err != nil {
		return apperr.ErrCourseForbidden.WithCause(err)
	}
	if canAccessCourseContent(id.IsPlatform, course.TeacherID, id.AccountID, course.Visibility, member) {
		return nil
	}
	return apperr.ErrCourseForbidden
}

// canAccessCourseContent 判断是否可访问课程学习内容;共享课程库只用于浏览/克隆,不授予学习内容访问权。
func canAccessCourseContent(isPlatform bool, teacherID, accountID int64, _ int16, isMember bool) bool {
	if isPlatform {
		return true
	}
	if teacherID == accountID {
		return true
	}
	return isMember
}

// validateCourseRequest 校验课程基本字段。
func validateCourseRequest(req CourseRequest) error {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Semester) == "" || req.Type <= 0 || req.Difficulty <= 0 || req.Credits <= 0 {
		return apperr.ErrCourseInvalid
	}
	return nil
}

// listGradesInTenant 查询课程成绩并转换 DTO。
func (s *Service) listGradesInTenant(ctx context.Context, courseID int64, page, size int) ([]CourseGradeDTO, error) {
	page, size = pagex.Normalize(page, size)
	rows, err := s.repo.listCourseGradesPage(ctx, courseID, size, (page-1)*size)
	if err != nil {
		return nil, apperr.ErrGradeQueryFailed.WithCause(err)
	}
	return rows, nil
}

// upsertOverrideGrade 写入覆盖成绩,保留已有 auto_total 并返回审计投影。
func (s *Service) upsertOverrideGrade(ctx context.Context, tenantID, courseID, studentID int64, score float64) (CourseGradeSnapshot, error) {
	row, err := s.repo.upsertOverrideCourseGrade(ctx, tenantID, s.idgen.Generate(), courseID, studentID, score)
	if err != nil {
		return CourseGradeSnapshot{}, apperr.ErrGradeInvalid.WithCause(err)
	}
	return row, nil
}

// getCourseGradeSnapshot 读取调分前成绩快照,没有成绩时返回空快照用于审计。
func (s *Service) getCourseGradeSnapshot(ctx context.Context, tenantID, courseID, studentID int64) (map[string]any, error) {
	row, found, err := s.repo.getCourseGradeOptional(ctx, tenantID, courseID, studentID)
	if err != nil {
		return nil, apperr.ErrGradeQueryFailed.WithCause(err)
	}
	if !found {
		return map[string]any{}, nil
	}
	return gradeAuditSnapshot(row), nil
}

// newInviteCode 生成不可预测的课程邀请码,避免加入凭证可由时间种子推测。
func (s *Service) newInviteCode() (string, error) {
	return crypto.RandomToken(8)
}
