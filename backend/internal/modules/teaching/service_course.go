// 课程查询、基本信息、共享与生命周期管理。
package teaching

import (
	"context"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/logging"
)

const inviteCodeLength = 10

// ListCourses 查询教师或学生课程列表,并补上卡片要显示的事实:
// 授课教师姓名、选课人数(教师视角)、本人学习进度(学生视角),都按当前页一次聚合。
func (s *Service) ListCourses(ctx context.Context, filter CourseListFilter) ([]CourseDTO, int64, int, int, pagex.Facets, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, nil, err
	}
	filter.Page, filter.Size = pagex.Normalize(filter.Page, filter.Size)
	role := strings.TrimSpace(filter.Role)
	if role == "" {
		role = contracts.RoleStudent
	}
	var (
		courses  []Course
		total    int64
		facts    courseFacts
		byStatus pagex.FacetGroup
	)
	studentView := role != contracts.RoleTeacher
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		if studentView {
			courses, total, err = tx.ListStudentCourses(ctx, id.TenantID, id.AccountID, filter)
		} else {
			courses, total, err = tx.ListTeacherCourses(ctx, id.TenantID, id.AccountID, filter)
		}
		if err != nil {
			return err
		}
		facts, err = loadCourseFacts(ctx, tx, id.TenantID, id.AccountID, courses, studentView)
		if err != nil {
			return err
		}
		byStatus, err = tx.CountCoursesByStatus(ctx, id.TenantID, id.AccountID, !studentView, filter.IsShared)
		return err
	}); err != nil {
		return nil, 0, 0, 0, nil, mapCourseError(err)
	}
	// 教师档案在 identity 模块,事务外经契约一次批量读取
	facts.teachers, err = s.courseTeachers(ctx, id.TenantID, courses)
	if err != nil {
		return nil, 0, 0, 0, nil, err
	}
	out := make([]CourseDTO, 0, len(courses))
	for _, course := range courses {
		dto, err := courseDTO(course)
		if err != nil {
			return nil, 0, 0, 0, nil, mapCourseError(err)
		}
		out = append(out, dto)
	}
	return applyCourseFacts(out, facts, studentView), total, filter.Page, filter.Size, pagex.Facets{"status": byStatus}, nil
}

// ListSharedCourses用当前租户的共享SELECT权限发现课程，不开放来源课程业务详情。
func (s *Service) ListSharedCourses(ctx context.Context, filter CourseListFilter) ([]SharedCourseDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	filter.Page, filter.Size = pagex.Normalize(filter.Page, filter.Size)
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	var courses []Course
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		courses, total, err = tx.ListSharedCourses(ctx, id.TenantID, filter)
		return err
	}); err != nil {
		return nil, 0, 0, 0, mapCourseError(err)
	}
	// 章节数要跨校读 chapter,走与克隆同一个受控特权事务;查询自身限定课程仍共享(M6 安全设计 §5)
	courseIDs := make([]int64, 0, len(courses))
	for _, course := range courses {
		courseIDs = append(courseIDs, course.ID)
	}
	chapters := map[int64]int64{}
	if len(courseIDs) > 0 {
		if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
			var err error
			chapters, err = tx.CountChaptersOfSharedCourses(ctx, id.TenantID, courseIDs)
			return err
		}); err != nil {
			return nil, 0, 0, 0, mapCourseError(err)
		}
	}
	return sharedCourseDTOs(courses, chapters), total, filter.Page, filter.Size, nil
}

// sharedCourseDTOs 把共享课程转换为目录投影并带上章节数;没有章节的课为 0。
func sharedCourseDTOs(courses []Course, chapters map[int64]int64) []SharedCourseDTO {
	out := make([]SharedCourseDTO, 0, len(courses))
	for _, course := range courses {
		out = append(out, SharedCourseDTO{ID: ids.ID(course.ID), Name: course.Name, Description: course.Description, Type: course.Type, Difficulty: course.Difficulty, Semester: course.Semester, Credits: course.Credits, UpdatedAt: timex.RFC3339OrEmpty(course.UpdatedAt), ChapterCount: chapters[course.ID]})
	}
	return out
}

