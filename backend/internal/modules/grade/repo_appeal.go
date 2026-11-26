// 访问成绩申诉及其处理状态。
package grade

import (
	"context"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/pagex"
)

// CreateGradeAppeal 创建成绩申诉。
func (t *txStore) CreateGradeAppeal(ctx context.Context, id, tenantID, studentID int64, req AppealRequest) (AppealDTO, error) {
	row, err := t.q.CreateGradeAppeal(ctx, sqlcgen.CreateGradeAppealParams{ID: id, TenantID: tenantID, StudentID: studentID, CourseID: req.CourseID.Int64(), Reason: req.Reason})
	if err != nil {
		return AppealDTO{}, err
	}
	return appealDTO(row), nil
}

// HasOpenGradeAppeal 判断同一学生同一课程是否已有待处理或受理中的申诉。
func (t *txStore) HasOpenGradeAppeal(ctx context.Context, courseID, studentID int64) (bool, error) {
	return t.q.HasOpenGradeAppeal(ctx, sqlcgen.HasOpenGradeAppealParams{CourseID: courseID, StudentID: studentID})
}

// GetGradeAppeal 查询成绩申诉。
func (t *txStore) GetGradeAppeal(ctx context.Context, id int64) (AppealDTO, error) {
	row, err := t.q.GetGradeAppeal(ctx, id)
	if err != nil {
		return AppealDTO{}, err
	}
	return appealDTO(row), nil
}

// CountGradeAppealsByStatus 按状态聚合申诉,范围同列表(学生本人 / 教师任教课程 / 校管全校)。
func (t *txStore) CountGradeAppealsByStatus(ctx context.Context, query AppealListQuery) (pagex.FacetGroup, error) {
	rows, err := t.q.CountGradeAppealsByStatus(ctx, sqlcgen.CountGradeAppealsByStatusParams{StudentID: query.StudentID, AllCourses: query.AllCourses, CourseIds: query.CourseIDs})
	if err != nil {
		return nil, err
	}
	group := pagex.FacetGroup{}
	for _, row := range rows {
		group.Add(row.Status, row.Count)
	}
	return group, nil
}

// ListGradeAppeals 按同一角色课程范围查询申诉分页及总数,不在分页后过滤。
func (t *txStore) ListGradeAppeals(ctx context.Context, query AppealListQuery) ([]AppealDTO, int64, error) {
	limit, offset := pagex.LimitOffset(query.Page, query.Size)
	if query.StudentID > 0 {
		return t.listStudentGradeAppeals(ctx, query, limit, offset)
	}
	rows, err := t.q.ListGradeAppeals(ctx, sqlcgen.ListGradeAppealsParams{Status: query.Status, AllCourses: query.AllCourses, CourseIds: query.CourseIDs, PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AppealDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, appealDTO(row))
	}
	total, err := t.q.CountGradeAppeals(ctx, sqlcgen.CountGradeAppealsParams{Status: query.Status, AllCourses: query.AllCourses, CourseIds: query.CourseIDs})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// listStudentGradeAppeals 只读一名学生本人的申诉,分页与计数同口径。
func (t *txStore) listStudentGradeAppeals(ctx context.Context, query AppealListQuery, limit, offset int32) ([]AppealDTO, int64, error) {
	rows, err := t.q.ListStudentGradeAppeals(ctx, sqlcgen.ListStudentGradeAppealsParams{StudentID: query.StudentID, Status: query.Status, PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AppealDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, appealDTO(row))
	}
	total, err := t.q.CountStudentGradeAppeals(ctx, sqlcgen.CountStudentGradeAppealsParams{StudentID: query.StudentID, Status: query.Status})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ListAcceptedAppealsByCourseStudent 查询待 M6 改分后收口的申诉。
func (t *txStore) ListAcceptedAppealsByCourseStudent(ctx context.Context, courseID, studentID int64) ([]AppealDTO, error) {
	rows, err := t.q.ListAcceptedAppealsByCourseStudent(ctx, sqlcgen.ListAcceptedAppealsByCourseStudentParams{CourseID: courseID, StudentID: studentID})
	if err != nil {
		return nil, err
	}
	out := make([]AppealDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, appealDTO(row))
	}
	return out, nil
}

// UpdateGradeAppealStatus 按明确前置状态更新申诉,避免重复处理旧状态。
func (t *txStore) UpdateGradeAppealStatus(ctx context.Context, id int64, fromStatus, toStatus int16, handlerID int64, comment string) (AppealDTO, error) {
	row, err := t.q.UpdateGradeAppealStatus(ctx, sqlcgen.UpdateGradeAppealStatusParams{ID: id, FromStatus: fromStatus, ToStatus: toStatus, HandlerID: handlerID, ResultComment: comment})
	if err != nil {
		return AppealDTO{}, err
	}
	return appealDTO(row), nil
}
