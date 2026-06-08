// M6 章节课时服务:课程内容结构、课时内容引用与课程目录。
package teaching

import (
	"context"
	"strings"

	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// CreateChapter 创建章节。
func (s *Service) CreateChapter(ctx context.Context, courseID int64, req ChapterRequest) (ChapterDTO, error) {
	if strings.TrimSpace(req.Title) == "" {
		return ChapterDTO{}, apperr.ErrCourseInvalid
	}
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return ChapterDTO{}, err
	}
	id, _ := tenantFromContext(ctx)
	var row sqlcgen.Chapter
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateChapter(ctx, sqlcgen.CreateChapterParams{ID: s.idgen.Generate(), TenantID: id.TenantID, CourseID: courseID, Title: req.Title, Sort: req.Sort})
		return err
	}); err != nil {
		return ChapterDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	return chapterDTOFromRow(row), s.writeAudit(ctx, id.TenantID, auditActionContentChange, auditTargetCourse, courseID, map[string]any{"chapter": row.Title})
}

// ListChapters 查询课程章节。
func (s *Service) ListChapters(ctx context.Context, courseID int64) ([]ChapterDTO, error) {
	if err := s.ensureCourseAccessible(ctx, courseID); err != nil {
		return nil, err
	}
	var rows []sqlcgen.Chapter
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListChaptersByCourse(ctx, courseID)
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	out := make([]ChapterDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, chapterDTOFromRow(row))
	}
	return out, nil
}

// UpdateChapter 更新章节。
func (s *Service) UpdateChapter(ctx context.Context, courseID, chapterID int64, req ChapterRequest) (ChapterDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return ChapterDTO{}, err
	}
	var row sqlcgen.Chapter
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateChapter(ctx, sqlcgen.UpdateChapterParams{ID: chapterID, Title: req.Title, Sort: req.Sort})
		if db.IsNoRows(err) {
			return apperr.ErrCourseNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ChapterDTO{}, ae
		}
		return ChapterDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	return chapterDTOFromRow(row), nil
}

// DeleteChapter 软删章节。
func (s *Service) DeleteChapter(ctx context.Context, courseID, chapterID int64) error {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return err
	}
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, err := q.SoftDeleteChapter(ctx, chapterID)
		if db.IsNoRows(err) {
			return apperr.ErrCourseNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrCourseInvalid.WithCause(err)
	}
	return nil
}

// CreateLesson 创建课时。
func (s *Service) CreateLesson(ctx context.Context, chapterID int64, req LessonRequest) (LessonDTO, error) {
	chapter, err := s.loadChapter(ctx, chapterID)
	if err != nil {
		return LessonDTO{}, err
	}
	if err := s.ensureTeacherOfCourse(ctx, chapter.CourseID); err != nil {
		return LessonDTO{}, err
	}
	if err := validateLessonContentRef(req.ContentType, req.ContentRef); err != nil {
		return LessonDTO{}, err
	}
	data, err := jsonx.ObjectBytes(req.ContentRef, apperr.ErrCourseInvalid)
	if err != nil {
		return LessonDTO{}, err
	}
	id, _ := tenantFromContext(ctx)
	var row sqlcgen.Lesson
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateLesson(ctx, sqlcgen.CreateLessonParams{ID: s.idgen.Generate(), TenantID: id.TenantID, ChapterID: chapterID, Title: req.Title, ContentType: req.ContentType, ContentRef: data, Sort: req.Sort})
		return createErr
	}); err != nil {
		return LessonDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	return lessonDTOFromRow(row), nil
}

// ListLessons 查询章节课时。
func (s *Service) ListLessons(ctx context.Context, chapterID int64) ([]LessonDTO, error) {
	chapter, err := s.loadChapter(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureCourseAccessible(ctx, chapter.CourseID); err != nil {
		return nil, err
	}
	var rows []sqlcgen.Lesson
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, findErr := q.ListLessonsByChapter(ctx, chapterID)
		rows = found
		return findErr
	}); err != nil {
		return nil, apperr.ErrCourseQueryFailed.WithCause(err)
	}
	return lessonDTOsFromRows(rows), nil
}

