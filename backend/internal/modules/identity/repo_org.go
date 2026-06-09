// M1 组织数据访问:集中处理院系、专业、班级和组织导入批次的持久化事务。
package identity

import (
	"context"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pgtypex"
	"chaimir/pkg/apperr"
)

// createDepartmentWithAudit 创建院系并写组织变更审计。
func (r *repo) createDepartmentWithAudit(ctx context.Context, id, tenantID int64, req CreateDepartmentRequest, auditLog AuditLogCreate) (OrgNode, error) {
	var row sqlcgen.Department
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		created, err := q.CreateDepartment(ctx, sqlcgen.CreateDepartmentParams{ID: id, TenantID: tenantID, Name: req.Name, Code: pgtypex.Text(req.Code)})
		if isUniqueViolation(err) {
			return apperr.ErrOrgNameExists
		}
		if err != nil {
			return err
		}
		row = created
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	}); err != nil {
		return OrgNode{}, err
	}
	return departmentNode(row), nil
}

// listDepartments 读取院系列表。
func (r *repo) listDepartments(ctx context.Context) ([]OrgNode, error) {
	var rows []sqlcgen.Department
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListDepartments(ctx)
		if err != nil {
			return err
		}
		rows = found
		return nil
	}); err != nil {
		return nil, err
	}
	out := make([]OrgNode, 0, len(rows))
	for _, row := range rows {
		out = append(out, departmentNode(row))
	}
	return out, nil
}

// updateDepartmentWithAudit 确认院系存在后更新并写审计。
func (r *repo) updateDepartmentWithAudit(ctx context.Context, id int64, req UpdateDepartmentRequest, auditLog AuditLogCreate) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetDepartmentByID(ctx, id); err != nil {
			return apperr.ErrDepartmentNotFound
		}
		if _, err := q.UpdateDepartment(ctx, sqlcgen.UpdateDepartmentParams{ID: id, Name: req.Name, Code: pgtypex.Text(req.Code)}); isUniqueViolation(err) {
			return apperr.ErrOrgNameExists
		} else if err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// deleteDepartmentWithAudit 确认院系存在后软删并写审计。
func (r *repo) deleteDepartmentWithAudit(ctx context.Context, id int64, auditLog AuditLogCreate) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetDepartmentByID(ctx, id); err != nil {
			return apperr.ErrDepartmentNotFound
		}
		if err := q.SoftDeleteDepartment(ctx, id); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// createMajorWithAudit 确认院系存在后创建专业并写审计。
func (r *repo) createMajorWithAudit(ctx context.Context, id, tenantID, deptID int64, req CreateMajorRequest, auditLog AuditLogCreate) (OrgNode, error) {
	var row sqlcgen.Major
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetDepartmentByID(ctx, deptID); err != nil {
			return apperr.ErrDepartmentNotFound
		}
		created, err := q.CreateMajor(ctx, sqlcgen.CreateMajorParams{ID: id, TenantID: tenantID, DepartmentID: deptID, Name: req.Name})
		if err != nil {
			return err
		}
		row = created
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	}); err != nil {
		return OrgNode{}, err
	}
	return majorNode(row), nil
}

// listMajorsByDepartment 按院系读取专业列表。
func (r *repo) listMajorsByDepartment(ctx context.Context, deptID int64) ([]OrgNode, error) {
	var rows []sqlcgen.Major
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListMajorsByDepartment(ctx, deptID)
		if err != nil {
			return err
		}
		rows = found
		return nil
	}); err != nil {
		return nil, err
	}
	out := make([]OrgNode, 0, len(rows))
	for _, row := range rows {
		out = append(out, majorNode(row))
	}
	return out, nil
}

// updateMajorWithAudit 确认专业存在后更新并写审计。
func (r *repo) updateMajorWithAudit(ctx context.Context, id int64, req UpdateMajorRequest, auditLog AuditLogCreate) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetMajorByID(ctx, id); err != nil {
			return apperr.ErrMajorNotFound
		}
		if _, err := q.UpdateMajor(ctx, sqlcgen.UpdateMajorParams{ID: id, Name: req.Name}); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// deleteMajorWithAudit 确认专业存在后软删并写审计。
func (r *repo) deleteMajorWithAudit(ctx context.Context, id int64, auditLog AuditLogCreate) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetMajorByID(ctx, id); err != nil {
			return apperr.ErrMajorNotFound
		}
		if err := q.SoftDeleteMajor(ctx, id); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// createClassWithAudit 确认专业存在后创建班级并写审计。
func (r *repo) createClassWithAudit(ctx context.Context, id, tenantID, majorID int64, req CreateClassRequest, auditLog AuditLogCreate) (OrgNode, error) {
	var row sqlcgen.Class
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetMajorByID(ctx, majorID); err != nil {
			return apperr.ErrMajorNotFound
		}
		created, err := q.CreateClass(ctx, sqlcgen.CreateClassParams{ID: id, TenantID: tenantID, MajorID: majorID, Name: req.Name, EnrollmentYear: req.EnrollmentYear})
		if err != nil {
			return err
		}
		row = created
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	}); err != nil {
		return OrgNode{}, err
	}
	return classNode(row), nil
}

// listClassesByMajor 按专业读取班级列表。
func (r *repo) listClassesByMajor(ctx context.Context, majorID int64) ([]OrgNode, error) {
	var rows []sqlcgen.Class
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListClassesByMajor(ctx, majorID)
		if err != nil {
			return err
		}
		rows = found
		return nil
	}); err != nil {
		return nil, err
	}
	out := make([]OrgNode, 0, len(rows))
	for _, row := range rows {
		out = append(out, classNode(row))
	}
	return out, nil
}

