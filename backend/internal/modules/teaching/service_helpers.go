// M6 服务公共 helper:加载实体、权限校验、邀请码和 DTO 批量转换。
package teaching

import (
	"context"
	"errors"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/timex"
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
	var row sqlcgen.Course
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		if status == CourseStatusPublished {
			publishable, err := q.EnsureCoursePublishable(ctx, courseID)
			if err != nil {
				return err
			}
			if !publishable {
				return apperr.ErrCourseInvalidState
			}
		}
		row, updateErr = q.UpdateCourseStatus(ctx, sqlcgen.UpdateCourseStatusParams{ID: courseID, Status: status})
		return updateErr
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return CourseDTO{}, ae
		}
		return CourseDTO{}, apperr.ErrCourseInvalidState.WithCause(err)
	}
	id, _ := tenantFromContext(ctx)
	if err := s.writeAudit(ctx, id.TenantID, auditActionCourseStatus, auditTargetCourse, courseID, map[string]any{"status": status}); err != nil {
		return CourseDTO{}, err
	}
	return courseDTOFromRow(row), nil
}

// updateCourseVisibility 执行课程共享状态变更。
func (s *Service) updateCourseVisibility(ctx context.Context, courseID int64, visibility int16) (CourseDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return CourseDTO{}, err
	}
	var row sqlcgen.Course
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateCourseVisibility(ctx, sqlcgen.UpdateCourseVisibilityParams{ID: courseID, Visibility: visibility})
		return err
	}); err != nil {
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	return courseDTOFromRow(row), nil
}

// cloneCourseStructure 复制课程章节与课时结构,不复制成员、提交、进度或成绩。
func (s *Service) cloneCourseStructure(ctx context.Context, q *sqlcgen.Queries, tenantID, sourceCourseID, targetCourseID int64) error {
	chapters, err := q.ListChaptersByCourse(ctx, sourceCourseID)
	if err != nil {
		return err
	}
	for _, chapter := range chapters {
		newChapter, err := q.CreateChapter(ctx, sqlcgen.CreateChapterParams{
			ID: s.idgen.Generate(), TenantID: tenantID, CourseID: targetCourseID, Title: chapter.Title, Sort: chapter.Sort,
		})
		if err != nil {
			return err
		}
		lessons, err := q.ListLessonsByChapter(ctx, chapter.ID)
		if err != nil {
			return err
		}
		for _, lesson := range lessons {
			if _, err := q.CreateLesson(ctx, sqlcgen.CreateLessonParams{
				ID: s.idgen.Generate(), TenantID: tenantID, ChapterID: newChapter.ID, Title: lesson.Title,
				ContentType: lesson.ContentType, ContentRef: lesson.ContentRef, Sort: lesson.Sort,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// loadCourse 读取课程并转换未命中错误。
func (s *Service) loadCourse(ctx context.Context, courseID int64) (sqlcgen.Course, error) {
	var row sqlcgen.Course
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetCourseByID(ctx, courseID)
		if db.IsNoRows(err) {
			return apperr.ErrCourseNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.Course{}, ae
		}
		return sqlcgen.Course{}, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return row, nil
}

// loadChapter 读取章节。
func (s *Service) loadChapter(ctx context.Context, chapterID int64) (sqlcgen.Chapter, error) {
	var row sqlcgen.Chapter
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetChapterByID(ctx, chapterID)
		if db.IsNoRows(err) {
			return apperr.ErrCourseNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.Chapter{}, ae
		}
		return sqlcgen.Chapter{}, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return row, nil
}

// loadLesson 读取课时。
func (s *Service) loadLesson(ctx context.Context, lessonID int64) (sqlcgen.Lesson, error) {
	var row sqlcgen.Lesson
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetLessonByID(ctx, lessonID)
		if db.IsNoRows(err) {
			return apperr.ErrCourseNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.Lesson{}, ae
		}
		return sqlcgen.Lesson{}, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return row, nil
}

// loadAssignment 读取作业。
func (s *Service) loadAssignment(ctx context.Context, assignmentID int64) (sqlcgen.Assignment, error) {
	var row sqlcgen.Assignment
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetAssignmentByID(ctx, assignmentID)
		if db.IsNoRows(err) {
			return apperr.ErrAssignmentNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.Assignment{}, ae
		}
		return sqlcgen.Assignment{}, apperr.ErrAssignmentQueryFailed.WithCause(err)
	}
	return row, nil
}

// loadSubmission 读取提交记录。
func (s *Service) loadSubmission(ctx context.Context, submissionID int64) (sqlcgen.Submission, error) {
	var row sqlcgen.Submission
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetSubmissionByID(ctx, submissionID)
		if db.IsNoRows(err) {
			return apperr.ErrSubmissionNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.Submission{}, ae
		}
		return sqlcgen.Submission{}, apperr.ErrSubmissionQueryFailed.WithCause(err)
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
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, memberErr := q.GetCourseMember(ctx, sqlcgen.GetCourseMemberParams{CourseID: courseID, StudentID: accountID})
		if db.IsNoRows(memberErr) {
			return forbidden
		}
		return memberErr
	}); err != nil {
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
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, memberErr := q.GetCourseMember(ctx, sqlcgen.GetCourseMemberParams{CourseID: courseID, StudentID: id.AccountID})
		if memberErr == nil && canAccessCourseContent(id.IsPlatform, course.TeacherID, id.AccountID, course.Visibility, true) {
			return nil
		}
		if db.IsNoRows(memberErr) {
			return apperr.ErrCourseForbidden
		}
		return memberErr
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrCourseForbidden.WithCause(err)
	}
	return nil
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
	var rows []sqlcgen.CourseGrade
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListCourseGrades(ctx, sqlcgen.ListCourseGradesParams{CourseID: courseID, LimitCount: int32(size), OffsetCount: int32((page - 1) * size)})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrGradeQueryFailed.WithCause(err)
	}
	out := make([]CourseGradeDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, gradeDTOFromRow(row))
	}
	return out, nil
}

