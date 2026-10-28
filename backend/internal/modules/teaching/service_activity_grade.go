// teaching service_activity_grade 文件实现进度、互动、评价和单课程成绩业务。
package teaching

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/transfer"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/logging"

	"github.com/xuri/excelize/v2"
)

// ReportProgress 上报课时学习进度。
func (s *Service) ReportProgress(ctx context.Context, lessonID int64, req ProgressRequest) (ProgressDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ProgressDTO{}, err
	}
	req, err = validateProgressRequest(req)
	if err != nil {
		return ProgressDTO{}, err
	}
	var progress LessonProgress
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		lesson, err := tx.GetLesson(ctx, id.TenantID, lessonID)
		if err != nil {
			return err
		}
		chapter, err := tx.GetChapter(ctx, id.TenantID, lesson.ChapterID)
		if err != nil {
			return err
		}
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, chapter.CourseID, id.AccountID); err != nil {
			return err
		}
		progress, err = tx.UpsertProgress(ctx, LessonProgress{ID: s.ids.Generate(), TenantID: id.TenantID, LessonID: lessonID, StudentID: id.AccountID, Status: req.Status, VideoPos: req.VideoPos, DurationSec: req.DurationSec})
		return err
	}); err != nil {
		return ProgressDTO{}, mapCourseError(err)
	}
	return progressDTO(progress), nil
}

// CourseProgressStats 统计课程进度。
func (s *Service) CourseProgressStats(ctx context.Context, courseID int64) (ProgressStatsDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ProgressStatsDTO{}, err
	}
	var stats ProgressStatsDTO
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		_, memberTotal, err := tx.ListCourseMembers(ctx, id.TenantID, courseID, 1, 1)
		if err != nil {
			return err
		}
		lessons, err := tx.ListLessonsByCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		progresses, err := tx.ListProgressByCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		stats.CourseID = ids.ID(courseID)
		stats.MemberCount = memberTotal
		stats.LessonCount = int64(len(lessons))
		for _, progress := range progresses {
			stats.LearningDurationSec += int64(progress.DurationSec)
			if progress.Status == ProgressDone {
				stats.CompletedCount++
			}
		}
		return nil
	}); err != nil {
		return ProgressStatsDTO{}, mapCourseError(err)
	}
	return stats, nil
}

// MyProgress 查询本人课程进度概览。
func (s *Service) MyProgress(ctx context.Context, courseID int64) ([]ProgressDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var progresses []LessonProgress
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		var err error
		progresses, err = tx.ListStudentProgressByCourse(ctx, id.TenantID, courseID, id.AccountID)
		return err
	}); err != nil {
		return nil, mapCourseError(err)
	}
	out := make([]ProgressDTO, 0, len(progresses))
	for _, progress := range progresses {
		out = append(out, progressDTO(progress))
	}
	return out, nil
}

// CreatePost 创建讨论帖或回复。
func (s *Service) CreatePost(ctx context.Context, courseID int64, req PostRequest) (PostDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return PostDTO{}, err
	}
	req, err = validatePostRequest(req)
	if err != nil {
		return PostDTO{}, err
	}
	var post DiscussionPost
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		// 父帖在同一事务内加锁读取,隔离课程并阻止校验与创建之间被删除。
		var parentContent *string
		if req.ParentID > 0 {
			parent, err := tx.GetPost(ctx, id.TenantID, req.ParentID.Int64())
			if err != nil {
				return apperr.ErrTeachingDiscussionInvalid.WithCause(err)
			}
			if parent.CourseID != courseID {
				return apperr.ErrTeachingDiscussionInvalid
			}
			parentContent = &parent.Content
		}
		post, err = tx.CreatePost(ctx, DiscussionPost{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, ParentID: req.ParentID.Int64(), AuthorID: id.AccountID, Content: req.Content})
		post.ParentContent = parentContent
		return err
	}); err != nil {
		return PostDTO{}, mapCourseError(err)
	}
	return postDTO(post), nil
}

// ListPosts 查询课程讨论。
func (s *Service) ListPosts(ctx context.Context, courseID int64, page, size int) ([]PostDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	var posts []DiscussionPost
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		var err error
		posts, total, err = tx.ListPosts(ctx, id.TenantID, courseID, page, size)
		return err
	}); err != nil {
		return nil, 0, 0, 0, mapCourseError(err)
	}
	out := make([]PostDTO, 0, len(posts))
	for _, post := range posts {
		out = append(out, postDTO(post))
	}
	return out, total, page, size, nil
}

