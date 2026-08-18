// teaching rules 文件集中实现 M6 输入校验和状态规则。
package teaching

import (
	"fmt"
	"math"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// validateCourseRequest 校验课程创建和编辑输入。
func validateCourseRequest(req CourseRequest) (CourseRequest, time.Time, time.Time, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.CoverRef = strings.TrimSpace(req.CoverRef)
	req.Semester = strings.TrimSpace(req.Semester)
	req.StartAt = strings.TrimSpace(req.StartAt)
	req.EndAt = strings.TrimSpace(req.EndAt)
	startAt, startErr := timex.ParseRFC3339(req.StartAt)
	endAt, endErr := timex.ParseRFC3339(req.EndAt)
	if req.Name == "" || req.Semester == "" || !validCourseType(req.Type) || !validDifficulty(req.Difficulty) || req.Credits < 0 || req.Credits > 99 {
		return CourseRequest{}, time.Time{}, time.Time{}, apperr.ErrTeachingCourseInvalid
	}
	if startErr != nil || endErr != nil || !endAt.After(startAt) {
		return CourseRequest{}, time.Time{}, time.Time{}, apperr.ErrTeachingCourseInvalid
	}
	if req.Schedule == nil {
		req.Schedule = map[string]any{}
	}
	return req, startAt, endAt, nil
}

// validateChapterRequest 校验章节输入。
func validateChapterRequest(req ChapterRequest) (ChapterRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || req.Sort < 0 {
		return ChapterRequest{}, apperr.ErrTeachingChapterInvalid
	}
	return req, nil
}

// validateLessonRequest 校验课时输入。
//
// 正文不做 HTML 实体转义:全站唯一的消费者是 React 文本节点(前端零
// dangerouslySetInnerHTML,后端不产 HTML),转义反而会把学生看到的 `a < b` 变成
// `a &lt; b` —— 区块链课程的正文与讨论必然出现代码片段与比较符号,这是可见缺陷而非防护。
// XSS 防护由渲染层的文本节点语义 + 前端 CSP 承担(M6 安全设计 §6)。
func validateLessonRequest(req LessonRequest) (LessonRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || req.Sort < 0 || !validLessonContentType(req.ContentType) {
		return LessonRequest{}, apperr.ErrTeachingLessonInvalid
	}
	req.ContentRef.Markdown = strings.TrimSpace(req.ContentRef.Markdown)
	req.ContentRef.PackageCode = strings.TrimSpace(req.ContentRef.PackageCode)
	req.ContentRef.Version = strings.TrimSpace(req.ContentRef.Version)
	switch req.ContentType {
	case LessonContentVideo, LessonContentAttachment:
		req.ContentRef = LessonContentRefRequest{}
	case LessonContentMarkdown:
		req.ContentRef.ExperimentID = 0
		req.ContentRef.PackageCode = ""
		req.ContentRef.Version = ""
	case LessonContentExperiment:
		req.ContentRef.Markdown = ""
		req.ContentRef.PackageCode = ""
		req.ContentRef.Version = ""
	case LessonContentSimulation:
		req.ContentRef.Markdown = ""
		req.ContentRef.ExperimentID = 0
	}
	return req, nil
}

// lessonContentRefMap 把已校验的公开课时引用转换为内部 JSONB 固定形状。
func lessonContentRefMap(contentType int16, ref LessonContentRefRequest) map[string]any {
	switch contentType {
	case LessonContentMarkdown:
		return map[string]any{"markdown": ref.Markdown}
	case LessonContentExperiment:
		if ref.ExperimentID <= 0 {
			return map[string]any{}
		}
		return map[string]any{"experiment_id": ref.ExperimentID.String()}
	case LessonContentSimulation:
		return map[string]any{"package_code": ref.PackageCode, "version": ref.Version}
	default:
		return map[string]any{}
	}
}

// validateAssignmentRequest 校验作业输入。
func validateAssignmentRequest(req AssignmentRequest) (AssignmentRequest, time.Time, error) {
	req.Title = strings.TrimSpace(req.Title)
	due, err := timex.ParseRFC3339(req.DueAt)
	if err != nil || req.Title == "" || req.MaxAttempts <= 0 || !validLatePolicy(req.LatePolicy) || len(req.Items) == 0 {
		return AssignmentRequest{}, time.Time{}, apperr.ErrTeachingAssignmentInvalid
	}
	if req.LatePenalty == nil {
		req.LatePenalty = map[string]any{}
	}
	if req.LatePolicy == LatePolicyPenalize && !hasLatePenaltyRule(req.LatePenalty) {
		return AssignmentRequest{}, time.Time{}, apperr.ErrTeachingAssignmentInvalid
	}
	for i := range req.Items {
		req.Items[i].ItemCode = strings.TrimSpace(req.Items[i].ItemCode)
		req.Items[i].ItemVersion = strings.TrimSpace(req.Items[i].ItemVersion)
		req.Items[i].JudgerCode = strings.TrimSpace(req.Items[i].JudgerCode)
		if req.Items[i].ItemCode == "" || req.Items[i].ItemVersion == "" || req.Items[i].Score <= 0 || req.Items[i].Seq <= 0 || !validGradingMode(req.Items[i].GradingMode) {
			return AssignmentRequest{}, time.Time{}, apperr.ErrTeachingAssignmentInvalid
		}
		if req.Items[i].GradingMode == GradingModeAuto && req.Items[i].JudgerCode == "" {
			return AssignmentRequest{}, time.Time{}, apperr.ErrTeachingAssignmentInvalid
		}
	}
	return req, due, nil
}

// validateDraftRequest 校验草稿输入。
func validateDraftRequest(req DraftRequest) (DraftRequest, error) {
	if req.Content == nil {
		return DraftRequest{}, apperr.ErrTeachingDraftInvalid
	}
	return req, nil
}

// validateSubmissionRequest 校验正式提交输入。
func validateSubmissionRequest(req SubmitAssignmentRequest) (SubmitAssignmentRequest, error) {
	if req.ContentRef == nil {
		return SubmitAssignmentRequest{}, apperr.ErrTeachingSubmissionInvalid
	}
	return req, nil
}

// validateGradeRequest 校验教师批改输入。
func validateGradeRequest(req GradeSubmissionRequest) (GradeSubmissionRequest, error) {
	req.Comment = strings.TrimSpace(req.Comment)
	if req.Score < 0 {
		return GradeSubmissionRequest{}, apperr.ErrTeachingSubmissionInvalid
	}
	return req, nil
}

// validateProgressRequest 校验学习进度输入。
func validateProgressRequest(req ProgressRequest) (ProgressRequest, error) {
	if req.Status < ProgressNotStarted || req.Status > ProgressDone || req.VideoPos < 0 || req.DurationSec < 0 {
		return ProgressRequest{}, apperr.ErrTeachingProgressInvalid
	}
	return req, nil
}

// validatePostRequest 校验讨论输入。正文原样存(不做 HTML 转义,理由见 validateLessonRequest)。
func validatePostRequest(req PostRequest) (PostRequest, error) {
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" || req.ParentID < 0 {
		return PostRequest{}, apperr.ErrTeachingDiscussionInvalid
	}
	return req, nil
}

