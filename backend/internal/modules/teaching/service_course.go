// teaching service_course 文件实现课程、章节、课时和成员业务。
package teaching

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

// ListCourses 查询教师或学生课程列表。
func (s *Service) ListCourses(ctx context.Context, filter CourseListFilter) ([]CourseDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	normalizePage(&filter.Page, &filter.Size)
	role := strings.TrimSpace(filter.Role)
	if role == "" {
		role = "student"
	}
	var (
		courses []Course
		total   int64
	)
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		if role == "teacher" {
			courses, total, err = tx.ListTeacherCourses(ctx, id.TenantID, id.AccountID, filter)
			return err
		}
		courses, total, err = tx.ListStudentCourses(ctx, id.TenantID, id.AccountID, filter)
		return err
	}); err != nil {
		return nil, 0, 0, 0, mapCourseError(err)
	}
	out := make([]CourseDTO, 0, len(courses))
	for _, course := range courses {
		out = append(out, courseDTO(course))
	}
	return out, total, filter.Page, filter.Size, nil
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
	course := Course{ID: s.ids.Generate(), TenantID: id.TenantID, TeacherID: id.AccountID, Name: req.Name, Description: req.Description, Type: req.Type, Difficulty: req.Difficulty, CoverURL: req.CoverURL, Semester: req.Semester, Credits: req.Credits, Schedule: req.Schedule, StartAt: startAt, EndAt: endAt, InviteCode: newInviteCode(), Status: CourseStatusDraft, Visibility: CourseVisibilityPrivate}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err = tx.CreateCourse(ctx, course)
		return err
	}); err != nil {
		return CourseDTO{}, mapCourseError(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "teaching.course.create", auditTargetCourse, course.ID, map[string]any{"name": course.Name}); err != nil {
		return CourseDTO{}, err
	}
	return courseDTO(course), nil
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
	var course Course
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(current, id.AccountID); err != nil {
			return err
		}
		current.Name, current.Description, current.Type, current.Difficulty, current.CoverURL, current.Semester, current.Credits, current.Schedule, current.StartAt, current.EndAt = req.Name, req.Description, req.Type, req.Difficulty, req.CoverURL, req.Semester, req.Credits, req.Schedule, startAt, endAt
		course, err = tx.UpdateCourse(ctx, current)
		return err
	}); err != nil {
		return CourseDTO{}, mapCourseError(err)
	}
	return courseDTO(course), nil
}

// CloneCourse 克隆本租户课程或共享课程库课程为当前教师的私有草稿。
func (s *Service) CloneCourse(ctx context.Context, courseID int64, req CloneCourseRequest) (CourseDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	req.Name = strings.TrimSpace(req.Name)
	var cloned Course
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		source, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		cloned, err = s.cloneCourseGraph(ctx, tx, source, id.TenantID, id.AccountID, req.Name)
		return err
	})
	if err != nil && isNoRows(err) {
		err = s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
			source, err := tx.GetCloneableCourse(ctx, courseID, id.TenantID)
			if err != nil {
				return err
			}
			if source.TenantID == id.TenantID {
				return apperr.ErrTeachingCourseNotFound
			}
			cloned, err = s.cloneCourseGraph(ctx, tx, source, id.TenantID, id.AccountID, req.Name)
			return err
		})
	}
	if err != nil {
		return CourseDTO{}, mapCourseError(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "teaching.course.clone", auditTargetCourse, cloned.ID, map[string]any{"source_course_id": courseID}); err != nil {
		return CourseDTO{}, err
	}
	return courseDTO(cloned), nil
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
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	var course Course
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(current, id.AccountID); err != nil {
			return err
		}
		course, err = tx.SetCourseVisibility(ctx, id.TenantID, courseID, CourseVisibilityShared)
		return err
	}); err != nil {
		return CourseDTO{}, mapCourseError(err)
	}
	return courseDTO(course), nil
}

// RefreshInviteCode 刷新课程邀请码。
func (s *Service) RefreshInviteCode(ctx context.Context, courseID int64) (CourseDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	var course Course
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(current, id.AccountID); err != nil {
			return err
		}
		course, err = tx.RefreshCourseInviteCode(ctx, id.TenantID, courseID, newInviteCode())
		return err
	}); err != nil {
		return CourseDTO{}, mapCourseError(err)
	}
	return courseDTO(course), nil
}

// AdvanceCourseStatusesOnce 按课程起止时间推进 published/running/ended 状态。
func (s *Service) AdvanceCourseStatusesOnce(ctx context.Context, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		dueToEnd, err := tx.ListCoursesDueToEnd(ctx, now)
		if err != nil {
			return err
		}
		for _, course := range dueToEnd {
			if _, err := tx.SetCourseStatus(ctx, course.TenantID, course.ID, CourseStatusEnded); err != nil {
				return err
			}
		}
		dueToRun, err := tx.ListCoursesDueToRun(ctx, now)
		if err != nil {
			return err
		}
		for _, course := range dueToRun {
			if _, err := tx.SetCourseStatus(ctx, course.TenantID, course.ID, CourseStatusRunning); err != nil {
				return err
			}
		}
		return nil
	})
}