// CreateCourse 创建课程草稿。
func (s *Service) CreateCourse(ctx context.Context, req CourseRequest) (CourseDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	req, startAt, endAt, err := validateCourseRequest(req)
	if err != nil {
		return CourseDTO{}, err
	}
	inviteCode, err := newInviteCode()
	if err != nil {
		return CourseDTO{}, err
	}
	courseID := s.ids.Generate()
	// 封面先搬到这门课的正式位再落库:课程 id 此刻已生成,不需要等记录写完。
	coverRef, err := s.promoteCourseCover(ctx, id.TenantID, id.AccountID, courseID, req.CoverRef, "")
	if err != nil {
		return CourseDTO{}, err
	}
	course := Course{ID: courseID, TenantID: id.TenantID, TeacherID: id.AccountID, Name: req.Name, Description: req.Description, Type: req.Type, Difficulty: req.Difficulty, CoverRef: coverRef, Semester: req.Semester, Credits: req.Credits, Schedule: req.Schedule, StartAt: startAt, EndAt: endAt, InviteCode: inviteCode, Status: CourseStatusDraft, Visibility: CourseVisibilityPrivate}
	var out CourseDTO
	commitAttempted := false
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err = tx.CreateCourse(ctx, course)
		if err != nil {
			return err
		}
		out, err = courseDTO(course)
		if err != nil {
			return err
		}
		if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "teaching.course.create", auditTargetCourse, course.ID, map[string]any{"name": course.Name}); err != nil {
			return err
		}
		commitAttempted = true
		return nil
	}); err != nil {
		err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, commitAttempted, func(ctx context.Context) error {
			return s.discardCourseCoverObject(ctx, id.TenantID, id.AccountID, courseID, coverRef)
		})
		return CourseDTO{}, mapCourseError(err)
	}
	if err := s.discardCourseCoverObject(ctx, id.TenantID, id.AccountID, 0, req.CoverRef); err != nil {
		logging.ErrorContext(ctx, "清理课程封面暂存对象失败", err.Error())
	}
	return out, nil
}

// UpdateCourse 更新课程基础信息。
func (s *Service) UpdateCourse(ctx context.Context, courseID int64, req CourseRequest) (CourseDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	req, startAt, endAt, err := validateCourseRequest(req)
	if err != nil {
		return CourseDTO{}, err
	}
	// 对象复制在事务外执行；提交时加锁确认封面未被其他编辑替换。
	var existing Course
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(current, id.AccountID); err != nil {
			return err
		}
		existing = current
		return nil
	}); err != nil {
		return CourseDTO{}, mapCourseError(err)
	}
	coverRef, err := s.promoteCourseCover(ctx, id.TenantID, id.AccountID, courseID, req.CoverRef, existing.CoverRef)
	if err != nil {
		return CourseDTO{}, err
	}
	var course Course
	var out CourseDTO
	commitAttempted := false
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetCourseForUpdate(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(current, id.AccountID); err != nil {
			return err
		}
		if current.CoverRef != existing.CoverRef {
			return apperr.ErrConflict
		}
		current.Name, current.Description, current.Type, current.Difficulty, current.CoverRef, current.Semester, current.Credits, current.Schedule, current.StartAt, current.EndAt = req.Name, req.Description, req.Type, req.Difficulty, coverRef, req.Semester, req.Credits, req.Schedule, startAt, endAt
		course, err = tx.UpdateCourse(ctx, current)
		if err != nil {
			return err
		}
		out, err = courseDTO(course)
		if err != nil {
			return err
		}
		if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "teaching.course.update", auditTargetCourse, courseID, map[string]any{"name": course.Name}); err != nil {
			return err
		}
		commitAttempted = true
		return nil
	}); err != nil {
		if coverRef != existing.CoverRef {
			err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, commitAttempted, func(ctx context.Context) error {
				return s.discardCourseCoverObject(ctx, id.TenantID, id.AccountID, courseID, coverRef)
			})
		}
		return CourseDTO{}, mapCourseError(err)
	}
	// 换封面后旧对象没有任何引用者(克隆不继承封面,一个对象只被一门课引用),留着只会堆积。
	if existing.CoverRef != course.CoverRef {
		if err := s.discardCourseCoverObject(ctx, id.TenantID, id.AccountID, courseID, existing.CoverRef); err != nil {
			logging.ErrorContext(ctx, "清理已替换课程封面失败", err.Error())
		}
		if err := s.discardCourseCoverObject(ctx, id.TenantID, id.AccountID, 0, req.CoverRef); err != nil {
			logging.ErrorContext(ctx, "清理课程封面暂存对象失败", err.Error())
		}
	}
	return out, nil
}

