// teaching service_activity_grade 文件实现进度、互动、评价和单课程成绩业务。
package teaching

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/transfer"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"

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
		stats.CourseID = courseID
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
		post, err = tx.CreatePost(ctx, DiscussionPost{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, ParentID: req.ParentID, AuthorID: id.AccountID, Content: req.Content})
		return err
	}); err != nil {
		return PostDTO{}, mapCourseError(err)
	}
	return postDTO(post), nil
}

// ListPosts 查询课程讨论。
func (s *Service) ListPosts(ctx context.Context, courseID int64, page, size int) ([]PostDTO, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	var posts []DiscussionPost
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		var err error
		posts, err = tx.ListPosts(ctx, id.TenantID, courseID, page, size)
		return err
	}); err != nil {
		return nil, 0, 0, mapCourseError(err)
	}
	out := make([]PostDTO, 0, len(posts))
	for _, post := range posts {
		out = append(out, postDTO(post))
	}
	return out, page, size, nil
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
func (s *Service) ListAnnouncements(ctx context.Context, courseID int64) ([]AnnouncementDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var items []Announcement
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensureCourseReadable(ctx, tx, id.TenantID, courseID, id.AccountID); err != nil {
			return err
		}
		var err error
		items, err = tx.ListAnnouncements(ctx, id.TenantID, courseID)
		return err
	}); err != nil {
		return nil, mapCourseError(err)
	}
	out := make([]AnnouncementDTO, 0, len(items))
	for _, item := range items {
		out = append(out, announcementDTO(item))
	}
	return out, nil
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
		if _, err := tx.GetCourseMember(ctx, id.TenantID, courseID, id.AccountID); err != nil {
			return apperr.ErrTeachingCourseForbidden
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
		weights, err := tx.ListGradeWeights(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		members, _, err := tx.ListCourseMembers(ctx, id.TenantID, courseID, 1, s.cfg.CourseGradesMaxRows)
		if err != nil {
			return err
		}
		scores := map[int64]float64{}
		for _, weight := range weights {
			if weight.SourceType != GradeSourceAssignment {
				continue
			}
			assignmentID, err := strconv.ParseInt(weight.SourceRef, 10, 64)
			if err != nil {
				return apperr.ErrTeachingGradeWeightInvalid
			}
			subs, _, err := tx.ListSubmissionsByAssignment(ctx, id.TenantID, assignmentID, 1, s.cfg.CourseGradesMaxRows)
			if err != nil {
				return err
			}
			best := map[int64]int32{}
			for _, sub := range subs {
				if sub.FinalScore > best[sub.StudentID] {
					best[sub.StudentID] = sub.FinalScore
				}
			}
			for studentID, score := range best {
				scores[studentID] += float64(score) * weight.Weight / 100
			}
		}
		grades = make([]CourseGrade, 0, len(members))
		for _, member := range members {
			grade, err := tx.UpsertCourseGrade(ctx, CourseGrade{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: courseID, StudentID: member.StudentID, AutoTotal: scores[member.StudentID], Credits: course.Credits})
			if err != nil {
				return err
			}
			grade.Credits = course.Credits
			grades = append(grades, grade)
		}
		return nil
	}); err != nil {
		return nil, mapGradeError(err)
	}
	out := make([]GradeDTO, 0, len(grades))
	for _, grade := range grades {
		out = append(out, gradeDTO(grade))
		if err := s.publishTeachingGradeUpdated(ctx, grade.TenantID, grade.CourseID, grade.StudentID); err != nil {
			return nil, apperr.ErrTeachingGradeInvalid.WithCause(err)
		}
	}
	return out, nil
}