// setCourseStatus 校验负责人后更新课程状态。
func (s *Service) setCourseStatus(ctx context.Context, courseID int64, status int16, action string) (CourseDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	var course Course
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetCourse(ctx, id.TenantID, courseID)
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
		return err
	}); err != nil {
		return CourseDTO{}, mapCourseError(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, action, auditTargetCourse, course.ID, map[string]any{"status": status}); err != nil {
		return CourseDTO{}, err
	}
	return courseDTO(course), nil
}

// cloneCourseGraph 在同一个事务中复制课程、章节、课时、作业和作业题目引用。
func (s *Service) cloneCourseGraph(ctx context.Context, tx TxStore, source Course, targetTenantID, teacherID int64, name string) (Course, error) {
	if source.TenantID != targetTenantID && source.Visibility != CourseVisibilityShared {
		return Course{}, apperr.ErrTeachingCourseForbidden
	}
	if name == "" {
		name = source.Name + " 副本"
	}
	cloned := Course{
		ID:          s.ids.Generate(),
		TenantID:    targetTenantID,
		TeacherID:   teacherID,
		Name:        name,
		Description: source.Description,
		Type:        source.Type,
		Difficulty:  source.Difficulty,
		CoverURL:    source.CoverURL,
		Semester:    source.Semester,
		Credits:     source.Credits,
		Schedule:    cloneMap(source.Schedule),
		StartAt:     source.StartAt,
		EndAt:       source.EndAt,
		InviteCode:  newInviteCode(),
		Status:      CourseStatusDraft,
		Visibility:  CourseVisibilityPrivate,
	}
	created, err := tx.CreateCourse(ctx, cloned)
	if err != nil {
		return Course{}, err
	}
	chapterMap, err := s.cloneChaptersAndLessons(ctx, tx, source, targetTenantID, created.ID)
	if err != nil {
		return Course{}, err
	}
	if err := s.cloneAssignments(ctx, tx, source, targetTenantID, created.ID, chapterMap); err != nil {
		return Course{}, err
	}
	return created, nil
}

// cloneChaptersAndLessons 复制课程目录结构并返回源章节到目标章节的映射。
func (s *Service) cloneChaptersAndLessons(ctx context.Context, tx TxStore, source Course, targetTenantID, targetCourseID int64) (map[int64]int64, error) {
	sourceChapters, err := tx.ListChapters(ctx, source.TenantID, source.ID)
	if err != nil {
		return nil, err
	}
	chapterMap := make(map[int64]int64, len(sourceChapters))
	for _, chapter := range sourceChapters {
		clonedChapter, err := tx.CreateChapter(ctx, Chapter{ID: s.ids.Generate(), TenantID: targetTenantID, CourseID: targetCourseID, Title: chapter.Title, Sort: chapter.Sort})
		if err != nil {
			return nil, err
		}
		chapterMap[chapter.ID] = clonedChapter.ID
		lessons, err := tx.ListLessonsByChapter(ctx, source.TenantID, chapter.ID)
		if err != nil {
			return nil, err
		}
		for _, lesson := range lessons {
			if _, err := tx.CreateLesson(ctx, Lesson{ID: s.ids.Generate(), TenantID: targetTenantID, ChapterID: clonedChapter.ID, Title: lesson.Title, ContentType: lesson.ContentType, ContentRef: cloneMap(lesson.ContentRef), Sort: lesson.Sort}); err != nil {
				return nil, err
			}
		}
	}
	return chapterMap, nil
}

// cloneAssignments 复制作业壳和题目引用,不复制任何提交或成绩数据。
func (s *Service) cloneAssignments(ctx context.Context, tx TxStore, source Course, targetTenantID, targetCourseID int64, chapterMap map[int64]int64) error {
	assignments, err := tx.ListAssignmentsByCourse(ctx, source.TenantID, source.ID)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		targetChapterID := int64(0)
		if assignment.ChapterID > 0 {
			targetChapterID = chapterMap[assignment.ChapterID]
			if targetChapterID == 0 {
				return apperr.ErrTeachingAssignmentInvalid
			}
		}
		clonedAssignment, err := tx.CreateAssignment(ctx, Assignment{ID: s.ids.Generate(), TenantID: targetTenantID, CourseID: targetCourseID, Title: assignment.Title, ChapterID: targetChapterID, DueAt: assignment.DueAt, MaxAttempts: assignment.MaxAttempts, LatePolicy: assignment.LatePolicy, LatePenalty: cloneMap(assignment.LatePenalty), Status: AssignmentStatusDraft})
		if err != nil {
			return err
		}
		items, err := tx.ListAssignmentItems(ctx, source.TenantID, assignment.ID)
		if err != nil {
			return err
		}
		clonedItems := make([]AssignmentItem, 0, len(items))
		for _, item := range items {
			if err := s.content.IncrementUsage(ctx, targetTenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion}); err != nil {
				return apperr.ErrTeachingAssignmentInvalid.WithCause(err)
			}
			clonedItems = append(clonedItems, AssignmentItem{ID: s.ids.Generate(), TenantID: targetTenantID, AssignmentID: clonedAssignment.ID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, Seq: item.Seq, GradingMode: item.GradingMode, JudgerCode: item.JudgerCode})
		}
		if _, err := tx.ReplaceAssignmentItems(ctx, targetTenantID, clonedAssignment.ID, clonedItems); err != nil {
			return err
		}
	}
	return nil
}

// newInviteCode 生成短邀请码。
func newInviteCode() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(time.Now().UTC().Format("150405.000000"))))[:10]
	}
	return strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf[:]))[:10]
}
