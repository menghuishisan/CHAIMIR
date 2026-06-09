// M6 进度与互动服务:课时进度、讨论、公告和课程评价。
package teaching

import (
	"context"
	"html"
	"strings"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
)

// UpsertProgress 上报课时进度。
func (s *Service) UpsertProgress(ctx context.Context, lessonID int64, req ProgressRequest) (ProgressStatsDTO, error) {
	if req.Status < ProgressNotStarted || req.Status > ProgressCompleted || req.DurationSec < 0 {
		return ProgressStatsDTO{}, apperr.ErrProgressInvalid
	}
	lesson, err := s.loadLesson(ctx, lessonID)
	if err != nil {
		return ProgressStatsDTO{}, err
	}
	chapter, err := s.loadChapter(ctx, lesson.ChapterID)
	if err != nil {
		return ProgressStatsDTO{}, err
	}
	tenantID, studentID, err := s.ensureStudentCourseMember(ctx, chapter.CourseID, apperr.ErrProgressForbidden)
	if err != nil {
		return ProgressStatsDTO{}, err
	}
	if err := s.repo.upsertLessonProgress(ctx, tenantID, s.idgen.Generate(), lessonID, studentID, req); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ProgressStatsDTO{}, ae
		}
		return ProgressStatsDTO{}, apperr.ErrProgressInvalid.WithCause(err)
	}
	return s.MyProgress(ctx, chapter.CourseID)
}

// ProgressStats 汇总课程学习进度。
func (s *Service) ProgressStats(ctx context.Context, courseID int64) (ProgressStatsDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return ProgressStatsDTO{}, err
	}
	rows, err := s.repo.listLessonProgressByCourse(ctx, courseID)
	if err != nil {
		return ProgressStatsDTO{}, apperr.ErrProgressQueryFailed.WithCause(err)
	}
	out := ProgressStatsDTO{CourseID: ids.Format(courseID)}
	for _, row := range rows {
		if row.Status == ProgressCompleted {
			out.CompletedCount++
		}
		if row.Status == ProgressInProgress {
			out.InProgressCount++
		}
		out.LearningDurationSec += int64(row.DurationSec)
	}
	return out, nil
}

// MyProgress 查询当前学生课程进度概览。
func (s *Service) MyProgress(ctx context.Context, courseID int64) (ProgressStatsDTO, error) {
	_, studentID, err := s.ensureStudentCourseMember(ctx, courseID, apperr.ErrProgressForbidden)
	if err != nil {
		return ProgressStatsDTO{}, err
	}
	rows, err := s.repo.listLessonProgressByCourseAndStudent(ctx, courseID, studentID)
	if err != nil {
		return ProgressStatsDTO{}, apperr.ErrProgressQueryFailed.WithCause(err)
	}
	out := ProgressStatsDTO{CourseID: ids.Format(courseID)}
	for _, row := range rows {
		if row.Status == ProgressCompleted {
			out.CompletedCount++
		}
		if row.Status == ProgressInProgress {
			out.InProgressCount++
		}
		out.LearningDurationSec += int64(row.DurationSec)
	}
	return out, nil
}

// ListPosts 查询讨论帖。
func (s *Service) ListPosts(ctx context.Context, courseID int64, page, size int) ([]PostDTO, error) {
	if err := s.ensureCourseAccessible(ctx, courseID); err != nil {
		return nil, err
	}
	page, size = pagex.Normalize(page, size)
	rows, err := s.repo.listDiscussionPosts(ctx, courseID, size, (page-1)*size)
	if err != nil {
		return nil, apperr.ErrDiscussionQueryFailed.WithCause(err)
	}
	return rows, nil
}

// CreatePost 创建讨论帖或回复,内容进行 HTML 转义后存储。
func (s *Service) CreatePost(ctx context.Context, courseID int64, req PostRequest) (PostDTO, error) {
	if strings.TrimSpace(req.Content) == "" {
		return PostDTO{}, apperr.ErrDiscussionInvalid
	}
	if err := s.ensureCourseAccessible(ctx, courseID); err != nil {
		return PostDTO{}, err
	}
	id, _ := tenantFromContext(ctx)
	parentID := ids.ParseOrZero(req.ParentID)
	row, err := s.repo.createDiscussionPost(ctx, id.TenantID, s.idgen.Generate(), courseID, parentID, id.AccountID, html.EscapeString(req.Content))
	if err != nil {
		return PostDTO{}, apperr.ErrDiscussionInvalid.WithCause(err)
	}
	return row, nil
}

