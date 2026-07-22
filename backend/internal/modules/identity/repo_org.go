// identity repo_org 文件封装组织架构表的数据访问和 sqlc 调用。
package identity

import (
	"context"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
)

// CreateDepartment 创建院系记录。
func (t *txStore) CreateDepartment(ctx context.Context, tenantID, id int64, req DepartmentRequest) (Department, error) {
	row, err := t.q.CreateDepartment(ctx, sqlcgen.CreateDepartmentParams{ID: id, TenantID: tenantID, Name: req.Name, Code: pgtypex.Text(req.Code)})
	if err != nil {
		return Department{}, err
	}
	return departmentFromRow(row), nil
}

// ListDepartments 读取当前租户全部未删除院系。
func (t *txStore) ListDepartments(ctx context.Context) ([]Department, error) {
	rows, err := t.q.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Department, 0, len(rows))
	for _, row := range rows {
		out = append(out, departmentFromRow(row))
	}
	return out, nil
}

// DepartmentExists 判断院系是否属于当前租户且未删除。
func (t *txStore) DepartmentExists(ctx context.Context, tenantID, id int64) (bool, error) {
	return t.q.DepartmentExists(ctx, sqlcgen.DepartmentExistsParams{ID: id, TenantID: tenantID})
}

// UpdateDepartment 更新院系名称和短码。
func (t *txStore) UpdateDepartment(ctx context.Context, tenantID, id int64, req DepartmentRequest) (Department, error) {
	row, err := t.q.UpdateDepartment(ctx, sqlcgen.UpdateDepartmentParams{ID: id, TenantID: tenantID, Name: req.Name, Code: pgtypex.Text(req.Code)})
	if err != nil {
		return Department{}, err
	}
	return departmentFromRow(row), nil
}

// DeleteDepartment 软删除院系记录。
func (t *txStore) DeleteDepartment(ctx context.Context, tenantID, id int64) error {
	return t.q.SoftDeleteDepartment(ctx, sqlcgen.SoftDeleteDepartmentParams{ID: id, TenantID: tenantID})
}

// CreateMajor 创建专业记录。
func (t *txStore) CreateMajor(ctx context.Context, tenantID, id int64, req MajorRequest) (Major, error) {
	row, err := t.q.CreateMajor(ctx, sqlcgen.CreateMajorParams{ID: id, TenantID: tenantID, DepartmentID: req.DepartmentID.Int64(), Name: req.Name})
	if err != nil {
		return Major{}, err
	}
	return majorFromRow(row), nil
}