// LikePost 点赞讨论。
func (s *Service) LikePost(ctx context.Context, postID int64) (PostDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return PostDTO{}, err
	}
	var post DiscussionPost
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetPost(ctx, id.TenantID, postID)
		if err != nil {
			return err
		}
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, current.CourseID, id.AccountID); err != nil {
			return err
		}
		post, err = tx.LikePost(ctx, id.TenantID, postID)
		return err
	}); err != nil {
		return PostDTO{}, apperr.ErrTeachingDiscussionInvalid.WithCause(err)
	}
	return postDTO(post), nil
}

// PinPost 设置讨论置顶。
func (s *Service) PinPost(ctx context.Context, postID int64, pinned bool) (PostDTO, error) {
	return s.teacherPinPost(ctx, postID, pinned)
}

// DeletePost 删除讨论。
func (s *Service) DeletePost(ctx context.Context, postID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	return s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		post, err := tx.GetPost(ctx, id.TenantID, postID)
		if err != nil {
			return apperr.ErrTeachingDiscussionInvalid.WithCause(err)
		}
		course, err := tx.GetCourse(ctx, id.TenantID, post.CourseID)
		if err != nil {
			return mapCourseError(err)
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		_, err = tx.DeletePost(ctx, id.TenantID, postID)
		return err
	})
}

// teacherPinPost 校验教师后置顶讨论。
func (s *Service) teacherPinPost(ctx context.Context, postID int64, pinned bool) (PostDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return PostDTO{}, err
	}
	var post DiscussionPost
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetPost(ctx, id.TenantID, postID)
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
		post, err = tx.PinPost(ctx, id.TenantID, postID, pinned)
		return err
	}); err != nil {
		return PostDTO{}, apperr.ErrTeachingDiscussionInvalid.WithCause(err)
	}
	return postDTO(post), nil
}

// CreateAnnouncement 创建课程公告。
func (s *Service) CreateAnnouncement(ctx context.Context, courseID int64, req AnnouncementRequest) (AnnouncementDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return AnnouncementDTO{}, err
	}
	req, err = validateAnnouncementRequest(req)
	if err != nil {
		return AnnouncementDTO{}, err
	}
	var item Announcement
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		item, err = tx.CreateAnnouncement(ctx, Announcement{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, Title: req.Title, Content: req.Content, IsPinned: req.IsPinned})
		return err
	}); err != nil {
		return AnnouncementDTO{}, mapCourseError(err)
	}
	return announcementDTO(item), nil
}

// ListAnnouncements 查询课程公告。
func (s *Service) ListAnnouncements(ctx context.Context, courseID int64, page, size int) ([]AnnouncementDTO, int64, int, int, error) {
	page, size = pagex.Normalize(page, size)
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, page, size, err
	}
	var items []Announcement
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		var err error
		items, total, err = tx.ListAnnouncements(ctx, id.TenantID, courseID, page, size)
		return err
	}); err != nil {
		return nil, 0, page, size, mapCourseError(err)
	}
	out := make([]AnnouncementDTO, 0, len(items))
	for _, item := range items {
		out = append(out, announcementDTO(item))
	}
	return out, total, page, size, nil
}

// PinAnnouncement 设置公告置顶。
func (s *Service) PinAnnouncement(ctx context.Context, announcementID int64, pinned bool) (AnnouncementDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return AnnouncementDTO{}, err
	}
	var item Announcement
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		item, err = tx.PinAnnouncement(ctx, id.TenantID, announcementID, pinned)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, item.CourseID)
		if err != nil {
			return err
		}
		return ensureTeacherOwned(course, id.AccountID)
	}); err != nil {
		return AnnouncementDTO{}, apperr.ErrTeachingDiscussionInvalid.WithCause(err)
	}
	return announcementDTO(item), nil
}

// ensureCourseReviewAccess对评价读写复用同一成员和课程结束门禁。
func ensureCourseReviewAccess(ctx context.Context, tx TxStore, tenantID, courseID, studentID int64) error {
	if _, err := tx.GetCourseMember(ctx, tenantID, courseID, studentID); err != nil {
		return apperr.ErrTeachingCourseForbidden.WithCause(err)
	}
	course, err := tx.GetCourse(ctx, tenantID, courseID)
	if err != nil {
		return err
	}
	if course.Status != CourseStatusEnded && course.Status != CourseStatusArchived {
		return apperr.ErrTeachingCourseStateInvalid
	}
	return nil
}