// GetLesson 查询课时内容。
func (s *Service) GetLesson(ctx context.Context, lessonID int64) (LessonDTO, error) {
	lesson, err := s.loadLesson(ctx, lessonID)
	if err != nil {
		return LessonDTO{}, err
	}
	chapter, err := s.loadChapter(ctx, lesson.ChapterID)
	if err != nil {
		return LessonDTO{}, err
	}
	if err := s.ensureCourseAccessible(ctx, chapter.CourseID); err != nil {
		return LessonDTO{}, err
	}
	return lessonDTOFromRow(lesson), nil
}

// UpdateLesson 更新课时。
func (s *Service) UpdateLesson(ctx context.Context, chapterID, lessonID int64, req LessonRequest) (LessonDTO, error) {
	chapter, err := s.loadChapter(ctx, chapterID)
	if err != nil {
		return LessonDTO{}, err
	}
	if err := s.ensureTeacherOfCourse(ctx, chapter.CourseID); err != nil {
		return LessonDTO{}, err
	}
	if err := validateLessonContentRef(req.ContentType, req.ContentRef); err != nil {
		return LessonDTO{}, err
	}
	data, err := jsonx.ObjectBytes(req.ContentRef, apperr.ErrCourseInvalid)
	if err != nil {
		return LessonDTO{}, err
	}
	var row sqlcgen.Lesson
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.UpdateLesson(ctx, sqlcgen.UpdateLessonParams{ID: lessonID, Title: req.Title, ContentType: req.ContentType, ContentRef: data, Sort: req.Sort})
		if db.IsNoRows(updateErr) {
			return apperr.ErrCourseNotFound
		}
		return updateErr
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return LessonDTO{}, ae
		}
		return LessonDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	return lessonDTOFromRow(row), nil
}

// SetLessonContent 设置课时内容引用。
func (s *Service) SetLessonContent(ctx context.Context, lessonID int64, req LessonContentRequest) (LessonDTO, error) {
	current, err := s.loadLesson(ctx, lessonID)
	if err != nil {
		return LessonDTO{}, err
	}
	chapter, err := s.loadChapter(ctx, current.ChapterID)
	if err != nil {
		return LessonDTO{}, err
	}
	if err := s.ensureTeacherOfCourse(ctx, chapter.CourseID); err != nil {
		return LessonDTO{}, err
	}
	if err := validateLessonContentRef(req.ContentType, req.ContentRef); err != nil {
		return LessonDTO{}, err
	}
	data, err := jsonx.ObjectBytes(req.ContentRef, apperr.ErrCourseInvalid)
	if err != nil {
		return LessonDTO{}, err
	}
	var row sqlcgen.Lesson
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.UpdateLessonContent(ctx, sqlcgen.UpdateLessonContentParams{ID: lessonID, ContentType: req.ContentType, ContentRef: data})
		return updateErr
	}); err != nil {
		return LessonDTO{}, apperr.ErrCourseInvalid.WithCause(err)
	}
	return lessonDTOFromRow(row), nil
}

// DeleteLesson 软删课时。
func (s *Service) DeleteLesson(ctx context.Context, chapterID, lessonID int64) error {
	chapter, err := s.loadChapter(ctx, chapterID)
	if err != nil {
		return err
	}
	if err := s.ensureTeacherOfCourse(ctx, chapter.CourseID); err != nil {
		return err
	}
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, deleteErr := q.SoftDeleteLesson(ctx, lessonID)
		if db.IsNoRows(deleteErr) {
			return apperr.ErrCourseNotFound
		}
		return deleteErr
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrCourseInvalid.WithCause(err)
	}
	return nil
}

// GetCourseOutline 查询课程目录并展开课时。
func (s *Service) GetCourseOutline(ctx context.Context, courseID int64) ([]ChapterDTO, error) {
	chapters, err := s.ListChapters(ctx, courseID)
	if err != nil {
		return nil, err
	}
	for i := range chapters {
		chapterID, ok := ids.Parse(chapters[i].ID)
		if !ok {
			return nil, apperr.ErrCourseIDInvalid
		}
		lessons, err := s.ListLessons(ctx, chapterID)
		if err != nil {
			return nil, err
		}
		chapters[i].Lessons = lessons
	}
	return chapters, nil
}
