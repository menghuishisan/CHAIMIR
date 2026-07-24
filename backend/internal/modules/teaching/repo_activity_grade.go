// teaching repo_activity_grade 文件封装进度、互动、评价和成绩数据访问。
package teaching

import (
	"context"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// UpsertProgress 写入或累计学习进度。
func (s *txStore) UpsertProgress(ctx context.Context, progress LessonProgress) (LessonProgress, error) {
	row, err := s.q.UpsertLessonProgress(ctx, sqlcgen.UpsertLessonProgressParams{ID: progress.ID, TenantID: progress.TenantID, LessonID: progress.LessonID, StudentID: progress.StudentID, Status: progress.Status, VideoPos: pgtypex.Int4(progress.VideoPos), DurationSec: progress.DurationSec})
	if err != nil {
		return LessonProgress{}, err
	}
	return progressFromRow(row), nil
}

// GetProgress 读取单课时进度。
func (s *txStore) GetProgress(ctx context.Context, tenantID, lessonID, studentID int64) (LessonProgress, error) {
	row, err := s.q.GetLessonProgress(ctx, sqlcgen.GetLessonProgressParams{TenantID: tenantID, LessonID: lessonID, StudentID: studentID})
	if err != nil {
		return LessonProgress{}, err
	}
	return progressFromRow(row), nil
}

// ListProgressByCourse 查询课程全部进度。
func (s *txStore) ListProgressByCourse(ctx context.Context, tenantID, courseID int64) ([]LessonProgress, error) {
	rows, err := s.q.ListProgressByCourse(ctx, sqlcgen.ListProgressByCourseParams{TenantID: tenantID, CourseID: courseID})
	if err != nil {
		return nil, err
	}
	out := make([]LessonProgress, 0, len(rows))
	for _, row := range rows {
		out = append(out, progressFromRow(row))
	}
	return out, nil
}

// ListStudentProgressByCourse 查询学生课程进度。
func (s *txStore) ListStudentProgressByCourse(ctx context.Context, tenantID, courseID, studentID int64) ([]LessonProgress, error) {
	rows, err := s.q.ListStudentProgressByCourse(ctx, sqlcgen.ListStudentProgressByCourseParams{TenantID: tenantID, CourseID: courseID, StudentID: studentID})
	if err != nil {
		return nil, err
	}
	out := make([]LessonProgress, 0, len(rows))
	for _, row := range rows {
		out = append(out, progressFromRow(row))
	}
	return out, nil
}

// ListStudentExperimentLessonIDs 查询学生已加入课程中引用指定实验的课时。
func (s *txStore) ListStudentExperimentLessonIDs(ctx context.Context, tenantID, experimentID, studentID int64) ([]int64, error) {
	return s.q.ListStudentExperimentLessonIDs(ctx, sqlcgen.ListStudentExperimentLessonIDsParams{TenantID: tenantID, Column2: ids.Format(experimentID), StudentID: studentID})
}

// UpsertExperimentScoreProjection 幂等保存 M7 实例最后发布的成绩事件。
func (s *txStore) UpsertExperimentScoreProjection(ctx context.Context, event contracts.ExperimentScoredEvent) error {
	score, err := pgtypex.NumericScale(event.Score, 2)
	if err != nil {
		return err
	}
	return s.q.UpsertExperimentScoreProjection(ctx, sqlcgen.UpsertExperimentScoreProjectionParams{InstanceID: event.InstanceID, TenantID: event.TenantID, ExperimentID: event.ExperimentID, StudentID: event.StudentID, Score: score, ScoredAt: timex.RequiredTimestamptz(event.ScoredAt)})
}

// ListBestExperimentScores 返回指定实验每名学生的最高实例得分。
func (s *txStore) ListBestExperimentScores(ctx context.Context, tenantID, experimentID int64) (map[int64]float64, error) {
	rows, err := s.q.ListBestExperimentScores(ctx, sqlcgen.ListBestExperimentScoresParams{TenantID: tenantID, ExperimentID: experimentID})
	if err != nil {
		return nil, err
	}
	out := make(map[int64]float64, len(rows))
	for _, row := range rows {
		out[row.StudentID] = row.Score
	}
	return out, nil
}

// CreatePost 创建讨论帖或回复。
func (s *txStore) CreatePost(ctx context.Context, post DiscussionPost) (DiscussionPost, error) {
	row, err := s.q.CreateDiscussionPost(ctx, sqlcgen.CreateDiscussionPostParams{ID: post.ID, TenantID: post.TenantID, CourseID: post.CourseID, ParentID: pgtypex.Int8(post.ParentID), AuthorID: post.AuthorID, Content: post.Content})
	if err != nil {
		return DiscussionPost{}, err
	}
	return postFromRow(row), nil
}

// GetPost 读取单条讨论帖或回复。
func (s *txStore) GetPost(ctx context.Context, tenantID, id int64) (DiscussionPost, error) {
	row, err := s.q.GetDiscussionPost(ctx, sqlcgen.GetDiscussionPostParams{TenantID: tenantID, ID: id})
	if err != nil {
		return DiscussionPost{}, err
	}
	return postFromRow(row), nil
}

// ListPosts 查询课程讨论和总数。
func (s *txStore) ListPosts(ctx context.Context, tenantID, courseID int64, page, size int) ([]DiscussionPost, int64, error) {
	rows, err := s.q.ListDiscussionPosts(ctx, sqlcgen.ListDiscussionPostsParams{TenantID: tenantID, CourseID: courseID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, err
	}
	out := make([]DiscussionPost, 0, len(rows))
	for _, row := range rows {
		out = append(out, postFromRow(row))
	}
	total, err := s.q.CountDiscussionPosts(ctx, sqlcgen.CountDiscussionPostsParams{TenantID: tenantID, CourseID: courseID})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// LikePost 增加讨论点赞数。
func (s *txStore) LikePost(ctx context.Context, tenantID, id int64) (DiscussionPost, error) {
	row, err := s.q.LikeDiscussionPost(ctx, sqlcgen.LikeDiscussionPostParams{TenantID: tenantID, ID: id})
	if err != nil {
		return DiscussionPost{}, err
	}
	return postFromRow(row), nil
}

// PinPost 设置讨论置顶。
func (s *txStore) PinPost(ctx context.Context, tenantID, id int64, pinned bool) (DiscussionPost, error) {
	row, err := s.q.PinDiscussionPost(ctx, sqlcgen.PinDiscussionPostParams{TenantID: tenantID, ID: id, IsPinned: pinned})
	if err != nil {
		return DiscussionPost{}, err
	}
	return postFromRow(row), nil
}

// DeletePost 软删讨论。
func (s *txStore) DeletePost(ctx context.Context, tenantID, id int64) (DiscussionPost, error) {
	row, err := s.q.SoftDeleteDiscussionPost(ctx, sqlcgen.SoftDeleteDiscussionPostParams{TenantID: tenantID, ID: id})
	if err != nil {
		return DiscussionPost{}, err
	}
	return postFromRow(row), nil
}

// CreateAnnouncement 创建课程公告。
func (s *txStore) CreateAnnouncement(ctx context.Context, item Announcement) (Announcement, error) {
	row, err := s.q.CreateAnnouncement(ctx, sqlcgen.CreateAnnouncementParams{ID: item.ID, TenantID: item.TenantID, CourseID: item.CourseID, Title: item.Title, Content: item.Content, IsPinned: item.IsPinned})
	if err != nil {
		return Announcement{}, err
	}
	return announcementFromRow(row), nil
}

// ListAnnouncements 查询课程公告。
func (s *txStore) ListAnnouncements(ctx context.Context, tenantID, courseID int64) ([]Announcement, error) {
	rows, err := s.q.ListAnnouncements(ctx, sqlcgen.ListAnnouncementsParams{TenantID: tenantID, CourseID: courseID})
	if err != nil {
		return nil, err
	}
	out := make([]Announcement, 0, len(rows))
	for _, row := range rows {
		out = append(out, announcementFromRow(row))
	}
	return out, nil
}

// PinAnnouncement 设置公告置顶。
func (s *txStore) PinAnnouncement(ctx context.Context, tenantID, id int64, pinned bool) (Announcement, error) {
	row, err := s.q.PinAnnouncement(ctx, sqlcgen.PinAnnouncementParams{TenantID: tenantID, ID: id, IsPinned: pinned})
	if err != nil {
		return Announcement{}, err
	}
	return announcementFromRow(row), nil
}

// UpsertReview 创建或更新课程评价。
func (s *txStore) UpsertReview(ctx context.Context, review CourseReview) (CourseReview, error) {
	row, err := s.q.UpsertCourseReview(ctx, sqlcgen.UpsertCourseReviewParams{ID: review.ID, TenantID: review.TenantID, CourseID: review.CourseID, StudentID: review.StudentID, Rating: review.Rating, Comment: pgtypex.Text(review.Comment)})
	if err != nil {
		return CourseReview{}, err
	}
	return reviewFromRow(row), nil
}

// ReplaceGradeWeights 覆盖课程成绩权重。
func (s *txStore) ReplaceGradeWeights(ctx context.Context, tenantID, courseID int64, weights []GradeWeight) ([]GradeWeight, error) {
	if err := s.q.DeleteGradeWeights(ctx, sqlcgen.DeleteGradeWeightsParams{TenantID: tenantID, CourseID: courseID}); err != nil {
		return nil, err
	}
	out := make([]GradeWeight, 0, len(weights))
	for _, weight := range weights {
		row, err := s.q.CreateGradeWeight(ctx, sqlcgen.CreateGradeWeightParams{ID: weight.ID, TenantID: tenantID, CourseID: courseID, SourceType: weight.SourceType, SourceRef: weight.SourceRef, Column6: formatNumber(weight.Weight)})
		if err != nil {
			return nil, err
		}
		out = append(out, weightFromCreateRow(row))
	}
	return out, nil
}

// ListGradeWeights 查询课程成绩权重。
func (s *txStore) ListGradeWeights(ctx context.Context, tenantID, courseID int64) ([]GradeWeight, error) {
	rows, err := s.q.ListGradeWeights(ctx, sqlcgen.ListGradeWeightsParams{TenantID: tenantID, CourseID: courseID})
	if err != nil {
		return nil, err
	}
	out := make([]GradeWeight, 0, len(rows))
	for _, row := range rows {
		out = append(out, weightFromListRow(row))
	}
	return out, nil
}

// UpsertCourseGrade 写入或更新自动成绩。
func (s *txStore) UpsertCourseGrade(ctx context.Context, grade CourseGrade) (CourseGrade, error) {
	row, err := s.q.UpsertCourseGrade(ctx, sqlcgen.UpsertCourseGradeParams{ID: grade.ID, TenantID: grade.TenantID, CourseID: grade.CourseID, StudentID: grade.StudentID, Column5: formatNumber(grade.AutoTotal)})
	if err != nil {
		return CourseGrade{}, err
	}
	return gradeFromUpsertRow(row), nil
}

// GetCourseGrade 读取单个学生课程成绩。
func (s *txStore) GetCourseGrade(ctx context.Context, tenantID, courseID, studentID int64) (CourseGrade, error) {
	row, err := s.q.GetCourseGrade(ctx, sqlcgen.GetCourseGradeParams{TenantID: tenantID, CourseID: courseID, StudentID: studentID})
	if err != nil {
		return CourseGrade{}, err
	}
	return gradeFromGetRow(row), nil
}

// ListCourseGrades 查询单课程成绩。
func (s *txStore) ListCourseGrades(ctx context.Context, tenantID, courseID int64, limit, offset int32) ([]CourseGrade, error) {
	rows, err := s.q.ListCourseGrades(ctx, sqlcgen.ListCourseGradesParams{TenantID: tenantID, CourseID: courseID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	out := make([]CourseGrade, 0, len(rows))
	for _, row := range rows {
		out = append(out, gradeFromListRow(row))
	}
	return out, nil
}

// ListStudentGrades 查询学生所有课程成绩。
func (s *txStore) ListStudentGrades(ctx context.Context, tenantID, studentID int64) ([]CourseGrade, error) {
	rows, err := s.q.ListStudentGrades(ctx, sqlcgen.ListStudentGradesParams{TenantID: tenantID, StudentID: studentID})
	if err != nil {
		return nil, err
	}
	out := make([]CourseGrade, 0, len(rows))
	for _, row := range rows {
		out = append(out, gradeFromStudentRow(row))
	}
	return out, nil
}

// OverrideCourseGrade 手动覆盖单课程成绩。
func (s *txStore) OverrideCourseGrade(ctx context.Context, tenantID, courseID, studentID int64, total float64) (CourseGrade, error) {
	row, err := s.q.OverrideCourseGrade(ctx, sqlcgen.OverrideCourseGradeParams{TenantID: tenantID, CourseID: courseID, StudentID: studentID, Column4: formatNumber(total)})
	if err != nil {
		return CourseGrade{}, err
	}
	return gradeFromOverrideRow(row), nil
}

// SetCourseGradesLock 同步课程成绩写保护投影。
func (s *txStore) SetCourseGradesLock(ctx context.Context, tenantID, courseID int64, locked bool) error {
	return s.q.SetCourseGradesLock(ctx, sqlcgen.SetCourseGradesLockParams{TenantID: tenantID, CourseID: courseID, IsLocked: locked})
}

// CreateTeachingGradeEventOutbox 在成绩写入事务内保存成绩变更事件。
func (s *txStore) CreateTeachingGradeEventOutbox(ctx context.Context, id, tenantID, courseID, studentID int64, traceID string, updatedAt time.Time) (TeachingGradeEventOutbox, error) {
	row, err := s.q.CreateTeachingGradeEventOutbox(ctx, sqlcgen.CreateTeachingGradeEventOutboxParams{ID: id, TenantID: tenantID, CourseID: courseID, StudentID: studentID, TraceID: traceID, EventUpdatedAt: timex.RequiredTimestamptz(updatedAt)})
	if err != nil {
		return TeachingGradeEventOutbox{}, err
	}
	return teachingGradeEventOutbox(row), nil
}

// ClaimPendingTeachingGradeEventOutbox 跨租户领取待发布或失败待重试的成绩事件。
func (s *txStore) ClaimPendingTeachingGradeEventOutbox(ctx context.Context, limit int32, staleBefore time.Time) ([]TeachingGradeEventOutbox, error) {
	rows, err := s.q.ClaimPendingTeachingGradeEventOutbox(ctx, sqlcgen.ClaimPendingTeachingGradeEventOutboxParams{StaleBefore: timex.RequiredTimestamptz(staleBefore), PageLimit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]TeachingGradeEventOutbox, 0, len(rows))
	for _, row := range rows {
		out = append(out, teachingGradeEventOutbox(row))
	}
	return out, nil
}

// MarkTeachingGradeEventOutboxPublished 标记成绩事件发布成功。
func (s *txStore) MarkTeachingGradeEventOutboxPublished(ctx context.Context, tenantID, id int64) (TeachingGradeEventOutbox, error) {
	row, err := s.q.MarkTeachingGradeEventOutboxPublished(ctx, sqlcgen.MarkTeachingGradeEventOutboxPublishedParams{TenantID: tenantID, ID: id})
	if err != nil {
		return TeachingGradeEventOutbox{}, err
	}
	return teachingGradeEventOutbox(row), nil
}

// MarkTeachingGradeEventOutboxFailed 标记成绩事件发布失败并保留脱敏原因。
func (s *txStore) MarkTeachingGradeEventOutboxFailed(ctx context.Context, tenantID, id int64, reason string) (TeachingGradeEventOutbox, error) {
	row, err := s.q.MarkTeachingGradeEventOutboxFailed(ctx, sqlcgen.MarkTeachingGradeEventOutboxFailedParams{TenantID: tenantID, ID: id, LastError: pgtypex.Text(reason)})
	if err != nil {
		return TeachingGradeEventOutbox{}, err
	}
	return teachingGradeEventOutbox(row), nil
}

// Stats 查询教学统计摘要。
func (s *txStore) Stats(ctx context.Context, tenantID int64) (contractsStats, error) {
	row, err := s.q.TeachingStats(ctx, tenantID)
	if err != nil {
		return contractsStats{}, err
	}
	return contractsStats{CourseCount: row.CourseCount, ActiveCourseCount: row.ActiveCourseCount, LearningDurationSec: row.LearningDurationSec}, nil
}
