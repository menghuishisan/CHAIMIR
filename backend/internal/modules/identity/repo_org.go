// identity repo_org 文件封装组织架构表的数据访问和 sqlc 调用。
package identity

import (
	"context"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/pkg/apperr"
)

// lockOrganizationScope 串行化同租户组织变更与档案挂靠,阻止检查后父级被并发软删除。
func (t *txStore) lockOrganizationScope(ctx context.Context, tenantID int64) error {
	return t.q.LockOrganizationScope(ctx, tenantID)
}

// CreateDepartment 创建院系记录。
func (t *txStore) CreateDepartment(ctx context.Context, tenantID, id int64, req DepartmentRequest) (Department, error) {
	if err := t.lockOrganizationScope(ctx, tenantID); err != nil {
		return Department{}, err
	}
	row, err := t.q.CreateDepartment(ctx, sqlcgen.CreateDepartmentParams{ID: id, TenantID: tenantID, Name: req.Name, Code: pgtypex.Text(req.Code)})
	if isNoRows(err) {
		return Department{}, apperr.ErrIdentityOrgCodeConflict.WithCause(err)
	}
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
	if err := t.lockOrganizationScope(ctx, tenantID); err != nil {
		return Department{}, err
	}
	exists, err := t.DepartmentExists(ctx, tenantID, id)
	if err != nil {
		return Department{}, err
	}
	if !exists {
		return Department{}, apperr.ErrNotFound
	}
	row, err := t.q.UpdateDepartment(ctx, sqlcgen.UpdateDepartmentParams{ID: id, TenantID: tenantID, Name: req.Name, Code: pgtypex.Text(req.Code)})
	if isNoRows(err) {
		return Department{}, apperr.ErrIdentityOrgCodeConflict.WithCause(err)
	}
	if err != nil {
		return Department{}, err
	}
	return departmentFromRow(row), nil
}

// DeleteDepartment 在组织锁内拒绝仍被专业或教师档案引用的院系删除。
func (t *txStore) DeleteDepartment(ctx context.Context, tenantID, id int64) error {
	if err := t.lockOrganizationScope(ctx, tenantID); err != nil {
		return err
	}
	exists, err := t.DepartmentExists(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.ErrNotFound
	}
	affected, err := t.q.SoftDeleteDepartment(ctx, sqlcgen.SoftDeleteDepartmentParams{ID: id, TenantID: tenantID})
	if err == nil && affected == 0 {
		return apperr.ErrIdentityOrgInUse
	}
	return err
}

// CreateMajor 创建专业记录。
func (t *txStore) CreateMajor(ctx context.Context, tenantID, id int64, req MajorRequest) (Major, error) {
	if err := t.lockOrganizationScope(ctx, tenantID); err != nil {
		return Major{}, err
	}
	if err := validateMajorParent(ctx, t, tenantID, req.DepartmentID.Int64()); err != nil {
		return Major{}, err
	}
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
	if err := t.lockOrganizationScope(ctx, tenantID); err != nil {
		return Major{}, err
	}
	if err := validateMajorParent(ctx, t, tenantID, req.DepartmentID.Int64()); err != nil {
		return Major{}, err
	}
	row, err := t.q.UpdateMajor(ctx, sqlcgen.UpdateMajorParams{ID: id, TenantID: tenantID, DepartmentID: req.DepartmentID.Int64(), Name: req.Name})
	if err != nil {
		return Major{}, err
	}
	return majorFromRow(row), nil
}

// DeleteMajor 在组织锁内拒绝仍有班级引用的专业删除。
func (t *txStore) DeleteMajor(ctx context.Context, tenantID, id int64) error {
	if err := t.lockOrganizationScope(ctx, tenantID); err != nil {
		return err
	}
	exists, err := t.MajorExists(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.ErrNotFound
	}
	affected, err := t.q.SoftDeleteMajor(ctx, sqlcgen.SoftDeleteMajorParams{ID: id, TenantID: tenantID})
	if err == nil && affected == 0 {
		return apperr.ErrIdentityOrgInUse
	}
	return err
}

// CreateClass 创建班级记录。
func (t *txStore) CreateClass(ctx context.Context, tenantID, id int64, req ClassRequest) (Class, error) {
	if err := t.lockOrganizationScope(ctx, tenantID); err != nil {
		return Class{}, err
	}
	if err := validateClassParent(ctx, t, tenantID, req.MajorID.Int64()); err != nil {
		return Class{}, err
	}
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
	if err := t.lockOrganizationScope(ctx, tenantID); err != nil {
		return Class{}, err
	}
	if err := validateClassParent(ctx, t, tenantID, req.MajorID.Int64()); err != nil {
		return Class{}, err
	}
	row, err := t.q.UpdateClass(ctx, sqlcgen.UpdateClassParams{ID: id, TenantID: tenantID, MajorID: req.MajorID.Int64(), Name: req.Name, EnrollmentYear: req.EnrollmentYear, Status: req.Status})
	if err != nil {
		return Class{}, err
	}
	return classFromRow(row), nil
}

// DeleteClass 在组织锁内拒绝仍被学生档案引用的班级删除。
func (t *txStore) DeleteClass(ctx context.Context, tenantID, id int64) error {
	if err := t.lockOrganizationScope(ctx, tenantID); err != nil {
		return err
	}
	exists, err := t.ClassExists(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.ErrNotFound
	}
	affected, err := t.q.SoftDeleteClass(ctx, sqlcgen.SoftDeleteClassParams{ID: id, TenantID: tenantID})
	if err == nil && affected == 0 {
		return apperr.ErrIdentityOrgInUse
	}
	return err
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
	if err := t.lockOrganizationScope(ctx, tenantID); err != nil {
		return 0, err
	}
	return t.q.PromoteClasses(ctx, sqlcgen.PromoteClassesParams{TenantID: tenantID, ClassIds: classIDs, TargetYear: targetYear})
}

// PromoteClassStudentProfiles 同步所选班级学生档案的入学年份。
func (t *txStore) PromoteClassStudentProfiles(ctx context.Context, tenantID int64, classIDs []int64, targetYear int16) error {
	return t.q.PromoteClassStudentProfiles(ctx, sqlcgen.PromoteClassStudentProfilesParams{TenantID: tenantID, ClassIds: classIDs, TargetYear: pgtypex.Int2(targetYear)})
}