// ListMajors 读取专业列表,departmentID 为 0 时读取全部。
func (t *txStore) ListMajors(ctx context.Context, departmentID int64) ([]Major, error) {
	rows, err := t.q.ListMajors(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	out := make([]Major, 0, len(rows))
	for _, row := range rows {
		out = append(out, majorFromRow(row))
	}
	return out, nil
}

// MajorExists 判断专业是否属于当前租户且未删除。
func (t *txStore) MajorExists(ctx context.Context, tenantID, id int64) (bool, error) {
	return t.q.MajorExists(ctx, sqlcgen.MajorExistsParams{ID: id, TenantID: tenantID})
}

// UpdateMajor 更新专业所属院系和名称。
func (t *txStore) UpdateMajor(ctx context.Context, tenantID, id int64, req MajorRequest) (Major, error) {
	row, err := t.q.UpdateMajor(ctx, sqlcgen.UpdateMajorParams{ID: id, TenantID: tenantID, DepartmentID: req.DepartmentID.Int64(), Name: req.Name})
	if err != nil {
		return Major{}, err
	}
	return majorFromRow(row), nil
}

// DeleteMajor 软删除专业记录。
func (t *txStore) DeleteMajor(ctx context.Context, tenantID, id int64) error {
	return t.q.SoftDeleteMajor(ctx, sqlcgen.SoftDeleteMajorParams{ID: id, TenantID: tenantID})
}

// CreateClass 创建班级记录。
func (t *txStore) CreateClass(ctx context.Context, tenantID, id int64, req ClassRequest) (Class, error) {
	row, err := t.q.CreateClass(ctx, sqlcgen.CreateClassParams{ID: id, TenantID: tenantID, MajorID: req.MajorID.Int64(), Name: req.Name, EnrollmentYear: req.EnrollmentYear, Status: req.Status})
	if err != nil {
		return Class{}, err
	}
	return classFromRow(row), nil
}

// ListClasses 读取班级列表,majorID 为 0 时读取全部。
func (t *txStore) ListClasses(ctx context.Context, majorID int64) ([]Class, error) {
	rows, err := t.q.ListClasses(ctx, majorID)
	if err != nil {
		return nil, err
	}
	out := make([]Class, 0, len(rows))
	for _, row := range rows {
		out = append(out, classFromRow(row))
	}
	return out, nil
}

// ClassExists 判断班级是否属于当前租户且未删除。
func (t *txStore) ClassExists(ctx context.Context, tenantID, id int64) (bool, error) {
	return t.q.ClassExists(ctx, sqlcgen.ClassExistsParams{ID: id, TenantID: tenantID})
}

// UpdateClass 更新班级所属专业、名称、入学年份和状态。
func (t *txStore) UpdateClass(ctx context.Context, tenantID, id int64, req ClassRequest) (Class, error) {
	row, err := t.q.UpdateClass(ctx, sqlcgen.UpdateClassParams{ID: id, TenantID: tenantID, MajorID: req.MajorID.Int64(), Name: req.Name, EnrollmentYear: req.EnrollmentYear, Status: req.Status})
	if err != nil {
		return Class{}, err
	}
	return classFromRow(row), nil
}

// DeleteClass 软删除班级记录。
func (t *txStore) DeleteClass(ctx context.Context, tenantID, id int64) error {
	return t.q.SoftDeleteClass(ctx, sqlcgen.SoftDeleteClassParams{ID: id, TenantID: tenantID})
}

// ArchiveClassesByEnrollmentYear 按入学年份归档班级。
func (t *txStore) ArchiveClassesByEnrollmentYear(ctx context.Context, tenantID int64, enrollmentYear int16) error {
	return t.q.ArchiveClassesByEnrollmentYear(ctx, sqlcgen.ArchiveClassesByEnrollmentYearParams{TenantID: tenantID, EnrollmentYear: enrollmentYear})
}

// ArchiveStudentAccountsByEnrollmentYear 按入学年份归档当前租户正常学生账号。
func (t *txStore) ArchiveStudentAccountsByEnrollmentYear(ctx context.Context, tenantID int64, enrollmentYear int16) error {
	return t.q.ArchiveStudentAccountsByEnrollmentYear(ctx, sqlcgen.ArchiveStudentAccountsByEnrollmentYearParams{TenantID: tenantID, EnrollmentYear: pgtypex.Int2(enrollmentYear)})
}

// RevokeStudentSessionsByEnrollmentYear 吊销按学年归档学生的有效会话。
func (t *txStore) RevokeStudentSessionsByEnrollmentYear(ctx context.Context, tenantID int64, enrollmentYear int16) error {
	return t.q.RevokeStudentSessionsByEnrollmentYear(ctx, sqlcgen.RevokeStudentSessionsByEnrollmentYearParams{TenantID: tenantID, EnrollmentYear: pgtypex.Int2(enrollmentYear)})
}

// PromoteClasses 把明确选择的正常班级更新到目标学年并同步名称中的年份。
func (t *txStore) PromoteClasses(ctx context.Context, tenantID int64, classIDs []int64, targetYear int16) (int64, error) {
	return t.q.PromoteClasses(ctx, sqlcgen.PromoteClassesParams{TenantID: tenantID, ClassIds: classIDs, TargetYear: targetYear})
}

// PromoteClassStudentProfiles 同步所选班级学生档案的入学年份。
func (t *txStore) PromoteClassStudentProfiles(ctx context.Context, tenantID int64, classIDs []int64, targetYear int16) error {
	return t.q.PromoteClassStudentProfiles(ctx, sqlcgen.PromoteClassStudentProfilesParams{TenantID: tenantID, ClassIds: classIDs, TargetYear: pgtypex.Int2(targetYear)})
}
