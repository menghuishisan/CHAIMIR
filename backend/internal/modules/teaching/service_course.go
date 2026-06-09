// M6 课程与成员服务:课程生命周期、共享、邀请码与选课成员管理。
package teaching

import (
	"context"
	"strings"

	"chaimir/internal/contracts"
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
	rows, err := s.repo.listCourses(ctx, id.AccountID, status, size, (page-1)*size, role == contracts.RoleStudent)
	if err != nil {
		return nil, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return rows, nil
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
	courseID := s.idgen.Generate()
	inviteCode, err := s.newInviteCode()
	if err != nil {
		return CourseDTO{}, apperr.ErrCourseInviteFailed.WithCause(err)
	}
	row, err := s.repo.createCourse(ctx, id.TenantID, courseID, id.AccountID, req, schedule, inviteCode)
	if err != nil {
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionCourseCreate, auditTargetCourse, courseID, map[string]any{"name": req.Name}); err != nil {
		return CourseDTO{}, err
	}
	return row, nil
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
	row, err := s.repo.updateCourse(ctx, courseID, req, schedule)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return CourseDTO{}, ae
		}
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	id, _ := tenantFromContext(ctx)
	if err := s.writeAudit(ctx, id.TenantID, auditActionCourseUpdate, auditTargetCourse, courseID, map[string]any{"name": req.Name}); err != nil {
		return CourseDTO{}, err
	}
	return row, nil
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
	row, err := s.repo.cloneCourseWithStructure(ctx, id.TenantID, newCourseID, id.AccountID, courseID, source.Name+" 副本", inviteCode, s.idgen.Generate)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return CourseDTO{}, ae
		}
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionCourseCreate, auditTargetCourse, newCourseID, map[string]any{"source_course_id": ids.Format(courseID), "name": row.Name}); err != nil {
		return CourseDTO{}, err
	}
	return row, nil
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
	row, err := s.repo.updateCourseInviteCode(ctx, courseID, inviteCode)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return CourseDTO{}, ae
		}
		return CourseDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	return row, nil
}

// JoinCourseByInvite 按邀请码加入课程。
func (s *Service) JoinCourseByInvite(ctx context.Context, inviteCode string) (MemberDTO, error) {
	tenantID, studentID, err := s.ensureStudentRole(ctx, apperr.ErrCourseMemberInvalid)
	if err != nil {
		return MemberDTO{}, err
	}
	member, err := s.repo.joinCourseByInvite(ctx, s.idgen.Generate(), tenantID, studentID, strings.TrimSpace(inviteCode))
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return MemberDTO{}, ae
		}
		return MemberDTO{}, apperr.ErrCourseJoinInvalid.WithCause(err)
	}
	return member, nil
}

// ListMembers 查询课程成员列表。
func (s *Service) ListMembers(ctx context.Context, courseID int64, page, size int) ([]MemberDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return nil, err
	}
	page, size = pagex.Normalize(page, size)
	rows, err := s.repo.listCourseMembers(ctx, courseID, size, (page-1)*size)
	if err != nil {
		return nil, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return rows, nil
}

// AddMembers 批量添加课程成员。
func (s *Service) AddMembers(ctx context.Context, courseID int64, studentIDs []string) ([]MemberDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return nil, err
	}
	id, _ := tenantFromContext(ctx)
	parsedIDs := make([]int64, 0, len(studentIDs))
	for _, raw := range studentIDs {
		studentID, ok := ids.Parse(raw)
		if !ok {
			return nil, apperr.ErrCourseMemberInvalid
		}
		if err := s.ensureAccountStudent(ctx, studentID); err != nil {
			return nil, err
		}
		parsedIDs = append(parsedIDs, studentID)
	}
	rows, err := s.repo.addCourseMembers(ctx, id.TenantID, courseID, parsedIDs, s.idgen.Generate)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrCourseMemberInvalid.WithCause(err)
	}
	return rows, s.writeAudit(ctx, id.TenantID, auditActionMemberChange, auditTargetCourse, courseID, map[string]any{"added": len(rows)})
}

// RemoveMember 移除课程成员。
func (s *Service) RemoveMember(ctx context.Context, courseID, studentID int64) error {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return err
	}
	if err := s.repo.removeCourseMember(ctx, courseID, studentID); err != nil {
		return apperr.ErrCourseMemberInvalid.WithCause(err)
	}
	id, _ := tenantFromContext(ctx)
	return s.writeAudit(ctx, id.TenantID, auditActionMemberChange, auditTargetCourse, courseID, map[string]any{"removed": ids.Format(studentID)})
}