// ListGrades 查询课程成绩。
func (s *Service) ListGrades(ctx context.Context, courseID int64) ([]GradeDTO, error) {
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
		grades, err = tx.ListCourseGrades(ctx, id.TenantID, courseID, int32(s.cfg.CourseGradesMaxRows), 0)
		return err
	}); err != nil {
		return nil, mapGradeError(err)
	}
	out := make([]GradeDTO, 0, len(grades))
	for _, grade := range grades {
		out = append(out, gradeDTO(grade))
	}
	return out, nil
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
		grade, err = tx.OverrideCourseGrade(ctx, id.TenantID, courseID, studentID, req.Total)
		if err == nil {
			grade.Credits = course.Credits
		}
		return err
	}); err != nil {
		return GradeDTO{}, mapGradeError(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "teaching.grade.override", auditTargetGrade, grade.ID, map[string]any{"course_id": courseID, "student_id": studentID, "total": req.Total}); err != nil {
		return GradeDTO{}, err
	}
	if err := s.publishTeachingGradeUpdated(ctx, id.TenantID, courseID, studentID); err != nil {
		return GradeDTO{}, apperr.ErrTeachingGradeInvalid.WithCause(err)
	}
	return gradeDTO(grade), nil
}

const gradeExportSubject = "teaching.course_grade_export"

// ExportGrades 导出课程成绩 Excel,并把产物登记到统一导入导出中心。
func (s *Service) ExportGrades(ctx context.Context, courseID int64) (ExportTaskDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ExportTaskDTO{}, err
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
		grades, err = s.listCourseGradesForExport(ctx, tx, id.TenantID, courseID)
		return err
	}); err != nil {
		return ExportTaskDTO{}, mapGradeError(err)
	}
	fileName := fmt.Sprintf("course-%d-grades.xlsx", courseID)
	task, err := s.transfers.CreateTask(ctx, transfer.NewTaskRequest{
		TenantID:    id.TenantID,
		AccountID:   id.AccountID,
		Channel:     transfer.ChannelExport,
		Subject:     gradeExportSubject,
		FileName:    fileName,
		ContentType: upload.XLSXContentType,
	})
	if err != nil {
		return ExportTaskDTO{}, apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	f := excelize.NewFile()
	defer f.Close()
	sheet := "成绩"
	index, err := f.NewSheet(sheet)
	if err != nil {
		return ExportTaskDTO{}, apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	f.SetActiveSheet(index)
	headers := []string{"course_id", "student_id", "auto_total", "override_total", "final_total", "is_overridden", "is_locked"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return ExportTaskDTO{}, apperr.ErrTeachingGradeExportFailed.WithCause(err)
		}
	}
	for r, grade := range grades {
		values := []any{grade.CourseID, grade.StudentID, grade.AutoTotal, "", finalTotal(grade), grade.IsOverridden, grade.IsLocked}
		if grade.IsOverridden {
			values[3] = grade.OverrideTotal
		}
		for c, value := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return ExportTaskDTO{}, apperr.ErrTeachingGradeExportFailed.WithCause(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return ExportTaskDTO{}, apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	data := buf.Bytes()
	plan, err := s.files.PlanUpload(storage.PlanUploadRequest{
		TenantID:        id.TenantID,
		AccountID:       id.AccountID,
		Module:          "transfer",
		ResourceType:    string(transfer.ChannelExport),
		ResourceID:      strconv.FormatInt(task.TaskID, 10),
		FileName:        fileName,
		ContentType:     upload.XLSXContentType,
		Size:            int64(len(data)),
		ExpectedBucket:  s.storage.BucketReport(),
		AllowedFileName: true,
		Content:         data,
		KindValidator: func(fileName, contentType string, content []byte) bool {
			return upload.CSVOrXLSXKind(fileName, contentType, content) == upload.KindXLSX
		},
	})
	if err != nil {
		return ExportTaskDTO{}, apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(data), int64(len(data)), upload.XLSXContentType); err != nil {
		return ExportTaskDTO{}, apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	completed, err := s.transfers.CompleteTask(ctx, id.TenantID, task.TaskID, transfer.CompleteTaskRequest{ObjectRef: plan.ObjectRef, Size: int64(len(data))})
	if err != nil {
		return ExportTaskDTO{}, apperr.ErrTeachingGradeExportFailed.WithCause(err)
	}
	return exportTaskDTO(completed), nil
}

// ListCourseGrades 实现 M6 对 M11 的只读成绩契约。
func (s *Service) ListCourseGrades(ctx context.Context, tenantID, courseID int64) ([]contracts.TeachingCourseGrade, error) {
	grades, err := s.listCourseGradesForTenant(ctx, tenantID, courseID)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.TeachingCourseGrade, 0, len(grades))
	for _, grade := range grades {
		out = append(out, contractGrade(grade))
	}
	return out, nil
}

// ListStudentGrades 实现 M6 对 M11 的学生成绩只读契约。
func (s *Service) ListStudentGrades(ctx context.Context, tenantID, studentID int64) ([]contracts.TeachingCourseGrade, error) {
	var grades []CourseGrade
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		grades, err = tx.ListStudentGrades(ctx, tenantID, studentID)
		return err
	}); err != nil {
		return nil, mapGradeError(err)
	}
	out := make([]contracts.TeachingCourseGrade, 0, len(grades))
	for _, grade := range grades {
		out = append(out, contractGrade(grade))
	}
	return out, nil
}