// PublishCourse 发布课程。
func (s *Service) PublishCourse(ctx context.Context, courseID int64) (CourseDTO, error) {
	return s.setCourseStatus(ctx, courseID, CourseStatusPublished, "teaching.course.publish")
}

// EndCourse 结束课程。
func (s *Service) EndCourse(ctx context.Context, courseID int64) (CourseDTO, error) {
	return s.setCourseStatus(ctx, courseID, CourseStatusEnded, "teaching.course.end")
}

// ArchiveCourse 归档课程。
func (s *Service) ArchiveCourse(ctx context.Context, courseID int64) (CourseDTO, error) {
	return s.setCourseStatus(ctx, courseID, CourseStatusArchived, "teaching.course.archive")
}

// ShareCourse 共享课程到课程库。
func (s *Service) ShareCourse(ctx context.Context, courseID int64) (CourseDTO, error) {
	return s.setCourseVisibility(ctx, courseID, CourseVisibilityShared, "teaching.course.share")
}

// UnshareCourse 撤销课程库可见性,不改变课程、学生或独立克隆。
func (s *Service) UnshareCourse(ctx context.Context, courseID int64) (CourseDTO, error) {
	return s.setCourseVisibility(ctx, courseID, CourseVisibilityPrivate, "teaching.course.unshare")
}

// setCourseVisibility 统一共享状态变更的租户、负责人、发布门禁和审计。
func (s *Service) setCourseVisibility(ctx context.Context, courseID int64, visibility int16, action string) (CourseDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	var out CourseDTO
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(current, id.AccountID); err != nil {
			return err
		}
		if visibility == CourseVisibilityShared && current.Status == CourseStatusDraft {
			return apperr.ErrTeachingCourseStateInvalid
		}
		course, err := tx.SetCourseVisibility(ctx, id.TenantID, courseID, visibility)
		if err != nil {
			return err
		}
		out, err = courseDTO(course)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, action, auditTargetCourse, course.ID, map[string]any{"visibility": visibility})
	}); err != nil {
		return CourseDTO{}, mapCourseError(err)
	}
	return out, nil
}

// RefreshInviteCode 刷新课程邀请码。
func (s *Service) RefreshInviteCode(ctx context.Context, courseID int64) (CourseDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	var out CourseDTO
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(current, id.AccountID); err != nil {
			return err
		}
		inviteCode, err := newInviteCode()
		if err != nil {
			return err
		}
		course, err := tx.RefreshCourseInviteCode(ctx, id.TenantID, courseID, inviteCode)
		if err != nil {
			return err
		}
		out, err = courseDTO(course)
		return err
	}); err != nil {
		return CourseDTO{}, mapCourseError(err)
	}
	return out, nil
}

// setCourseStatus 校验负责人后更新课程状态。
func (s *Service) setCourseStatus(ctx context.Context, courseID int64, status int16, action string) (CourseDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	var course Course
	var out CourseDTO
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetCourseForUpdate(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(current, id.AccountID); err != nil {
			return err
		}
		switch status {
		case CourseStatusPublished:
			lessonCount, err := tx.CountCourseLessons(ctx, id.TenantID, courseID)
			if err != nil {
				return err
			}
			if err := ensureCanPublishCourse(current, lessonCount); err != nil {
				return err
			}
		case CourseStatusEnded:
			if err := ensureCanEndCourse(current); err != nil {
				return err
			}
		case CourseStatusArchived:
			if err := ensureCanArchiveCourse(current); err != nil {
				return err
			}
		}
		course, err = tx.SetCourseStatus(ctx, id.TenantID, courseID, status)
		if err != nil {
			return err
		}
		out, err = courseDTO(course)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, action, auditTargetCourse, course.ID, map[string]any{"status": status})
	}); err != nil {
		return CourseDTO{}, mapCourseError(err)
	}
	return out, nil
}

// newInviteCode 生成短邀请码,随机源失败必须显式返回错误而不是退化为可预测码。
func newInviteCode() (string, error) {
	code, err := pkgcrypto.RandomToken(inviteCodeLength)
	if err != nil {
		return "", apperr.ErrTeachingCourseInvalid.WithCause(err)
	}
	return code, nil
}
