// grade repo_grade_events 仅访问M11消费游标、申诉及锁事件,不读取M6表。
package grade

import (
	"context"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
)

// LockGradeStudentScope 串行化同一学生跨课程GPA聚合,与课程锁使用独立命名空间。
func (t *txStore) LockGradeStudentScope(ctx context.Context, tenantID, studentID int64) error {
	return t.q.LockGradeStudentScope(ctx, sqlcgen.LockGradeStudentScopeParams{Column1: tenantID, Column2: studentID})
}

// ApplyGradeEventRevision 仅推进更大的权威成绩修订,重复或迟到事件不重新改变状态。
func (t *txStore) ApplyGradeEventRevision(ctx context.Context, tenantID, courseID, studentID, revision int64) (bool, error) {
	_, err := t.q.ApplyGradeEventRevision(ctx, sqlcgen.ApplyGradeEventRevisionParams{TenantID: tenantID, CourseID: courseID, StudentID: studentID, Revision: revision})
	if isNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

// GetLatestGradeLockOutboxByCourse 读取该课程最后提交的权威锁修订。
func (t *txStore) GetLatestGradeLockOutboxByCourse(ctx context.Context, courseID int64) (GradeLockOutbox, error) {
	row, err := t.q.GetLatestGradeLockOutboxByCourse(ctx, courseID)
	if err != nil {
		return GradeLockOutbox{}, err
	}
	return gradeLockOutbox(row), nil
}

// HasAcceptedAppealsByCourse 判断课程是否仍有等待教师改分的已受理申诉。
func (t *txStore) HasAcceptedAppealsByCourse(ctx context.Context, courseID int64) (bool, error) {
	return t.q.HasAcceptedAppealsByCourse(ctx, courseID)
}

// AcceptGradeAppeal 原子受理待处理申诉,只接受同课程已持久化的解锁修订。
func (t *txStore) AcceptGradeAppeal(ctx context.Context, appealID, handlerID int64, comment string, revision int64) (AppealDTO, error) {
	row, err := t.q.AcceptGradeAppeal(ctx, sqlcgen.AcceptGradeAppealParams{ID: appealID, HandlerID: pgtypex.Int8(handlerID), ResultComment: pgtypex.Text(comment), UnlockRevision: pgtypex.Int8(revision)})
	if err != nil {
		return AppealDTO{}, err
	}
	return appealDTO(row), nil
}