// GetMyCourseReview读取当前成员本人已保存的评价,未评价返回明确空值。
func (s *Service) GetMyCourseReview(ctx context.Context, courseID int64) (MyCourseReviewDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return MyCourseReviewDTO{}, err
	}
	var result MyCourseReviewDTO
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := ensureCourseReviewAccess(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		review, err := tx.GetReview(ctx, id.TenantID, courseID, id.AccountID)
		if isNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		dto := reviewDTO(review)
		result.Review = &dto
		return nil
	}); err != nil {
		return MyCourseReviewDTO{}, mapCourseError(err)
	}
	return result, nil
}

// ReviewCourse 创建或更新课程评价。
func (s *Service) ReviewCourse(ctx context.Context, courseID int64, req ReviewRequest) (ReviewDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	req, err = validateReviewRequest(req)
	if err != nil {
		return ReviewDTO{}, err
	}
	var review CourseReview
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := ensureCourseReviewAccess(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		review, err = tx.UpsertReview(ctx, CourseReview{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, StudentID: id.AccountID, Rating: req.Rating, Comment: req.Comment})
		return err
	}); err != nil {
		return ReviewDTO{}, mapCourseError(err)
	}
	return reviewDTO(review), nil
}

// SetGradeWeights 覆盖课程成绩权重。
func (s *Service) SetGradeWeights(ctx context.Context, courseID int64, req GradeWeightRequest) ([]GradeWeightDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	req, err = validateGradeWeightRequest(req)
	if err != nil {
		return nil, err
	}
	weights := make([]GradeWeight, 0, len(req.Items))
	for _, item := range req.Items {
		weights = append(weights, GradeWeight{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, SourceType: item.SourceType, SourceRef: item.SourceRef, Weight: item.Weight})
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		if err := tx.EnsureCourseGradesWritable(ctx, id.TenantID, courseID); err != nil {
			return err
		}
		if _, err := validateCourseGradeWeights(ctx, tx, id.TenantID, courseID, weights); err != nil {
			return err
		}
		weights, err = tx.ReplaceGradeWeights(ctx, id.TenantID, courseID, weights)
		return err
	}); err != nil {
		return nil, mapGradeError(err)
	}
	out := make([]GradeWeightDTO, 0, len(weights))
	for _, weight := range weights {
		out = append(out, gradeWeightDTO(weight))
	}
	return out, nil
}

// validateCourseGradeWeights 保存和计算共用课程有效来源，拒绝失效对象及非法满分。
func validateCourseGradeWeights(ctx context.Context, tx TxStore, tenantID, courseID int64, weights []GradeWeight) (map[string]CourseGradeSource, error) {
	items := make([]GradeWeightInput, 0, len(weights))
	for _, weight := range weights {
		items = append(items, GradeWeightInput{SourceType: weight.SourceType, SourceRef: weight.SourceRef, Weight: weight.Weight})
	}
	req, err := validateGradeWeightRequest(GradeWeightRequest{Items: items})
	if err != nil {
		return nil, err
	}
	sources, err := tx.ListCourseGradeSources(ctx, tenantID, courseID)
	if err != nil {
		return nil, err
	}
	available := make(map[string]CourseGradeSource, len(sources))
	for _, source := range sources {
		if !validGradeScore(source.MaxScore) || source.MaxScore <= 0 {
			return nil, apperr.ErrTeachingGradeWeightInvalid
		}
		available[fmt.Sprintf("%d:%s", source.SourceType, source.SourceRef.String())] = source
	}
	for _, item := range req.Items {
		if _, ok := available[fmt.Sprintf("%d:%s", item.SourceType, item.SourceRef)]; !ok {
			return nil, apperr.ErrTeachingGradeWeightInvalid.WithCause(fmt.Errorf("课程成绩来源不可用: course_id=%d source_type=%d source_ref=%s", courseID, item.SourceType, item.SourceRef))
		}
	}
	return available, nil
}

// ListGradeWeights 查询课程成绩权重。
func (s *Service) ListGradeWeights(ctx context.Context, courseID int64) ([]GradeWeightDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var weights []GradeWeight
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		var err error
		weights, err = tx.ListGradeWeights(ctx, id.TenantID, courseID)
		return err
	}); err != nil {
		return nil, mapGradeError(err)
	}
	out := make([]GradeWeightDTO, 0, len(weights))
	for _, weight := range weights {
		out = append(out, gradeWeightDTO(weight))
	}
	return out, nil
}