// upsertOverrideGrade 写入覆盖成绩,保留已有 auto_total。
func (s *Service) upsertOverrideGrade(ctx context.Context, tenantID, courseID, studentID int64, score float64) (sqlcgen.CourseGrade, error) {
	override, err := pgNumeric(score)
	if err != nil {
		return sqlcgen.CourseGrade{}, apperr.ErrGradeInvalid.WithCause(err)
	}
	var row sqlcgen.CourseGrade
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		current, err := q.GetCourseGrade(ctx, sqlcgen.GetCourseGradeParams{CourseID: courseID, StudentID: studentID})
		autoTotal := override
		if err == nil {
			autoTotal = current.AutoTotal
		} else if !db.IsNoRows(err) {
			return err
		}
		row, err = q.UpsertCourseGrade(ctx, sqlcgen.UpsertCourseGradeParams{ID: s.idgen.Generate(), TenantID: tenantID, CourseID: courseID, StudentID: studentID, AutoTotal: autoTotal, OverrideTotal: override, IsOverridden: true})
		return err
	}); err != nil {
		return sqlcgen.CourseGrade{}, apperr.ErrGradeInvalid.WithCause(err)
	}
	return row, nil
}

// getCourseGradeSnapshot 读取调分前成绩快照,没有成绩时返回空快照用于审计。
func (s *Service) getCourseGradeSnapshot(ctx context.Context, tenantID, courseID, studentID int64) (map[string]any, error) {
	var snapshot map[string]any
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		row, err := q.GetCourseGrade(ctx, sqlcgen.GetCourseGradeParams{CourseID: courseID, StudentID: studentID})
		if db.IsNoRows(err) {
			snapshot = map[string]any{}
			return nil
		}
		if err != nil {
			return err
		}
		snapshot = gradeAuditSnapshot(row)
		return nil
	}); err != nil {
		return nil, apperr.ErrGradeQueryFailed.WithCause(err)
	}
	return snapshot, nil
}

// gradeAuditSnapshot 转换成绩行供审计记录 old/new 值。
func gradeAuditSnapshot(row sqlcgen.CourseGrade) map[string]any {
	dto := gradeDTOFromRow(row)
	return map[string]any{
		"auto_total":     dto.AutoTotal,
		"override_total": dto.OverrideTotal,
		"final_total":    dto.FinalTotal,
		"is_overridden":  dto.IsOverridden,
	}
}

// memberDTOFromRow 转换课程成员行。
func memberDTOFromRow(row sqlcgen.CourseMember) MemberDTO {
	return MemberDTO{ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), StudentID: ids.Format(row.StudentID), JoinMode: row.JoinMode, JoinedAt: timex.FromTimestamptz(row.JoinedAt)}
}

// memberDTOsFromRows 批量转换课程成员。
func memberDTOsFromRows(rows []sqlcgen.CourseMember) []MemberDTO {
	out := make([]MemberDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, memberDTOFromRow(row))
	}
	return out
}

// lessonDTOsFromRows 批量转换课时。
func lessonDTOsFromRows(rows []sqlcgen.Lesson) []LessonDTO {
	out := make([]LessonDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, lessonDTOFromRow(row))
	}
	return out
}

// submissionDTOsFromRows 批量转换提交。
func submissionDTOsFromRows(rows []sqlcgen.Submission) []SubmissionDTO {
	out := make([]SubmissionDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, submissionDTOFromRow(row))
	}
	return out
}

// postDTOFromRow 转换讨论帖行。
func postDTOFromRow(row sqlcgen.DiscussionPost) PostDTO {
	return PostDTO{ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), ParentID: optionalID(row.ParentID), AuthorID: ids.Format(row.AuthorID), Content: row.Content, IsPinned: row.IsPinned, LikeCount: row.LikeCount, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// postDTOsFromRows 批量转换讨论帖。
func postDTOsFromRows(rows []sqlcgen.DiscussionPost) []PostDTO {
	out := make([]PostDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, postDTOFromRow(row))
	}
	return out
}

// announcementDTOFromRow 转换公告行。
func announcementDTOFromRow(row sqlcgen.Announcement) AnnouncementDTO {
	return AnnouncementDTO{ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), Title: row.Title, Content: row.Content, IsPinned: row.IsPinned, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// announcementDTOsFromRows 批量转换公告。
func announcementDTOsFromRows(rows []sqlcgen.Announcement) []AnnouncementDTO {
	out := make([]AnnouncementDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, announcementDTOFromRow(row))
	}
	return out
}

// hasAutoGrading 判断作业是否包含自动判题项目。
func hasAutoGrading(items []sqlcgen.AssignmentItem) bool {
	for _, item := range items {
		if item.GradingMode == GradingModeAuto {
			return true
		}
	}
	return false
}

// firstAutoItem 返回第一个自动判题项目。
func firstAutoItem(items []sqlcgen.AssignmentItem) sqlcgen.AssignmentItem {
	for _, item := range items {
		if item.GradingMode == GradingModeAuto {
			return item
		}
	}
	return sqlcgen.AssignmentItem{}
}

// newInviteCode 生成不可预测的课程邀请码,避免加入凭证可由时间种子推测。
func (s *Service) newInviteCode() (string, error) {
	return crypto.RandomToken(8)
}

// mustOptionalID 解析可选 ID。
func mustOptionalID(v string) int64 {
	id, _ := ids.Parse(v)
	return id
}