// Stats 实现 M6 对 M9 的教学统计只读契约。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.TeachingStats, error) {
	var stats contracts.TeachingStats
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		got, err := tx.Stats(ctx, tenantID)
		if err != nil {
			return err
		}
		stats = contracts.TeachingStats{TenantID: tenantID, CourseCount: got.CourseCount, ActiveCourseCount: got.ActiveCourseCount, LearningDurationSec: got.LearningDurationSec}
		return nil
	}); err != nil {
		return contracts.TeachingStats{}, mapCourseError(err)
	}
	return stats, nil
}

// HandleGradeLockChanged 处理 M11 驱动的写保护投影事件。
func (s *Service) HandleGradeLockChanged(ctx context.Context, event contracts.GradeReviewLockChangedEvent) error {
	return s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		return tx.SetCourseGradesLock(ctx, event.TenantID, event.CourseID, event.Locked)
	})
}

// listCourseGradesForTenant 按租户读取单课程成绩。
func (s *Service) listCourseGradesForTenant(ctx context.Context, tenantID, courseID int64) ([]CourseGrade, error) {
	var grades []CourseGrade
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		grades, err = tx.ListCourseGrades(ctx, tenantID, courseID, int32(s.cfg.CourseGradesMaxRows), 0)
		return err
	}); err != nil {
		return nil, mapGradeError(err)
	}
	return grades, nil
}

// listCourseGradesForExport 按导出批量配置分批读取课程成绩。
func (s *Service) listCourseGradesForExport(ctx context.Context, tx TxStore, tenantID, courseID int64) ([]CourseGrade, error) {
	batchSize := int32(s.cfg.GradeExportBatchSize)
	out := make([]CourseGrade, 0, batchSize)
	for offset := int32(0); ; offset += batchSize {
		batch, err := tx.ListCourseGrades(ctx, tenantID, courseID, batchSize, offset)
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if int32(len(batch)) < batchSize {
			return out, nil
		}
	}
}

// publishGradeUpdated 在作业提交回写后发布成绩更新事件。
func (s *Service) publishGradeUpdated(ctx context.Context, tenantID, assignmentID, studentID int64) error {
	var courseID int64
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		assignment, err := tx.GetAssignment(ctx, tenantID, assignmentID)
		if err != nil {
			return err
		}
		courseID = assignment.CourseID
		return nil
	}); err != nil {
		return mapAssignmentError(err)
	}
	return s.publishTeachingGradeUpdated(ctx, tenantID, courseID, studentID)
}

// publishTeachingGradeUpdated 发布 M6 成绩变更事件。
func (s *Service) publishTeachingGradeUpdated(ctx context.Context, tenantID, courseID, studentID int64) error {
	return s.bus.Publish(ctx, contracts.SubjectTeachingGradeUpdated, contracts.TeachingGradeUpdatedEvent{TenantID: tenantID, CourseID: courseID, StudentID: studentID, UpdatedAt: timeNowUTC()})
}

// timeNowUTC 便于事件时间统一走 UTC。
func timeNowUTC() time.Time {
	return timex.Now()
}