// ComputeCourseGrades 按权重计算全班单课程成绩。
func (s *Service) ComputeCourseGrades(ctx context.Context, courseID int64) ([]GradeDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var grades []CourseGrade
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		if err := tx.EnsureCourseGradesWritable(ctx, id.TenantID, courseID); err != nil {
			return err
		}
		weights, err := tx.ListGradeWeights(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		// 在读取提交和写入成绩前检查整份配置,避免空配置或失效来源覆盖全班成绩。
		sources, err := validateCourseGradeWeights(ctx, tx, id.TenantID, courseID, weights)
		if err != nil {
			return err
		}
		scores := make(map[int64]float64)
		for _, weight := range weights {
			source := sources[fmt.Sprintf("%d:%s", weight.SourceType, weight.SourceRef)]
			sourceScores, err := tx.ListGradeSourceScores(ctx, id.TenantID, source.SourceRef.Int64(), weight.SourceType)
			if err != nil {
				return err
			}
			for _, item := range sourceScores {
				if item.Pending {
					return apperr.ErrTeachingGradePending.WithCause(fmt.Errorf("课程来源仍待评分: course_id=%d source_type=%d source_id=%s student_id=%d", courseID, source.SourceType, source.SourceRef.String(), item.StudentID))
				}
				if !validGradeScore(item.Score) {
					return apperr.ErrTeachingGradeInvalid
				}
				normalized := item.Score
				if weight.SourceType != GradeSourceExperiment {
					normalized = item.Score * 100 / source.MaxScore
				}
				if normalized > 100 {
					return apperr.ErrTeachingGradeInvalid.WithCause(fmt.Errorf("课程来源得分超过满分: course_id=%d source_id=%s student_id=%d", courseID, source.SourceRef.String(), item.StudentID))
				}
				scores[item.StudentID] += normalized * weight.Weight / 100
			}
		}
		grades = make([]CourseGrade, 0)
		// 全部成员均在本事务内写入成绩及 outbox,任一页失败都不得提交部分结果。
		for page := 1; ; page++ {
			members, total, err := tx.ListCourseMembers(ctx, id.TenantID, courseID, page, pagex.MaximumSize())
			if err != nil {
				return err
			}
			for _, member := range members {
				grade, err := tx.UpsertCourseGrade(ctx, CourseGrade{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, StudentID: member.StudentID, AutoTotal: scores[member.StudentID], Credits: course.Credits})
				if err != nil {
					return err
				}
				if err := s.enqueueTeachingGradeEventOutbox(ctx, tx, grade.TenantID, grade.CourseID, grade.StudentID); err != nil {
					return err
				}
				grade.Credits = course.Credits
				grades = append(grades, grade)
			}
			if int64(page)*int64(pagex.MaximumSize()) >= total || len(members) == 0 {
				break
			}
		}
		return nil
	}); err != nil {
		return nil, mapGradeError(err)
	}
	out := make([]GradeDTO, 0, len(grades))
	for _, grade := range grades {
		out = append(out, gradeDTO(grade))
	}
	s.drainTeachingGradeEventOutboxBestEffort(ctx)
	return out, nil
}

// ListGrades 查询课程成绩。
func (s *Service) ListGrades(ctx context.Context, courseID int64, page, size int) ([]GradeDTO, int64, int, int, error) {
	page, size = pagex.Normalize(page, size)
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, page, size, err
	}
	var grades []CourseGrade
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		limit, offset := pagex.LimitOffset(page, size)
		grades, err = tx.ListCourseGrades(ctx, id.TenantID, courseID, limit, offset)
		if err == nil {
			total, err = tx.CountCourseGrades(ctx, id.TenantID, courseID)
		}
		return err
	}); err != nil {
		return nil, 0, page, size, mapGradeError(err)
	}
	out := make([]GradeDTO, 0, len(grades))
	for _, grade := range grades {
		out = append(out, gradeDTO(grade))
	}
	return out, total, page, size, nil
}

