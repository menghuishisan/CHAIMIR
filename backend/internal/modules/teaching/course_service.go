// M6 课程与成员服务:课程生命周期、共享、邀请码与选课成员管理。
package teaching

import (
	"context"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
)

// ListCourses 查询教师或学生课程列表。
func (s *Service) ListCourses(ctx context.Context, role string, status int16, page, size int) ([]CourseDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	role, err := normalizeCourseListRole(role)
	if err != nil {
		return nil, err
	}
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.Course
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if role == contracts.RoleStudent {
			rows, err = q.ListStudentCourses(ctx, sqlcgen.ListStudentCoursesParams{StudentID: id.AccountID, Status: pgInt2Filter(status), LimitCount: int32(size), OffsetCount: int32((page - 1) * size)})
		} else {
			rows, err = q.ListTeacherCourses(ctx, sqlcgen.ListTeacherCoursesParams{TeacherID: id.AccountID, Status: pgInt2Filter(status), LimitCount: int32(size), OffsetCount: int32((page - 1) * size)})
		}
		return err
	}); err != nil {
		return nil, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return courseDTOsFromRows(rows), nil
}

// CreateCourse 创建草稿课程。
func (s *Service) CreateCourse(ctx context.Context, req CourseRequest) (CourseDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return CourseDTO{}, apperr.ErrUnauthorized
	}
	if err := s.ensureTeacherRole(ctx); err != nil {
		return CourseDTO{}, err
	}
	if err := validateCourseRequest(req); err != nil {
		return CourseDTO{}, err
	}
	schedule, err := jsonx.ObjectBytes(req.Schedule, apperr.ErrCourseInvalid)
	if err != nil {
		return CourseDTO{}, err
	}
	credits, err := pgNumeric(req.Credits)
	if err != nil {
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	courseID := s.idgen.Generate()
	inviteCode, err := s.newInviteCode()
	if err != nil {
		return CourseDTO{}, apperr.ErrCourseInviteFailed.WithCause(err)
	}
	var row sqlcgen.Course
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateCourse(ctx, sqlcgen.CreateCourseParams{
			ID: courseID, TenantID: id.TenantID, TeacherID: id.AccountID, Name: req.Name,
			Description: req.Description, Type: req.Type, Difficulty: req.Difficulty, CoverUrl: pgText(req.CoverURL),
			Semester: req.Semester, Credits: credits, Schedule: schedule, InviteCode: inviteCode,
			Status: CourseStatusDraft, Visibility: CourseVisibilityPrivate,
		})
		return createErr
	}); err != nil {
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionCourseCreate, auditTargetCourse, courseID, map[string]any{"name": req.Name}); err != nil {
		return CourseDTO{}, err
	}
	return courseDTOFromRow(row), nil
}

// UpdateCourse 编辑课程基础信息。
func (s *Service) UpdateCourse(ctx context.Context, courseID int64, req CourseRequest) (CourseDTO, error) {
	if err := validateCourseRequest(req); err != nil {
		return CourseDTO{}, err
	}
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return CourseDTO{}, err
	}
	schedule, err := jsonx.ObjectBytes(req.Schedule, apperr.ErrCourseInvalid)
	if err != nil {
		return CourseDTO{}, err
	}
	credits, err := pgNumeric(req.Credits)
	if err != nil {
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	var row sqlcgen.Course
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.UpdateCourse(ctx, sqlcgen.UpdateCourseParams{
			ID: courseID, Name: req.Name, Description: req.Description, Type: req.Type, Difficulty: req.Difficulty,
			CoverUrl: pgText(req.CoverURL), Semester: req.Semester, Credits: credits, Schedule: schedule,
		})
		if db.IsNoRows(updateErr) {
			return apperr.ErrCourseNotFound
		}
		return updateErr
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return CourseDTO{}, ae
		}
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	id, _ := tenantFromContext(ctx)
	if err := s.writeAudit(ctx, id.TenantID, auditActionCourseUpdate, auditTargetCourse, courseID, map[string]any{"name": req.Name}); err != nil {
		return CourseDTO{}, err
	}
	return courseDTOFromRow(row), nil
}

// PublishCourse 发布课程。
func (s *Service) PublishCourse(ctx context.Context, courseID int64) (CourseDTO, error) {
	return s.updateCourseStatus(ctx, courseID, CourseStatusPublished)
}

// EndCourse 结束课程。
func (s *Service) EndCourse(ctx context.Context, courseID int64) (CourseDTO, error) {
	return s.updateCourseStatus(ctx, courseID, CourseStatusEnded)
}

// ArchiveCourse 归档课程。
func (s *Service) ArchiveCourse(ctx context.Context, courseID int64) (CourseDTO, error) {
	return s.updateCourseStatus(ctx, courseID, CourseStatusArchived)
}

