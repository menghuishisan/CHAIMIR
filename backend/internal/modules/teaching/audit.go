// M6 审计写入:统一经 platform/audit 写入 M1 audit_log。
package teaching

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

const (
	auditActionCourseCreate     = "teaching.course.create"
	auditActionCourseUpdate     = "teaching.course.update"
	auditActionCourseStatus     = "teaching.course.status"
	auditActionMemberChange     = "teaching.member.change"
	auditActionContentChange    = "teaching.content.change"
	auditActionAssignmentChange = "teaching.assignment.change"
	auditActionSubmissionGrade  = "teaching.submission.grade"
	auditActionGradeWeight      = "teaching.grade.weight"
	auditActionGradeOverride    = "teaching.grade.override"
	auditTargetCourse           = "teaching.course"
	auditTargetAssignment       = "teaching.assignment"
	auditTargetSubmission       = "teaching.submission"
	auditTargetGrade            = "teaching.grade"
)

// writeAudit 记录成功业务操作审计。
func (s *Service) writeAudit(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	if s.auditor == nil {
		return apperr.ErrTeachingAuditFailed
	}
	actorID, actorRole, err := audit.ResolveActor(ctx, s.identity)
	if err != nil {
		return apperr.ErrTeachingAuditFailed.WithCause(err)
	}
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrTeachingAuditFailed.WithCause(err)
	}
	if err := s.auditor.Write(ctx, entry); err != nil {
		return apperr.ErrTeachingAuditFailed.WithCause(err)
	}
	return nil
}
