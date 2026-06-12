// teaching rules 文件集中实现 M6 输入校验和状态规则。
package teaching

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"strings"
	"time"

	"chaimir/pkg/apperr"
)

// validateCourseRequest 校验课程创建和编辑输入。
func validateCourseRequest(req CourseRequest) (CourseRequest, time.Time, time.Time, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.CoverURL = strings.TrimSpace(req.CoverURL)
	req.Semester = strings.TrimSpace(req.Semester)
	req.StartAt = strings.TrimSpace(req.StartAt)
	req.EndAt = strings.TrimSpace(req.EndAt)
	startAt, startErr := time.Parse(time.RFC3339, req.StartAt)
	endAt, endErr := time.Parse(time.RFC3339, req.EndAt)
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
func validateLessonRequest(req LessonRequest) (LessonRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || req.Sort < 0 || !validLessonContentType(req.ContentType) {
		return LessonRequest{}, apperr.ErrTeachingLessonInvalid
	}
	if req.ContentRef == nil {
		req.ContentRef = map[string]any{}
	}
	if req.ContentType == LessonContentMarkdown {
		req.ContentRef = sanitizeStringMap(req.ContentRef)
	}
	return req, nil
}

// validateAssignmentRequest 校验作业输入。
func validateAssignmentRequest(req AssignmentRequest) (AssignmentRequest, time.Time, error) {
	req.Title = strings.TrimSpace(req.Title)
	due, err := time.Parse(time.RFC3339, strings.TrimSpace(req.DueAt))
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

// validatePostRequest 校验讨论输入。
func validatePostRequest(req PostRequest) (PostRequest, error) {
	req.Content = sanitizeUserText(req.Content)
	if req.Content == "" || req.ParentID < 0 {
		return PostRequest{}, apperr.ErrTeachingDiscussionInvalid
	}
	return req, nil
}

// validateAnnouncementRequest 校验公告输入。
func validateAnnouncementRequest(req AnnouncementRequest) (AnnouncementRequest, error) {
	req.Title = sanitizeUserText(req.Title)
	req.Content = sanitizeUserText(req.Content)
	if req.Title == "" || req.Content == "" {
		return AnnouncementRequest{}, apperr.ErrTeachingDiscussionInvalid
	}
	return req, nil
}

// validateReviewRequest 校验课程评价输入。
func validateReviewRequest(req ReviewRequest) (ReviewRequest, error) {
	req.Comment = sanitizeUserText(req.Comment)
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
		if points < 0 {
			return 0, apperr.ErrTeachingAssignmentInvalid
		}
		return int32(math.Ceil(points)), nil
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
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
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

// normalizePage 将分页限制在平台统一范围。
func normalizePage(page, size *int) {
	if *page <= 0 {
		*page = 1
	}
	if *size <= 0 {
		*size = 20
	}
	if *size > 100 {
		*size = 100
	}
}

// sanitizeUserText 清理用户可见文本中的 HTML 控制字符,防止存储型脚本进入响应。
func sanitizeUserText(value string) string {
	return html.EscapeString(strings.TrimSpace(value))
}

// sanitizeStringMap 递归清理 JSON 对象中的字符串值,用于 Markdown 课时内容。
func sanitizeStringMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = sanitizeJSONValue(value)
	}
	return out
}

// sanitizeJSONValue 清理 JSON 值中的字符串并保留数组与对象结构。
func sanitizeJSONValue(value any) any {
	switch typed := value.(type) {
	case string:
		return sanitizeUserText(typed)
	case map[string]any:
		return sanitizeStringMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeJSONValue(item))
		}
		return out
	default:
		return value
	}
}
