// 访问学业预警与学生确认状态。
package grade

import (
	"context"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"
)

// CreateAcademicWarning 首次创建预警；已有范围仅更新详情，保留确认和投递状态。
func (t *txStore) CreateAcademicWarning(ctx context.Context, id, tenantID, studentID, semesterID int64, typ int16, detail map[string]any) (WarningDTO, error) {
	data, err := jsonx.ObjectBytes(detail, apperr.ErrGradeWarningInvalid)
	if err != nil {
		return WarningDTO{}, err
	}
	row, err := t.q.CreateAcademicWarning(ctx, sqlcgen.CreateAcademicWarningParams{ID: id, TenantID: tenantID, StudentID: studentID, SemesterID: semesterID, Type: typ, Detail: data, NotifyTraceID: response.TraceFromContext(ctx)})
	if err != nil {
		return WarningDTO{}, err
	}
	return warningDTO(row)
}

// CountAcademicWarningsByStatus 按状态聚合学业预警(套用学生与学期筛选,不套用状态)。
func (t *txStore) CountAcademicWarningsByStatus(ctx context.Context, studentID, semesterID int64) (pagex.FacetGroup, error) {
	rows, err := t.q.CountAcademicWarningsByStatus(ctx, sqlcgen.CountAcademicWarningsByStatusParams{StudentID: studentID, SemesterID: semesterID})
	if err != nil {
		return nil, err
	}
	group := pagex.FacetGroup{}
	for _, row := range rows {
		group.Add(row.Status, row.Count)
	}
	return group, nil
}

// ListAcademicWarnings 查询学业预警分页列表和总数。
// status、semesterID 传 0 表示不按该维度过滤,与 SQL 中的可选条件一致。
func (t *txStore) ListAcademicWarnings(ctx context.Context, studentID int64, status int16, semesterID int64, page, size int) ([]WarningDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListAcademicWarnings(ctx, sqlcgen.ListAcademicWarningsParams{StudentID: studentID, Status: status, SemesterID: semesterID, PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]WarningDTO, 0, len(rows))
	for _, row := range rows {
		item, err := warningDTO(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	total, err := t.q.CountAcademicWarnings(ctx, sqlcgen.CountAcademicWarningsParams{StudentID: studentID, Status: status, SemesterID: semesterID})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// AckAcademicWarning 确认学业预警。
func (t *txStore) AckAcademicWarning(ctx context.Context, id, studentID int64) (WarningDTO, error) {
	row, err := t.q.AckAcademicWarning(ctx, sqlcgen.AckAcademicWarningParams{ID: id, StudentID: studentID})
	if err != nil {
		return WarningDTO{}, err
	}
	return warningDTO(row)
}