// OverrideGrade 手动调整单课程成绩。
func (s *Service) OverrideGrade(ctx context.Context, courseID, studentID int64, req OverrideGradeRequest) (GradeDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return GradeDTO{}, err
	}
	req, err = validateGradeOverrideRequest(req)
	if err != nil {
		return GradeDTO{}, err
	}
	var grade CourseGrade
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		if err := tx.EnsureCourseGradesWritable(ctx, id.TenantID, courseID); err != nil {
			return err
		}
		grade, err = tx.OverrideCourseGrade(ctx, id.TenantID, courseID, studentID, req.Total)
		if err == nil {
			grade.Credits = course.Credits
		}
		if err != nil {
			return err
		}
		return s.enqueueTeachingGradeEventOutbox(ctx, tx, id.TenantID, courseID, studentID)
	}); err != nil {
		return GradeDTO{}, mapGradeError(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "teaching.grade.override", auditTargetGrade, grade.ID, map[string]any{"course_id": courseID, "student_id": studentID, "total": req.Total}); err != nil {
		return GradeDTO{}, err
	}
	s.drainTeachingGradeEventOutboxBestEffort(ctx)
	return gradeDTO(grade), nil
}

const gradeExportSubject = "teaching.course_grade_export"

// ExportGrades 创建可重放的课程成绩导出任务，产物由 M6 worker 异步生成。
func (s *Service) ExportGrades(ctx context.Context, courseID int64) (transfer.TaskDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return transfer.TaskDTO{}, err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return transfer.TaskDTO{}, mapGradeError(err)
	}
	fileName := "course-" + ids.Format(courseID) + "-grades.xlsx"
	task, err := s.transfers.CreateTask(ctx, transfer.NewTaskRequest{
		TenantID:    id.TenantID,
		AccountID:   id.AccountID,
		Channel:     transfer.ChannelExport,
		Subject:     gradeExportSubject,
		FileName:    fileName,
		ContentType: upload.XLSXContentType,
	})
	if err != nil {
		return transfer.TaskDTO{}, apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.CreateCourseGradeExportRequest(ctx, CourseGradeExportRequest{TransferTaskID: task.TaskID, TenantID: id.TenantID, AccountID: id.AccountID, CourseID: courseID})
		return err
	}); err != nil {
		if cleanupErr := s.transfers.DeletePendingTask(ctx, id.TenantID, task.TaskID); cleanupErr != nil {
			logging.ErrorContext(ctx, "teaching grade export task compensation failed", cleanupErr.Error(), slog.Int64("tenant_id", id.TenantID), slog.Int64("course_id", courseID), slog.Int64("transfer_task_id", task.TaskID))
		}
		return transfer.TaskDTO{}, apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	return exportTaskDTO(task), nil
}

// RunGradeExportOnce 执行一次 M6 成绩导出请求扫描，供统一 background runner 调用。
func (s *Service) RunGradeExportOnce(ctx context.Context) error {
	limit, ok := intx.Int32(s.cfg.GradeExportWorkerBatchSize)
	if !ok || limit <= 0 {
		return apperr.ErrTeachingGradeExportFailed
	}
	var requests []CourseGradeExportRequest
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		requests, err = tx.ListDueCourseGradeExportRequests(ctx, timex.Now(), limit)
		return err
	}); err != nil {
		return apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	for _, req := range requests {
		if err := s.runGradeExportRequest(ctx, req); err != nil {
			logging.ErrorContext(ctx, "teaching grade export worker item failed", err.Error(), slog.Int64("tenant_id", req.TenantID), slog.Int64("course_id", req.CourseID), slog.Int64("transfer_task_id", req.TransferTaskID))
		}
	}
	return nil
}