// updateClassWithAudit 确认班级存在后更新名称和入学年份并写审计。
func (r *repo) updateClassWithAudit(ctx context.Context, id int64, name string, year int16, auditLog AuditLogCreate) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetClassByID(ctx, id); err != nil {
			return apperr.ErrClassNotFound
		}
		if _, err := q.UpdateClass(ctx, sqlcgen.UpdateClassParams{ID: id, Name: name, EnrollmentYear: year}); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// archiveClassWithAudit 归档班级并级联归档在读学生账号。
func (r *repo) archiveClassWithAudit(ctx context.Context, classID int64, auditLog AuditLogCreate) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetClassByID(ctx, classID); err != nil {
			return apperr.ErrClassNotFound
		}
		if err := q.ArchiveClass(ctx, classID); err != nil {
			return err
		}
		// 班级归档的学生账号级联状态变更必须和班级状态同事务提交。
		if err := q.ArchiveAccountsByClass(ctx, classID); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// deleteClassWithAudit 确认班级存在后软删并写审计。
func (r *repo) deleteClassWithAudit(ctx context.Context, id int64, auditLog AuditLogCreate) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetClassByID(ctx, id); err != nil {
			return apperr.ErrClassNotFound
		}
		if err := q.SoftDeleteClass(ctx, id); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// importOrgWithAudit 批量导入组织节点、写批次摘要和审计。
func (r *repo) importOrgWithAudit(ctx context.Context, tenantID, operatorID, batchID int64, req OrgImportRequest, result *OrgImportResult, detailJSON func() ([]byte, error), auditLog func() (AuditLogCreate, error), nextID func() int64) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		for _, dept := range req.Departments {
			importDepartmentNode(ctx, q, tenantID, dept, result, nextID)
		}
		detail, err := detailJSON()
		if err != nil {
			return err
		}
		if _, err := q.CreateImportBatch(ctx, sqlcgen.CreateImportBatchParams{
			ID: batchID, TenantID: tenantID, OperatorID: operatorID, TargetType: ImportTargetOrg,
			FileName: req.FileName, Total: int32(result.Total), Success: int32(result.Success),
			Failed: int32(result.Failed), ErrorDetail: detail, Status: ImportDone,
		}); err != nil {
			return err
		}
		auditRow, err := auditLog()
		if err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditRow))
	})
}

// importDepartmentNode 导入一个院系及其子节点,失败时继续处理后续行。
func importDepartmentNode(ctx context.Context, q *sqlcgen.Queries, tenantID int64, dept OrgImportDepartment, result *OrgImportResult, nextID func() int64) {
	line := len(result.Rows) + 1
	if dept.Name == "" {
		appendOrgImportFailure(result, line, "院系名称不能为空")
		return
	}
	row, err := q.CreateDepartment(ctx, sqlcgen.CreateDepartmentParams{ID: nextID(), TenantID: tenantID, Name: dept.Name, Code: pgtypex.Text(dept.Code)})
	if err != nil {
		appendOrgImportFailure(result, line, appErrorMessage(err, "院系写入失败"))
		return
	}
	appendOrgImportSuccess(result, line)
	for _, major := range dept.Majors {
		importMajorNode(ctx, q, tenantID, row.ID, major, result, nextID)
	}
}

// importMajorNode 导入一个专业及其班级。
func importMajorNode(ctx context.Context, q *sqlcgen.Queries, tenantID, departmentID int64, major OrgImportMajor, result *OrgImportResult, nextID func() int64) {
	line := len(result.Rows) + 1
	if major.Name == "" {
		appendOrgImportFailure(result, line, "专业名称不能为空")
		return
	}
	row, err := q.CreateMajor(ctx, sqlcgen.CreateMajorParams{ID: nextID(), TenantID: tenantID, DepartmentID: departmentID, Name: major.Name})
	if err != nil {
		appendOrgImportFailure(result, line, appErrorMessage(err, "专业写入失败"))
		return
	}
	appendOrgImportSuccess(result, line)
	for _, class := range major.Classes {
		importClassNode(ctx, q, tenantID, row.ID, class, result, nextID)
	}
}

// importClassNode 导入一个班级。
func importClassNode(ctx context.Context, q *sqlcgen.Queries, tenantID, majorID int64, class OrgImportClass, result *OrgImportResult, nextID func() int64) {
	line := len(result.Rows) + 1
	if class.Name == "" || class.EnrollmentYear <= 0 {
		appendOrgImportFailure(result, line, "班级名称或入学年份不能为空")
		return
	}
	_, err := q.CreateClass(ctx, sqlcgen.CreateClassParams{ID: nextID(), TenantID: tenantID, MajorID: majorID, Name: class.Name, EnrollmentYear: class.EnrollmentYear})
	if err != nil {
		appendOrgImportFailure(result, line, appErrorMessage(err, "班级写入失败"))
		return
	}
	appendOrgImportSuccess(result, line)
}

// departmentNode 把院系行转换为统一组织节点。
func departmentNode(row sqlcgen.Department) OrgNode {
	return OrgNode{ID: ids.Format(row.ID), Name: row.Name, Code: textVal(row.Code)}
}

// majorNode 把专业行转换为统一组织节点。
func majorNode(row sqlcgen.Major) OrgNode {
	return OrgNode{ID: ids.Format(row.ID), Name: row.Name, ParentID: ids.Format(row.DepartmentID)}
}

// classNode 把班级行转换为统一组织节点。
func classNode(row sqlcgen.Class) OrgNode {
	year := row.EnrollmentYear
	status := row.Status
	return OrgNode{ID: ids.Format(row.ID), Name: row.Name, ParentID: ids.Format(row.MajorID), EnrollmentYear: &year, Status: &status}
}