// validateAnnouncementRequest 校验公告输入。
func validateAnnouncementRequest(req AnnouncementRequest) (AnnouncementRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" || req.Content == "" {
		return AnnouncementRequest{}, apperr.ErrTeachingDiscussionInvalid
	}
	return req, nil
}

// validateReviewRequest 校验课程评价输入。
func validateReviewRequest(req ReviewRequest) (ReviewRequest, error) {
	req.Comment = strings.TrimSpace(req.Comment)
	if req.Rating < 1 || req.Rating > 5 {
		return ReviewRequest{}, apperr.ErrTeachingDiscussionInvalid
	}
	return req, nil
}

// validateGradeWeightRequest 校验成绩权重和来源。
func validateGradeWeightRequest(req GradeWeightRequest) (GradeWeightRequest, error) {
	if len(req.Items) == 0 {
		return GradeWeightRequest{}, apperr.ErrTeachingGradeWeightInvalid
	}
	total := 0.0
	seen := map[string]struct{}{}
	for i := range req.Items {
		req.Items[i].SourceRef = strings.TrimSpace(req.Items[i].SourceRef)
		if !validGradeSource(req.Items[i].SourceType) || req.Items[i].SourceRef == "" || req.Items[i].Weight <= 0 {
			return GradeWeightRequest{}, apperr.ErrTeachingGradeWeightInvalid
		}
		// 作业成绩需要回查提交记录,所以 source_ref 必须是作业雪花 ID;
		// 实验和考试的来源由课程配置命名,不应被错误要求为数据库 ID。
		if req.Items[i].SourceType == GradeSourceAssignment {
			if _, ok := ids.Parse(req.Items[i].SourceRef); !ok {
				return GradeWeightRequest{}, apperr.ErrTeachingGradeWeightInvalid
			}
		}
		key := fmt.Sprintf("%d:%s", req.Items[i].SourceType, req.Items[i].SourceRef)
		if _, ok := seen[key]; ok {
			return GradeWeightRequest{}, apperr.ErrTeachingGradeWeightInvalid
		}
		seen[key] = struct{}{}
		total += req.Items[i].Weight
	}
	if math.Abs(total-100) > 0.0001 {
		return GradeWeightRequest{}, apperr.ErrTeachingGradeWeightInvalid
	}
	return req, nil
}

// validateGradeOverrideRequest 校验手动调分输入。
func validateGradeOverrideRequest(req OverrideGradeRequest) (OverrideGradeRequest, error) {
	if req.Total < 0 || req.Total > 100 {
		return OverrideGradeRequest{}, apperr.ErrTeachingGradeInvalid
	}
	return req, nil
}

// ensureTeacherOwned 校验教师是否为课程负责人。
func ensureTeacherOwned(course Course, accountID int64) error {
	if course.TeacherID != accountID {
		return apperr.ErrTeachingCourseForbidden
	}
	return nil
}