// runGradeExportRequest claims one request's transfer task and produces its XLSX artifact.
func (s *Service) runGradeExportRequest(ctx context.Context, req CourseGradeExportRequest) error {
	task, err := s.transfers.ClaimTask(ctx, req.TenantID, req.TransferTaskID)
	if err != nil {
		return s.reconcileGradeExportRequest(ctx, req)
	}
	data, err := s.buildGradeExportXLSX(ctx, req)
	if err == nil {
		artifactFileName, nameErr := transfer.LeaseArtifactFileName(task.FileName, task.LeaseToken)
		plan, planErr := s.files.PlanUpload(ctx, storage.PlanUploadRequest{TenantID: req.TenantID, AccountID: req.AccountID, Module: "transfer", ResourceType: string(transfer.ChannelExport), ResourceID: ids.Format(task.TaskID), FileName: artifactFileName, ContentType: upload.XLSXContentType, Size: int64(len(data)), ExpectedBucket: s.storage.BucketReport(), AllowedFileName: true, Content: data, KindValidator: func(fileName, contentType string, content []byte) bool {
			return upload.CSVOrXLSXKind(fileName, contentType, content) == upload.KindXLSX
		}})
		if nameErr != nil {
			planErr = nameErr
		}
		if planErr == nil {
			planErr = s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(data), int64(len(data)), upload.XLSXContentType)
		}
		if planErr == nil {
			_, planErr = s.transfers.CompleteTask(ctx, task, transfer.CompleteTaskRequest{ObjectRef: plan.ObjectRef, Size: int64(len(data))})
			if planErr != nil {
				if cleanupErr := s.storage.Delete(ctx, plan.Bucket, plan.Key); cleanupErr != nil {
					logging.ErrorContext(ctx, "teaching grade export artifact cleanup failed", cleanupErr.Error(), slog.Int64("tenant_id", req.TenantID), slog.Int64("transfer_task_id", req.TransferTaskID))
				}
			}
		}
		err = planErr
	}
	if err != nil {
		return s.failGradeExportRequest(ctx, req, task, err)
	}
	return s.deleteGradeExportRequest(ctx, req)
}

// buildGradeExportXLSX reads the authorized course grades and encodes the immutable worker artifact.
func (s *Service) buildGradeExportXLSX(ctx context.Context, req CourseGradeExportRequest) ([]byte, error) {
	var grades []CourseGrade
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, req.TenantID, req.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, req.AccountID); err != nil {
			return err
		}
		grades, err = s.listCourseGradesForExport(ctx, tx, req.TenantID, req.CourseID)
		return err
	}); err != nil {
		return nil, err
	}
	f := excelize.NewFile()
	defer logging.CloseContext(ctx, "关闭课程成绩导出工作簿失败", f)
	sheet := "成绩"
	index, err := f.NewSheet(sheet)
	if err != nil {
		return nil, apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	f.SetActiveSheet(index)
	headers := []string{"course_id", "student_id", "auto_total", "override_total", "final_total", "is_overridden", "is_locked"}
	for i, header := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, apperr.ErrTeachingGradeExportFailed.WithCause(err)
		}
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return nil, apperr.ErrTeachingGradeExportFailed.WithCause(err)
		}
	}
	for r, grade := range grades {
		values := []any{grade.CourseID, grade.StudentID, grade.AutoTotal, "", finalTotal(grade), grade.IsOverridden, grade.IsLocked}
		if grade.IsOverridden {
			values[3] = grade.OverrideTotal
		}
		for c, value := range values {
			cell, err := excelize.CoordinatesToCellName(c+1, r+2)
			if err != nil {
				return nil, apperr.ErrTeachingGradeExportFailed.WithCause(err)
			}
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return nil, apperr.ErrTeachingGradeExportFailed.WithCause(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	return buf.Bytes(), nil
}

// reconcileGradeExportRequest clears terminal requests or waits until their task becomes claimable.
func (s *Service) reconcileGradeExportRequest(ctx context.Context, req CourseGradeExportRequest) error {
	task, err := s.transfers.GetTask(ctx, req.TenantID, req.TransferTaskID)
	if err != nil {
		if errors.Is(err, apperr.ErrTransferTaskNotFound) {
			return s.deleteGradeExportRequest(ctx, req)
		}
		return s.deferGradeExportRequest(ctx, req, timex.Now().Add(time.Duration(s.cfg.GradeExportWorkerPollMs)*time.Millisecond))
	}
	switch task.Status {
	case transfer.StatusSucceeded, transfer.StatusFailed:
		return s.deleteGradeExportRequest(ctx, req)
	case transfer.StatusRetrying:
		return s.deferGradeExportRequest(ctx, req, task.NextAttemptAfter)
	case transfer.StatusRunning:
		return s.deferGradeExportRequest(ctx, req, task.LeaseUntil)
	default:
		return s.deferGradeExportRequest(ctx, req, timex.Now().Add(time.Duration(s.cfg.GradeExportWorkerPollMs)*time.Millisecond))
	}
}

// failGradeExportRequest advances the leased task's retry state and synchronizes its next module scan.
func (s *Service) failGradeExportRequest(ctx context.Context, req CourseGradeExportRequest, task transfer.Task, cause error) error {
	failed, err := s.transfers.FailTask(ctx, task, cause, timex.Now())
	if err != nil {
		return s.deferGradeExportRequest(ctx, req, timex.Now().Add(time.Duration(s.cfg.GradeExportWorkerPollMs)*time.Millisecond))
	}
	if failed.Status == transfer.StatusFailed {
		return s.deleteGradeExportRequest(ctx, req)
	}
	return s.deferGradeExportRequest(ctx, req, failed.NextAttemptAfter)
}

// deferGradeExportRequest schedules the next attempt in the request table owned by M6.
func (s *Service) deferGradeExportRequest(ctx context.Context, req CourseGradeExportRequest, next time.Time) error {
	if next.IsZero() || !next.After(timex.Now()) {
		next = timex.Now().Add(time.Duration(s.cfg.GradeExportWorkerPollMs) * time.Millisecond)
	}
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.SetCourseGradeExportRequestNextCheck(ctx, req.TenantID, req.TransferTaskID, next)
		return err
	})
}