// LikePost 增加讨论帖点赞数。
func (s *Service) LikePost(ctx context.Context, postID int64) (PostDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return PostDTO{}, apperr.ErrUnauthorized
	}
	row, err := s.repo.incrementPostLike(ctx, postID, id.IsPlatform, id.AccountID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return PostDTO{}, ae
		}
		return PostDTO{}, apperr.ErrDiscussionLikeInvalid.WithCause(err)
	}
	return row, nil
}

// PinPost 切换讨论帖置顶。
func (s *Service) PinPost(ctx context.Context, postID int64) (PostDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return PostDTO{}, apperr.ErrUnauthorized
	}
	row, err := s.repo.togglePostPin(ctx, postID, id.IsPlatform, id.AccountID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return PostDTO{}, ae
		}
		return PostDTO{}, apperr.ErrDiscussionModerationInvalid.WithCause(err)
	}
	return row, nil
}

// DeletePost 软删违规讨论帖。
func (s *Service) DeletePost(ctx context.Context, postID int64) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if err := s.repo.softDeletePost(ctx, postID, id.IsPlatform, id.AccountID); err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrDiscussionModerationInvalid.WithCause(err)
	}
	return nil
}

// ListAnnouncements 查询课程公告。
func (s *Service) ListAnnouncements(ctx context.Context, courseID int64, page, size int) ([]AnnouncementDTO, error) {
	if err := s.ensureCourseAccessible(ctx, courseID); err != nil {
		return nil, err
	}
	page, size = pagex.Normalize(page, size)
	rows, err := s.repo.listAnnouncements(ctx, courseID, size, (page-1)*size)
	if err != nil {
		return nil, apperr.ErrAnnouncementQueryFailed.WithCause(err)
	}
	return rows, nil
}

// CreateAnnouncement 创建课程公告。
func (s *Service) CreateAnnouncement(ctx context.Context, courseID int64, req AnnouncementRequest) (AnnouncementDTO, error) {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return AnnouncementDTO{}, apperr.ErrAnnouncementInvalid
	}
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return AnnouncementDTO{}, err
	}
	id, _ := tenantFromContext(ctx)
	row, err := s.repo.createAnnouncement(ctx, id.TenantID, s.idgen.Generate(), courseID, req.Title, html.EscapeString(req.Content))
	if err != nil {
		return AnnouncementDTO{}, apperr.ErrAnnouncementInvalid.WithCause(err)
	}
	return row, nil
}

// PinAnnouncement 切换公告置顶。
func (s *Service) PinAnnouncement(ctx context.Context, announcementID int64) (AnnouncementDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return AnnouncementDTO{}, apperr.ErrUnauthorized
	}
	row, err := s.repo.toggleAnnouncementPin(ctx, announcementID, id.IsPlatform, id.AccountID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return AnnouncementDTO{}, ae
		}
		return AnnouncementDTO{}, apperr.ErrAnnouncementModerationInvalid.WithCause(err)
	}
	return row, nil
}

// ReviewCourse 写入学生课程评价。
func (s *Service) ReviewCourse(ctx context.Context, courseID int64, req ReviewRequest) (map[string]any, error) {
	if req.Rating < 1 || req.Rating > 5 {
		return nil, apperr.ErrReviewInvalid
	}
	tenantID, studentID, err := s.ensureStudentCourseMember(ctx, courseID, apperr.ErrReviewForbidden)
	if err != nil {
		return nil, err
	}
	if err := s.repo.upsertCourseReview(ctx, s.idgen.Generate(), tenantID, courseID, studentID, req.Rating, html.EscapeString(req.Comment)); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrReviewForbidden.WithCause(err)
	}
	return map[string]any{"saved": true}, nil
}