// ensureCourseJoinable 校验课程是否允许学生加入。
func ensureCourseJoinable(course Course) error {
	if course.Status != CourseStatusPublished && course.Status != CourseStatusRunning {
		return apperr.ErrTeachingInviteInvalid
	}
	return nil
}

// ensureCanPublishCourse 校验课程发布前置条件。
func ensureCanPublishCourse(course Course, lessonCount int64) error {
	if course.Status != CourseStatusDraft || lessonCount <= 0 {
		return apperr.ErrTeachingCourseStateInvalid
	}
	return nil
}

// ensureCanEndCourse 校验课程结束前置状态。
func ensureCanEndCourse(course Course) error {
	if course.Status != CourseStatusPublished && course.Status != CourseStatusRunning {
		return apperr.ErrTeachingCourseStateInvalid
	}
	return nil
}

// ensureCanArchiveCourse 校验课程归档前置状态。
func ensureCanArchiveCourse(course Course) error {
	if course.Status != CourseStatusEnded {
		return apperr.ErrTeachingCourseStateInvalid
	}
	return nil
}

// ensureCanManageMembers 校验成员管理课程状态。
func ensureCanManageMembers(course Course) error {
	if course.Status != CourseStatusPublished && course.Status != CourseStatusRunning {
		return apperr.ErrTeachingCourseStateInvalid
	}
	return nil
}

// applyLatePolicy 计算迟交状态与初始分。
func applyLatePolicy(assignment Assignment, now time.Time) (bool, error) {
	if now.After(assignment.DueAt) {
		if assignment.LatePolicy == LatePolicyReject {
			return true, apperr.ErrTeachingLateSubmissionRejected
		}
		return true, nil
	}
	return false, nil
}

// applyLatePenalty 根据作业迟交策略计算最终分,保留原始批改分用于追溯。
func applyLatePenalty(assignment Assignment, rawScore int32, isLate bool) (int32, error) {
	if rawScore < 0 {
		return 0, apperr.ErrTeachingSubmissionInvalid
	}
	if !isLate || assignment.LatePolicy == LatePolicyNoPenalty {
		return rawScore, nil
	}
	if assignment.LatePolicy == LatePolicyReject {
		return 0, apperr.ErrTeachingLateSubmissionRejected
	}
	penalty, err := latePenaltyAmount(assignment.LatePenalty, rawScore)
	if err != nil {
		return 0, err
	}
	final := rawScore - penalty
	if final < 0 {
		return 0, nil
	}
	return final, nil
}

// hasLatePenaltyRule 判断迟交扣分策略是否包含可执行规则。
func hasLatePenaltyRule(rule map[string]any) bool {
	_, pointsOK := numericRuleValue(rule, "points")
	_, percentOK := numericRuleValue(rule, "percent")
	return pointsOK || percentOK
}

// latePenaltyAmount 从 JSON 策略解析扣分分值或百分比。
func latePenaltyAmount(rule map[string]any, rawScore int32) (int32, error) {
	if points, ok := numericRuleValue(rule, "points"); ok {
		if math.IsNaN(points) || math.IsInf(points, 0) || points < 0 || points > math.MaxInt32 {
			return 0, apperr.ErrTeachingAssignmentInvalid
		}
		penalty, fits := intx.Int64ToInt32(int64(math.Ceil(points)))
		if !fits {
			return 0, apperr.ErrTeachingAssignmentInvalid
		}
		return penalty, nil
	}
	if percent, ok := numericRuleValue(rule, "percent"); ok {
		if percent < 0 || percent > 100 {
			return 0, apperr.ErrTeachingAssignmentInvalid
		}
		return int32(math.Ceil(float64(rawScore) * percent / 100)), nil
	}
	return 0, apperr.ErrTeachingAssignmentInvalid
}

// numericRuleValue 读取迟交策略中的数值字段。
func numericRuleValue(rule map[string]any, key string) (float64, bool) {
	value, ok := rule[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return jsonx.Float64FromAnyOK(typed)
	}
}

// validCourseType 校验课程类型。
func validCourseType(value int16) bool {
	return value >= CourseTypeTheory && value <= CourseTypeProject
}

// validDifficulty 校验课程难度。
func validDifficulty(value int16) bool {
	return value >= DifficultyIntro && value <= DifficultyResearch
}

// validLessonContentType 校验课时内容形态。
func validLessonContentType(value int16) bool {
	return value >= LessonContentVideo && value <= LessonContentSimulation
}

// validLatePolicy 校验迟交策略。
func validLatePolicy(value int16) bool {
	return value >= LatePolicyReject && value <= LatePolicyNoPenalty
}

// validGradingMode 校验题目评分方式。
func validGradingMode(value int16) bool {
	return value == GradingModeAuto || value == GradingModeManual
}

// validGradeSource 校验成绩来源类型。
func validGradeSource(value int16) bool {
	return value >= GradeSourceAssignment && value <= GradeSourceExam
}