// deleteGradeExportRequest removes a terminal M6 request after its transfer task is durable.
func (s *Service) deleteGradeExportRequest(ctx context.Context, req CourseGradeExportRequest) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		return tx.DeleteCourseGradeExportRequest(ctx, req.TenantID, req.TransferTaskID)
	})
}

// HandleGradeLockChanged 处理 M11 驱动的写保护投影事件。
func (s *Service) HandleGradeLockChanged(ctx context.Context, event contracts.GradeReviewLockChangedEvent) error {
	if event.TenantID <= 0 || event.CourseID <= 0 || event.ReviewID <= 0 || event.Revision <= 0 {
		return apperr.ErrTeachingGradeInvalid
	}
	return s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		return tx.SetCourseGradesLock(ctx, event.TenantID, event.CourseID, event.Locked, event.Revision)
	})
}

// listCourseGradesForTenant 按租户读取单课程成绩。
func (s *Service) listCourseGradesForTenant(ctx context.Context, tenantID, courseID int64) ([]CourseGrade, error) {
	var grades []CourseGrade
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		maxRows, ok := intx.Int32(s.cfg.CourseGradesMaxRows)
		if !ok || maxRows <= 0 {
			return apperr.ErrTeachingGradeInvalid
		}
		grades, err = tx.ListCourseGrades(ctx, tenantID, courseID, maxRows, 0)
		return err
	}); err != nil {
		return nil, mapGradeError(err)
	}
	return grades, nil
}

// listCourseGradesForExport 按导出批量配置分批读取课程成绩。
func (s *Service) listCourseGradesForExport(ctx context.Context, tx TxStore, tenantID, courseID int64) ([]CourseGrade, error) {
	batchSize, ok := intx.Int32(s.cfg.GradeExportBatchSize)
	if !ok || batchSize <= 0 {
		return nil, apperr.ErrTeachingGradeExportFailed
	}
	out := make([]CourseGrade, 0, int(batchSize))
	for offset := int32(0); ; offset += batchSize {
		batch, err := tx.ListCourseGrades(ctx, tenantID, courseID, batchSize, offset)
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if len(batch) < int(batchSize) {
			return out, nil
		}
	}
}

// publishGradeUpdated 在作业提交回写后持久化成绩更新事件。
func (s *Service) publishGradeUpdated(ctx context.Context, tenantID, assignmentID, studentID int64) error {
	var courseID int64
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		assignment, err := tx.GetAssignment(ctx, tenantID, assignmentID)
		if err != nil {
			return err
		}
		courseID = assignment.CourseID
		return s.enqueueTeachingGradeEventOutbox(ctx, tx, tenantID, courseID, studentID)
	}); err != nil {
		return mapAssignmentError(err)
	}
	s.drainTeachingGradeEventOutboxBestEffort(ctx)
	return nil
}

// enqueueTeachingGradeEventOutbox 在成绩写入同一事务内保存 M11 消费的成绩事件。
func (s *Service) enqueueTeachingGradeEventOutbox(ctx context.Context, tx TxStore, tenantID, courseID, studentID int64) error {
	traceID := strings.TrimSpace(response.TraceFromContext(ctx))
	if tenantID <= 0 || courseID <= 0 || studentID <= 0 || traceID == "" {
		return apperr.ErrTeachingGradeEventPublishFailed
	}
	// 与课程写入共用行锁,事件捕获本次改分实际看到的解锁修订。
	if err := tx.EnsureCourseGradesWritable(ctx, tenantID, courseID); err != nil {
		return err
	}
	if _, err := tx.CreateTeachingGradeEventOutbox(ctx, s.ids.Generate(), tenantID, courseID, studentID, traceID, timex.Now()); err != nil {
		return apperr.ErrTeachingGradeEventPublishFailed.WithCause(err)
	}
	return nil
}

