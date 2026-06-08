// M6 进度与互动服务:课时进度、讨论、公告和课程评价。
package teaching

import (
	"context"
	"html"
	"strings"

	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
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
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, progressErr := q.UpsertLessonProgress(ctx, sqlcgen.UpsertLessonProgressParams{
			ID: s.idgen.Generate(), TenantID: tenantID, LessonID: lessonID, StudentID: studentID,
			Status: req.Status, VideoPos: pgtype.Int4{Int32: req.VideoPos, Valid: req.VideoPos > 0}, DurationSec: req.DurationSec,
		})
		if db.IsNoRows(progressErr) {
			return apperr.ErrProgressForbidden
		}
		return progressErr
	}); err != nil {
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
	var rows []sqlcgen.LessonProgress
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListLessonProgressByCourse(ctx, courseID)
		rows = found
		return err
	}); err != nil {
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
	var rows []sqlcgen.LessonProgress
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListLessonProgressByCourseAndStudent(ctx, sqlcgen.ListLessonProgressByCourseAndStudentParams{CourseID: courseID, StudentID: studentID})
		rows = found
		return err
	}); err != nil {
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
	var rows []sqlcgen.DiscussionPost
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListDiscussionPosts(ctx, sqlcgen.ListDiscussionPostsParams{CourseID: courseID, LimitCount: int32(size), OffsetCount: int32((page - 1) * size)})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrDiscussionQueryFailed.WithCause(err)
	}
	return postDTOsFromRows(rows), nil
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
	parentID := mustOptionalID(req.ParentID)
	var row sqlcgen.DiscussionPost
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateDiscussionPost(ctx, sqlcgen.CreateDiscussionPostParams{ID: s.idgen.Generate(), TenantID: id.TenantID, CourseID: courseID, ParentID: pgInt8(parentID), AuthorID: id.AccountID, Content: html.EscapeString(req.Content)})
		return err
	}); err != nil {
		return PostDTO{}, apperr.ErrDiscussionInvalid.WithCause(err)
	}
	return postDTOFromRow(row), nil
}

// LikePost 增加讨论帖点赞数。
func (s *Service) LikePost(ctx context.Context, postID int64) (PostDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return PostDTO{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.DiscussionPost
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.IncrementPostLike(ctx, sqlcgen.IncrementPostLikeParams{ID: postID, IsPlatform: id.IsPlatform, ActorID: id.AccountID})
		if db.IsNoRows(err) {
			return apperr.ErrDiscussionLikeInvalid
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return PostDTO{}, ae
		}
		return PostDTO{}, apperr.ErrDiscussionLikeInvalid.WithCause(err)
	}
	return postDTOFromRow(row), nil
}

// PinPost 切换讨论帖置顶。
func (s *Service) PinPost(ctx context.Context, postID int64) (PostDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return PostDTO{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.DiscussionPost
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.TogglePostPin(ctx, sqlcgen.TogglePostPinParams{ID: postID, IsPlatform: id.IsPlatform, ActorID: id.AccountID})
		if db.IsNoRows(err) {
			return apperr.ErrDiscussionModerationInvalid
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return PostDTO{}, ae
		}
		return PostDTO{}, apperr.ErrDiscussionModerationInvalid.WithCause(err)
	}
	return postDTOFromRow(row), nil
}

// DeletePost 软删违规讨论帖。
func (s *Service) DeletePost(ctx context.Context, postID int64) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, err := q.SoftDeletePost(ctx, sqlcgen.SoftDeletePostParams{ID: postID, IsPlatform: id.IsPlatform, ActorID: id.AccountID})
		if db.IsNoRows(err) {
			return apperr.ErrDiscussionModerationInvalid
		}
		return err
	}); err != nil {
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
	var rows []sqlcgen.Announcement
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListAnnouncements(ctx, sqlcgen.ListAnnouncementsParams{CourseID: courseID, LimitCount: int32(size), OffsetCount: int32((page - 1) * size)})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrAnnouncementQueryFailed.WithCause(err)
	}
	return announcementDTOsFromRows(rows), nil
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
	var row sqlcgen.Announcement
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateAnnouncement(ctx, sqlcgen.CreateAnnouncementParams{ID: s.idgen.Generate(), TenantID: id.TenantID, CourseID: courseID, Title: req.Title, Content: html.EscapeString(req.Content)})
		return err
	}); err != nil {
		return AnnouncementDTO{}, apperr.ErrAnnouncementInvalid.WithCause(err)
	}
	return announcementDTOFromRow(row), nil
}

// PinAnnouncement 切换公告置顶。
func (s *Service) PinAnnouncement(ctx context.Context, announcementID int64) (AnnouncementDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return AnnouncementDTO{}, apperr.ErrUnauthorized
	}
	var row sqlcgen.Announcement
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.ToggleAnnouncementPin(ctx, sqlcgen.ToggleAnnouncementPinParams{ID: announcementID, IsPlatform: id.IsPlatform, ActorID: id.AccountID})
		if db.IsNoRows(err) {
			return apperr.ErrAnnouncementModerationInvalid
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return AnnouncementDTO{}, ae
		}
		return AnnouncementDTO{}, apperr.ErrAnnouncementModerationInvalid.WithCause(err)
	}
	return announcementDTOFromRow(row), nil
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
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, err := q.UpsertCourseReview(ctx, sqlcgen.UpsertCourseReviewParams{ID: s.idgen.Generate(), TenantID: tenantID, CourseID: courseID, StudentID: studentID, Rating: req.Rating, Comment: pgText(html.EscapeString(req.Comment))})
		if db.IsNoRows(err) {
			return apperr.ErrReviewForbidden
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrReviewForbidden.WithCause(err)
	}
	return map[string]any{"saved": true}, nil
}