// CloneCourse 克隆课程结构,不复制成员/提交/成绩。
func (s *Service) CloneCourse(ctx context.Context, courseID int64) (CourseDTO, error) {
	source, err := s.loadCourse(ctx, courseID)
	if err != nil {
		return CourseDTO{}, err
	}
	id, ok := tenantFromContext(ctx)
	if !ok {
		return CourseDTO{}, apperr.ErrUnauthorized
	}
	if source.TeacherID != id.AccountID && source.Visibility != CourseVisibilityShared {
		return CourseDTO{}, apperr.ErrCourseForbidden
	}
	newCourseID := s.idgen.Generate()
	inviteCode, err := s.newInviteCode()
	if err != nil {
		return CourseDTO{}, apperr.ErrCourseInviteFailed.WithCause(err)
	}
	var row sqlcgen.Course
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		created, err := q.CreateCourse(ctx, sqlcgen.CreateCourseParams{
			ID: newCourseID, TenantID: id.TenantID, TeacherID: id.AccountID, Name: source.Name + " 副本",
			Description: source.Description, Type: source.Type, Difficulty: source.Difficulty, CoverUrl: source.CoverUrl,
			Semester: source.Semester, Credits: source.Credits, Schedule: source.Schedule, InviteCode: inviteCode,
			Status: CourseStatusDraft, Visibility: CourseVisibilityPrivate,
		})
		if err != nil {
			return err
		}
		row = created
		return s.cloneCourseStructure(ctx, q, id.TenantID, courseID, newCourseID)
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return CourseDTO{}, ae
		}
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionCourseCreate, auditTargetCourse, newCourseID, map[string]any{"source_course_id": ids.Format(courseID), "name": row.Name}); err != nil {
		return CourseDTO{}, err
	}
	return courseDTOFromRow(row), nil
}

// ShareCourse 把课程共享到课程库。
func (s *Service) ShareCourse(ctx context.Context, courseID int64) (CourseDTO, error) {
	return s.updateCourseVisibility(ctx, courseID, CourseVisibilityShared)
}

// RefreshInviteCode 刷新课程邀请码。
func (s *Service) RefreshInviteCode(ctx context.Context, courseID int64) (CourseDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return CourseDTO{}, err
	}
	inviteCode, err := s.newInviteCode()
	if err != nil {
		return CourseDTO{}, apperr.ErrCourseInviteFailed.WithCause(err)
	}
	var row sqlcgen.Course
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.UpdateCourseInviteCode(ctx, sqlcgen.UpdateCourseInviteCodeParams{ID: courseID, InviteCode: inviteCode})
		if db.IsNoRows(e) {
			return apperr.ErrCourseNotFound
		}
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return CourseDTO{}, ae
		}
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	return courseDTOFromRow(row), nil
}

// JoinCourseByInvite 按邀请码加入课程。
func (s *Service) JoinCourseByInvite(ctx context.Context, inviteCode string) (MemberDTO, error) {
	tenantID, studentID, err := s.ensureStudentRole(ctx, apperr.ErrCourseMemberInvalid)
	if err != nil {
		return MemberDTO{}, err
	}
	var member sqlcgen.CourseMember
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		course, err := q.GetCourseByInviteCode(ctx, strings.TrimSpace(inviteCode))
		if db.IsNoRows(err) {
			return apperr.ErrCourseJoinInvalid
		}
		if err != nil {
			return err
		}
		member, err = q.AddCourseMember(ctx, sqlcgen.AddCourseMemberParams{ID: s.idgen.Generate(), TenantID: tenantID, CourseID: course.ID, StudentID: studentID, JoinMode: JoinModeInvite})
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return MemberDTO{}, ae
		}
		return MemberDTO{}, apperr.ErrCourseJoinInvalid.WithCause(err)
	}
	return memberDTOFromRow(member), nil
}

// ListMembers 查询课程成员列表。
func (s *Service) ListMembers(ctx context.Context, courseID int64, page, size int) ([]MemberDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return nil, err
	}
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.CourseMember
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListCourseMembers(ctx, sqlcgen.ListCourseMembersParams{CourseID: courseID, LimitCount: int32(size), OffsetCount: int32((page - 1) * size)})
		return err
	}); err != nil {
		return nil, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return memberDTOsFromRows(rows), nil
}

// AddMembers 批量添加课程成员。
func (s *Service) AddMembers(ctx context.Context, courseID int64, studentIDs []string) ([]MemberDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return nil, err
	}
	id, _ := tenantFromContext(ctx)
	out := []MemberDTO{}
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		for _, raw := range studentIDs {
			studentID, ok := ids.Parse(raw)
			if !ok {
				return apperr.ErrCourseMemberInvalid
			}
			if err := s.ensureAccountStudent(ctx, studentID); err != nil {
				return err
			}
			member, err := q.AddCourseMember(ctx, sqlcgen.AddCourseMemberParams{ID: s.idgen.Generate(), TenantID: id.TenantID, CourseID: courseID, StudentID: studentID, JoinMode: JoinModeTeacher})
			if err != nil {
				return err
			}
			out = append(out, memberDTOFromRow(member))
		}
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrCourseMemberInvalid.WithCause(err)
	}
	return out, s.writeAudit(ctx, id.TenantID, auditActionMemberChange, auditTargetCourse, courseID, map[string]any{"added": len(out)})
}

// RemoveMember 移除课程成员。
func (s *Service) RemoveMember(ctx context.Context, courseID, studentID int64) error {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return err
	}
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		return q.RemoveCourseMember(ctx, sqlcgen.RemoveCourseMemberParams{CourseID: courseID, StudentID: studentID})
	}); err != nil {
		return apperr.ErrCourseMemberInvalid.WithCause(err)
	}
	id, _ := tenantFromContext(ctx)
	return s.writeAudit(ctx, id.TenantID, auditActionMemberChange, auditTargetCourse, courseID, map[string]any{"removed": ids.Format(studentID)})
}