// RunTeachingGradeEventOutboxOnce 领取并发布 M6 成绩变更事件。
func (s *Service) RunTeachingGradeEventOutboxOnce(ctx context.Context) error {
	limit, ok := intx.Int32(s.cfg.GradeEventOutboxBatchSize)
	if !ok || limit <= 0 {
		return apperr.ErrTeachingGradeEventPublishFailed
	}
	staleBefore := timex.Now()
	var items []TeachingGradeEventOutbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		leaseToken, tokenErr := pkgcrypto.RandomToken(48)
		if tokenErr != nil {
			return tokenErr
		}
		leaseUntil := staleBefore.Add(time.Duration(s.cfg.GradeEventOutboxStaleMs) * time.Millisecond)
		items, err = tx.ClaimPendingTeachingGradeEventOutbox(ctx, limit, staleBefore, leaseToken, leaseUntil)
		if err != nil {
			return apperr.ErrTeachingGradeEventPublishFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	var firstErr error
	for _, item := range items {
		if err := s.publishGradeEventOutboxItem(ctx, item); err != nil {
			logging.ErrorContext(ctx, "teaching grade outbox publish failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("course_id", item.CourseID), slog.Int64("student_id", item.StudentID), slog.Int64("outbox_id", item.ID))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// publishGradeEventOutboxItem 发布单条成绩事件并按结果回写 outbox 状态。
func (s *Service) publishGradeEventOutboxItem(ctx context.Context, item TeachingGradeEventOutbox) error {
	if item.Revision <= 0 || item.LockRevision < 0 || item.EventUpdatedAt.IsZero() {
		return apperr.ErrTeachingGradeEventPublishFailed.WithCause(fmt.Errorf("成绩事件缺少权威修订或原始时间: outbox_id=%d", item.ID))
	}
	eventCtx := response.WithTrace(ctx, item.TraceID)
	payload := contracts.TeachingGradeUpdatedEvent{Revision: item.Revision, LockRevision: item.LockRevision, TenantID: item.TenantID, TraceID: item.TraceID, CourseID: item.CourseID, StudentID: item.StudentID, UpdatedAt: item.EventUpdatedAt}
	if err := s.bus.Publish(eventCtx, contracts.SubjectTeachingGradeUpdated, payload); err != nil {
		s.recordTeachingGradeEventOutboxFailure(eventCtx, item, err)
		return apperr.ErrTeachingGradeEventPublishFailed.WithCause(err)
	}
	return s.markTeachingGradeEventOutboxPublished(eventCtx, item)
}

// markTeachingGradeEventOutboxPublished 标记成绩事件发布成功。
func (s *Service) markTeachingGradeEventOutboxPublished(ctx context.Context, item TeachingGradeEventOutbox) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkTeachingGradeEventOutboxPublished(ctx, item.TenantID, item.ID, item.LeaseToken)
		if err != nil {
			return apperr.ErrTeachingGradeEventPublishFailed.WithCause(err)
		}
		return nil
	})
}

// recordTeachingGradeEventOutboxFailure 记录成绩事件发布失败并等待后台重试。
func (s *Service) recordTeachingGradeEventOutboxFailure(ctx context.Context, item TeachingGradeEventOutbox, cause error) {
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkTeachingGradeEventOutboxFailed(ctx, item.TenantID, item.ID, logging.SanitizeError(cause.Error()), item.LeaseToken)
		return err
	}); err != nil {
		logging.ErrorContext(ctx, "teaching grade event outbox failure mark failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("course_id", item.CourseID), slog.Int64("student_id", item.StudentID), slog.Int64("outbox_id", item.ID))
	}
}

// drainTeachingGradeEventOutboxBestEffort 在请求提交后尽快投递,失败交给后台任务补偿。
func (s *Service) drainTeachingGradeEventOutboxBestEffort(ctx context.Context) {
	if err := s.RunTeachingGradeEventOutboxOnce(ctx); err != nil {
		logging.ErrorContext(ctx, "teaching grade event outbox drain failed", err.Error())
	}
}
