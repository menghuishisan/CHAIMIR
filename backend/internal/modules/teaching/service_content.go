// teaching service_content 文件实现章节、课时、成员和课程学习业务。
package teaching

import (
	"context"
	"strings"

	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
)

// CreateChapter 创建课程章节。
func (s *Service) CreateChapter(ctx context.Context, courseID int64, req ChapterRequest) (ChapterDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ChapterDTO{}, err
	}
	req, err = validateChapterRequest(req)
	if err != nil {
		return ChapterDTO{}, err
	}
	chapter := Chapter{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, Title: req.Title, Sort: req.Sort}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		chapter, err = tx.CreateChapter(ctx, chapter)
		return err
	}); err != nil {
		return ChapterDTO{}, mapCourseError(err)
	}
	return chapterDTO(chapter), nil
}

// ListChapters 查询课程章节。
func (s *Service) ListChapters(ctx context.Context, courseID int64) ([]ChapterDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var chapters []Chapter
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		var err error
		chapters, err = tx.ListChapters(ctx, id.TenantID, courseID)
		return err
	}); err != nil {
		return nil, mapCourseError(err)
	}
	out := make([]ChapterDTO, 0, len(chapters))
	for _, chapter := range chapters {
		out = append(out, chapterDTO(chapter))
	}
	return out, nil
}

// UpdateChapter 更新章节。
func (s *Service) UpdateChapter(ctx context.Context, chapterID int64, req ChapterRequest) (ChapterDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ChapterDTO{}, err
	}
	req, err = validateChapterRequest(req)
	if err != nil {
		return ChapterDTO{}, err
	}
	var chapter Chapter
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetChapter(ctx, id.TenantID, chapterID)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, current.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		current.Title, current.Sort = req.Title, req.Sort
		chapter, err = tx.UpdateChapter(ctx, current)
		return err
	}); err != nil {
		return ChapterDTO{}, mapCourseError(err)
	}
	return chapterDTO(chapter), nil
}

// DeleteChapter 删除章节。
func (s *Service) DeleteChapter(ctx context.Context, chapterID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	return s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		chapter, err := tx.GetChapter(ctx, id.TenantID, chapterID)
		if err != nil {
			return mapCourseError(err)
		}
		course, err := tx.GetCourse(ctx, id.TenantID, chapter.CourseID)
		if err != nil {
			return mapCourseError(err)
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		_, err = tx.DeleteChapter(ctx, id.TenantID, chapterID)
		return mapCourseError(err)
	})
}

// CreateLesson 创建课时。
func (s *Service) CreateLesson(ctx context.Context, chapterID int64, req LessonRequest) (LessonDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return LessonDTO{}, err
	}
	req, err = validateLessonRequest(req)
	if err != nil {
		return LessonDTO{}, err
	}
	lesson := Lesson{ID: s.ids.Generate(), TenantID: id.TenantID, ChapterID: chapterID, Title: req.Title, ContentType: req.ContentType, ContentRef: req.ContentRef, Sort: req.Sort}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		chapter, err := tx.GetChapter(ctx, id.TenantID, chapterID)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, chapter.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		lesson, err = tx.CreateLesson(ctx, lesson)
		return err
	}); err != nil {
		return LessonDTO{}, mapCourseError(err)
	}
	return lessonDTO(lesson), nil
}

// GetLessonForUser 读取课时内容。
func (s *Service) GetLessonForUser(ctx context.Context, lessonID int64) (LessonDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return LessonDTO{}, err
	}
	var lesson Lesson
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		lesson, err = tx.GetLesson(ctx, id.TenantID, lessonID)
		if err != nil {
			return err
		}
		chapter, err := tx.GetChapter(ctx, id.TenantID, lesson.ChapterID)
		if err != nil {
			return err
		}
		return s.ensureCourseReadable(ctx, tx, id.TenantID, chapter.CourseID, id.AccountID)
	}); err != nil {
		return LessonDTO{}, mapCourseError(err)
	}
	return lessonDTO(lesson), nil
}

// ListLessonsByChapter 查询章节下课时列表。
func (s *Service) ListLessonsByChapter(ctx context.Context, chapterID int64) ([]LessonDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var lessons []Lesson
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		chapter, err := tx.GetChapter(ctx, id.TenantID, chapterID)
		if err != nil {
			return err
		}
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, chapter.CourseID, id.AccountID); err != nil {
			return err
		}
		lessons, err = tx.ListLessonsByChapter(ctx, id.TenantID, chapterID)
		return err
	}); err != nil {
		return nil, mapCourseError(err)
	}
	out := make([]LessonDTO, 0, len(lessons))
	for _, lesson := range lessons {
		out = append(out, lessonDTO(lesson))
	}
	return out, nil
}

// UpdateLesson 更新课时。
func (s *Service) UpdateLesson(ctx context.Context, lessonID int64, req LessonRequest) (LessonDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return LessonDTO{}, err
	}
	req, err = validateLessonRequest(req)
	if err != nil {
		return LessonDTO{}, err
	}
	var lesson Lesson
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetLesson(ctx, id.TenantID, lessonID)
		if err != nil {
			return err
		}
		chapter, err := tx.GetChapter(ctx, id.TenantID, current.ChapterID)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, chapter.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		current.Title, current.ContentType, current.ContentRef, current.Sort = req.Title, req.ContentType, req.ContentRef, req.Sort
		lesson, err = tx.UpdateLesson(ctx, current)
		return err
	}); err != nil {
		return LessonDTO{}, mapCourseError(err)
	}
	return lessonDTO(lesson), nil
}

// SetLessonContent 更新课时绑定的内容引用。
func (s *Service) SetLessonContent(ctx context.Context, lessonID int64, req LessonRequest) (LessonDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return LessonDTO{}, err
	}
	req, err = validateLessonRequest(req)
	if err != nil {
		return LessonDTO{}, err
	}
	var lesson Lesson
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetLesson(ctx, id.TenantID, lessonID)
		if err != nil {
			return err
		}
		chapter, err := tx.GetChapter(ctx, id.TenantID, current.ChapterID)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, chapter.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		lesson, err = tx.SetLessonContent(ctx, id.TenantID, lessonID, req.ContentType, req.ContentRef)
		return err
	}); err != nil {
		return LessonDTO{}, mapCourseError(err)
	}
	return lessonDTO(lesson), nil
}

// DeleteLesson 删除课时。
func (s *Service) DeleteLesson(ctx context.Context, lessonID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	return s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		lesson, err := tx.GetLesson(ctx, id.TenantID, lessonID)
		if err != nil {
			return mapCourseError(err)
		}
		chapter, err := tx.GetChapter(ctx, id.TenantID, lesson.ChapterID)
		if err != nil {
			return mapCourseError(err)
		}
		course, err := tx.GetCourse(ctx, id.TenantID, chapter.CourseID)
		if err != nil {
			return mapCourseError(err)
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		_, err = tx.DeleteLesson(ctx, id.TenantID, lessonID)
		return mapCourseError(err)
	})
}

// JoinCourseByInvite 学生通过邀请码加入课程。
func (s *Service) JoinCourseByInvite(ctx context.Context, req JoinCourseRequest) (MemberDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return MemberDTO{}, err
	}
	code := strings.TrimSpace(req.InviteCode)
	if code == "" {
		return MemberDTO{}, apperr.ErrTeachingInviteInvalid
	}
	var member CourseMember
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourseByInviteCode(ctx, code)
		if err != nil {
			return apperr.ErrTeachingInviteInvalid
		}
		if course.TenantID != id.TenantID {
			return apperr.ErrTeachingInviteInvalid
		}
		if err := ensureCourseJoinable(course); err != nil {
			return err
		}
		member, err = tx.CreateCourseMember(ctx, CourseMember{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: course.ID, StudentID: id.AccountID, JoinMode: JoinModeInvite})
		return err
	}); err != nil {
		return MemberDTO{}, err
	}
	return memberDTO(member), nil
}

// AddCourseMembers 批量添加课程成员。
func (s *Service) AddCourseMembers(ctx context.Context, courseID int64, req BatchMembersRequest) ([]MemberDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.StudentIDs) == 0 {
		return nil, apperr.ErrTeachingMemberInvalid
	}
	var members []CourseMember
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		if err := ensureCanManageMembers(course); err != nil {
			return err
		}
		members = make([]CourseMember, 0, len(req.StudentIDs))
		for _, studentID := range req.StudentIDs {
			if studentID <= 0 {
				return apperr.ErrTeachingMemberInvalid
			}
			member, err := tx.CreateCourseMember(ctx, CourseMember{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, StudentID: studentID, JoinMode: JoinModeTeacher})
			if err != nil {
				return err
			}
			members = append(members, member)
		}
		return nil
	}); err != nil {
		return nil, mapCourseError(err)
	}
	out := make([]MemberDTO, 0, len(members))
	for _, member := range members {
		out = append(out, memberDTO(member))
	}
	return out, nil
}

// ListCourseMembers 查询课程成员。
func (s *Service) ListCourseMembers(ctx context.Context, courseID int64, page, size int) ([]MemberDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	var members []CourseMember
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		members, total, err = tx.ListCourseMembers(ctx, id.TenantID, courseID, page, size)
		return err
	}); err != nil {
		return nil, 0, 0, 0, mapCourseError(err)
	}
	out := make([]MemberDTO, 0, len(members))
	for _, member := range members {
		out = append(out, memberDTO(member))
	}
	return out, total, page, size, nil
}

// RemoveCourseMember 移除课程成员。
func (s *Service) RemoveCourseMember(ctx context.Context, courseID, studentID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	return s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return mapCourseError(err)
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		if err := ensureCanManageMembers(course); err != nil {
			return err
		}
		return tx.DeleteCourseMember(ctx, id.TenantID, courseID, studentID)
	})
}

// GetCourseOutline 读取课程目录与本人进度。
func (s *Service) GetCourseOutline(ctx context.Context, courseID int64) (OutlineDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return OutlineDTO{}, err
	}
	var course Course
	var chapters []Chapter
	var lessons []Lesson
	var progresses []LessonProgress
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		course, err = tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		chapters, err = tx.ListChapters(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		lessons, err = tx.ListLessonsByCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		progresses, err = tx.ListStudentProgressByCourse(ctx, id.TenantID, courseID, id.AccountID)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return OutlineDTO{}, mapCourseError(err)
	}
	out := OutlineDTO{Course: courseDTO(course)}
	for _, chapter := range chapters {
		out.Chapters = append(out.Chapters, chapterDTO(chapter))
	}
	for _, lesson := range lessons {
		out.Lessons = append(out.Lessons, lessonDTO(lesson))
	}
	for _, progress := range progresses {
		out.Progress = append(out.Progress, progressDTO(progress))
	}
	return out, nil
}

// ensureCourseReadable 校验账号是否为授课教师或课程成员。
func (s *Service) ensureCourseReadable(ctx context.Context, tx TxStore, tenantID, courseID, accountID int64) error {
	course, err := tx.GetCourse(ctx, tenantID, courseID)
	if err != nil {
		return err
	}
	if course.TeacherID == accountID {
		return nil
	}
	if _, err := tx.GetCourseMember(ctx, tenantID, courseID, accountID); err != nil {
		return apperr.ErrTeachingCourseForbidden
	}
	return nil
}
