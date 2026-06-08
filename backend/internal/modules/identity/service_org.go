// M1 组织架构服务:院系/专业/班级 CRUD + 班级归档/升级。
// 依据 docs/01 §3 接口、§5 §7 流程(归档级联学生、升级)。
package identity

import (
	"context"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// ---- 院系 ----

// CreateDepartment 建院系。
func (s *Service) CreateDepartment(ctx context.Context, req CreateDepartmentRequest) (*OrgNode, error) {
	id := s.idgen.Generate()
	var row sqlcgen.Department
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		r, e := q.CreateDepartment(ctx, sqlcgen.CreateDepartmentParams{
			ID: id, TenantID: tenantFromCtx(ctx), Name: req.Name, Code: pgText(req.Code),
		})
		if isUniqueViolation(e) {
			return apperr.ErrOrgNameExists
		}
		if e != nil {
			return e
		}
		row = r
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
			"operation": "department.create",
		})
	}); err != nil {
		return nil, toAppErr(err)
	}
	return &OrgNode{ID: ids.Format(row.ID), Name: row.Name, Code: textVal(row.Code)}, nil
}

// ListDepartments 列院系。
func (s *Service) ListDepartments(ctx context.Context) ([]OrgNode, error) {
	var out []OrgNode
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		rows, e := q.ListDepartments(ctx)
		if e != nil {
			return e
		}
		for _, r := range rows {
			out = append(out, OrgNode{ID: ids.Format(r.ID), Name: r.Name, Code: textVal(r.Code)})
		}
		return nil
	}); err != nil {
		return nil, toAppErr(err)
	}
	return out, nil
}

// UpdateDepartment 改院系。
func (s *Service) UpdateDepartment(ctx context.Context, id int64, req UpdateDepartmentRequest) error {
	return s.mustTenantWith(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetDepartmentByID(ctx, id); e != nil {
			return apperr.ErrDepartmentNotFound
		}
		_, e := q.UpdateDepartment(ctx, sqlcgen.UpdateDepartmentParams{ID: id, Name: req.Name, Code: pgText(req.Code)})
		if isUniqueViolation(e) {
			return apperr.ErrOrgNameExists
		}
		if e != nil {
			return e
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
			"operation": "department.update",
			"fields":    []string{"name", "code"},
		})
	}, apperr.ErrOrgMutationFailed)
}

// DeleteDepartment 软删院系。
func (s *Service) DeleteDepartment(ctx context.Context, id int64) error {
	return s.mustTenantWith(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetDepartmentByID(ctx, id); e != nil {
			return apperr.ErrDepartmentNotFound
		}
		if err := q.SoftDeleteDepartment(ctx, id); err != nil {
			return err
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
			"operation": "department.delete",
		})
	}, apperr.ErrOrgMutationFailed)
}

// ---- 专业 ----

// CreateMajor 建专业。
func (s *Service) CreateMajor(ctx context.Context, req CreateMajorRequest) (*OrgNode, error) {
	deptID, ok := ids.Parse(req.DepartmentID)
	if !ok {
		return nil, apperr.ErrOrgParentIDInvalid
	}
	id := s.idgen.Generate()
	var row sqlcgen.Major
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetDepartmentByID(ctx, deptID); e != nil {
			return apperr.ErrDepartmentNotFound
		}
		r, e := q.CreateMajor(ctx, sqlcgen.CreateMajorParams{
			ID: id, TenantID: tenantFromCtx(ctx), DepartmentID: deptID, Name: req.Name,
		})
		if e != nil {
			return e
		}
		row = r
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
			"operation":     "major.create",
			"department_id": ids.Format(deptID),
		})
	}); err != nil {
		return nil, toAppErr(err)
	}
	return &OrgNode{ID: ids.Format(row.ID), Name: row.Name, ParentID: ids.Format(row.DepartmentID)}, nil
}

// ListMajorsByDepartment 按院系列专业。
func (s *Service) ListMajorsByDepartment(ctx context.Context, deptID int64) ([]OrgNode, error) {
	var out []OrgNode
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		rows, e := q.ListMajorsByDepartment(ctx, deptID)
		if e != nil {
			return e
		}
		for _, r := range rows {
			out = append(out, OrgNode{ID: ids.Format(r.ID), Name: r.Name, ParentID: ids.Format(r.DepartmentID)})
		}
		return nil
	}); err != nil {
		return nil, toAppErr(err)
	}
	return out, nil
}

// UpdateMajor 改专业名称。
func (s *Service) UpdateMajor(ctx context.Context, id int64, req UpdateMajorRequest) error {
	return s.mustTenantWith(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetMajorByID(ctx, id); e != nil {
			return apperr.ErrMajorNotFound
		}
		if _, e := q.UpdateMajor(ctx, sqlcgen.UpdateMajorParams{ID: id, Name: req.Name}); e != nil {
			return e
		}

		// 专业变更影响组织树展示和人员挂靠路径,必须留下组织变更审计。
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
			"operation": "major.update",
			"fields":    []string{"name"},
		})
	}, apperr.ErrOrgMutationFailed)
}

// DeleteMajor 软删专业。
func (s *Service) DeleteMajor(ctx context.Context, id int64) error {
	return s.mustTenantWith(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetMajorByID(ctx, id); e != nil {
			return apperr.ErrMajorNotFound
		}
		if err := q.SoftDeleteMajor(ctx, id); err != nil {
			return err
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
			"operation": "major.delete",
		})
	}, apperr.ErrOrgMutationFailed)
}

// ---- 班级 ----

// CreateClass 建班级。
func (s *Service) CreateClass(ctx context.Context, req CreateClassRequest) (*OrgNode, error) {
	majorID, ok := ids.Parse(req.MajorID)
	if !ok {
		return nil, apperr.ErrOrgParentIDInvalid
	}
	id := s.idgen.Generate()
	var row sqlcgen.Class
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetMajorByID(ctx, majorID); e != nil {
			return apperr.ErrMajorNotFound
		}
		r, e := q.CreateClass(ctx, sqlcgen.CreateClassParams{
			ID: id, TenantID: tenantFromCtx(ctx), MajorID: majorID, Name: req.Name, EnrollmentYear: req.EnrollmentYear,
		})
		if e != nil {
			return e
		}
		row = r
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
			"operation":       "class.create",
			"major_id":        ids.Format(majorID),
			"enrollment_year": req.EnrollmentYear,
		})
	}); err != nil {
		return nil, toAppErr(err)
	}
	y := row.EnrollmentYear
	st := row.Status
	return &OrgNode{ID: ids.Format(row.ID), Name: row.Name, ParentID: ids.Format(row.MajorID), EnrollmentYear: &y, Status: &st}, nil
}

// ListClassesByMajor 按专业列班级。
func (s *Service) ListClassesByMajor(ctx context.Context, majorID int64) ([]OrgNode, error) {
	var out []OrgNode
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		rows, e := q.ListClassesByMajor(ctx, majorID)
		if e != nil {
			return e
		}
		for _, r := range rows {
			y := r.EnrollmentYear
			st := r.Status
			out = append(out, OrgNode{ID: ids.Format(r.ID), Name: r.Name, ParentID: ids.Format(r.MajorID), EnrollmentYear: &y, Status: &st})
		}
		return nil
	}); err != nil {
		return nil, toAppErr(err)
	}
	return out, nil
}

// UpdateClass 改班级名称与入学年份。
func (s *Service) UpdateClass(ctx context.Context, id int64, req UpdateClassRequest) error {
	return s.mustTenantWith(ctx, func(q *sqlcgen.Queries) error {
		if req.EnrollmentYear <= 0 {
			return apperr.ErrClassEnrollmentYearInvalid
		}
		if _, e := q.GetClassByID(ctx, id); e != nil {
			return apperr.ErrClassNotFound
		}
		if _, e := q.UpdateClass(ctx, sqlcgen.UpdateClassParams{ID: id, Name: req.Name, EnrollmentYear: req.EnrollmentYear}); e != nil {
			return e
		}

		// 班级名称/入学年份会影响批量归档与升级判断,更新后写组织变更审计。
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
			"operation":       "class.update",
			"fields":          []string{"name", "enrollment_year"},
			"enrollment_year": req.EnrollmentYear,
		})
	}, apperr.ErrOrgMutationFailed)
}

// ArchiveClass 归档班级:班级置归档 + 级联归档其在读学生账号(docs/01 §7)。
func (s *Service) ArchiveClass(ctx context.Context, classID int64) error {
	return s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetClassByID(ctx, classID); e != nil {
			return apperr.ErrClassNotFound
		}
		if e := q.ArchiveClass(ctx, classID); e != nil {
			return e
		}
		// 级联:该班级在读学生账号一并归档。
		if e := q.ArchiveAccountsByClass(ctx, classID); e != nil {
			return e
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, classID, map[string]any{
			"operation": "class.archive",
			"cascade":   "student_accounts",
		})
	})
}

// PromoteClass 班级升级:调整 enrollment_year/名称(学生随班上移,账号状态不变)。
func (s *Service) PromoteClass(ctx context.Context, classID int64, name string, year int16) error {
	return s.mustTenantWith(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetClassByID(ctx, classID); e != nil {
			return apperr.ErrClassNotFound
		}
		if _, e := q.UpdateClass(ctx, sqlcgen.UpdateClassParams{ID: classID, Name: name, EnrollmentYear: year}); e != nil {
			return e
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, classID, map[string]any{
			"operation":       "class.promote",
			"enrollment_year": year,
		})
	}, apperr.ErrOrgMutationFailed)
}

// DeleteClass 软删班级。
func (s *Service) DeleteClass(ctx context.Context, id int64) error {
	return s.mustTenantWith(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetClassByID(ctx, id); e != nil {
			return apperr.ErrClassNotFound
		}
		if err := q.SoftDeleteClass(ctx, id); err != nil {
			return err
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
			"operation": "class.delete",
		})
	}, apperr.ErrOrgMutationFailed)
}

// ImportOrg 批量导入院系/专业/班级,并生成组织导入批次记录。
func (s *Service) ImportOrg(ctx context.Context, operatorID int64, req OrgImportRequest) (*OrgImportResult, error) {
	if len(req.Departments) == 0 {
		return nil, apperr.ErrImportEmpty
	}
	total := countOrgImportRows(req)
	if total > s.importMaxRows {
		return nil, apperr.ErrImportTooLarge
	}

	batchID := s.idgen.Generate()
	result := &OrgImportResult{BatchID: ids.Format(batchID), Total: total}
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		tenantID := tenantFromCtx(ctx)
		for _, dept := range req.Departments {
			s.importDepartmentNode(ctx, q, tenantID, dept, result)
		}
		detailJSON, err := jsonx.AnyBytes(result.Rows, apperr.ErrOrgImportRowsInvalid)
		if err != nil {
			return err
		}
		_, err = q.CreateImportBatch(ctx, sqlcgen.CreateImportBatchParams{
			ID:          batchID,
			TenantID:    tenantID,
			OperatorID:  operatorID,
			TargetType:  ImportTargetOrg,
			FileName:    req.FileName,
			Total:       int32(result.Total),
			Success:     int32(result.Success),
			Failed:      int32(result.Failed),
			ErrorDetail: detailJSON,
			Status:      ImportDone,
		})
		if err != nil {
			return err
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionOrgImport, AuditTargetImportBatch, batchID, map[string]any{
			"total":   result.Total,
			"success": result.Success,
			"failed":  result.Failed,
		})
	}); err != nil {
		return nil, toAppErr(err)
	}
	return result, nil
}

// BatchArchiveClasses 批量归档班级,逐项返回失败原因。
func (s *Service) BatchArchiveClasses(ctx context.Context, classIDs []string) (*BatchClassOperationResult, error) {
	if len(classIDs) == 0 {
		return nil, apperr.ErrBatchClassIDsInvalid
	}
	res := &BatchClassOperationResult{Total: len(classIDs)}
	for _, rawID := range classIDs {
		classID, ok := ids.Parse(rawID)
		row := BatchClassOperationRow{ClassID: rawID}
		if !ok {
			row.Error = "班级 ID 不正确"
			res.Failed++
			res.Rows = append(res.Rows, row)
			continue
		}
		if err := s.ArchiveClass(ctx, classID); err != nil {
			row.Error = appErrorMessage(err, "班级归档失败")
			res.Failed++
			res.Rows = append(res.Rows, row)
			continue
		}
		res.Success++
		res.Rows = append(res.Rows, row)
	}
	return res, nil
}

// BatchPromoteClasses 批量升级班级,逐项返回失败原因。
func (s *Service) BatchPromoteClasses(ctx context.Context, rows []ClassPromoteInput) (*BatchClassOperationResult, error) {
	if len(rows) == 0 {
		return nil, apperr.ErrBatchClassPromoteInvalid
	}
	res := &BatchClassOperationResult{Total: len(rows)}
	for _, in := range rows {
		classID, ok := ids.Parse(in.ClassID)
		row := BatchClassOperationRow{ClassID: in.ClassID}
		if !ok || in.Name == "" || in.EnrollmentYear <= 0 {
			row.Error = "班级升级信息不完整"
			res.Failed++
			res.Rows = append(res.Rows, row)
			continue
		}
		if err := s.PromoteClass(ctx, classID, in.Name, in.EnrollmentYear); err != nil {
			row.Error = appErrorMessage(err, "班级升级失败")
			res.Failed++
			res.Rows = append(res.Rows, row)
			continue
		}
		res.Success++
		res.Rows = append(res.Rows, row)
	}
	return res, nil
}

// importDepartmentNode 导入一个院系及其子节点,失败时继续处理后续行。
func (s *Service) importDepartmentNode(ctx context.Context, q *sqlcgen.Queries, tenantID int64, dept OrgImportDepartment, result *OrgImportResult) {
	line := len(result.Rows) + 1
	if dept.Name == "" {
		appendOrgImportFailure(result, line, "院系名称不能为空")
		return
	}
	row, err := q.CreateDepartment(ctx, sqlcgen.CreateDepartmentParams{
		ID: s.idgen.Generate(), TenantID: tenantID, Name: dept.Name, Code: pgText(dept.Code),
	})
	if err != nil {
		appendOrgImportFailure(result, line, appErrorMessage(err, "院系写入失败"))
		return
	}
	appendOrgImportSuccess(result, line)
	for _, major := range dept.Majors {
		s.importMajorNode(ctx, q, tenantID, row.ID, major, result)
	}
}

// importMajorNode 导入一个专业及其班级。
func (s *Service) importMajorNode(ctx context.Context, q *sqlcgen.Queries, tenantID, departmentID int64, major OrgImportMajor, result *OrgImportResult) {
	line := len(result.Rows) + 1
	if major.Name == "" {
		appendOrgImportFailure(result, line, "专业名称不能为空")
		return
	}
	row, err := q.CreateMajor(ctx, sqlcgen.CreateMajorParams{
		ID: s.idgen.Generate(), TenantID: tenantID, DepartmentID: departmentID, Name: major.Name,
	})
	if err != nil {
		appendOrgImportFailure(result, line, appErrorMessage(err, "专业写入失败"))
		return
	}
	appendOrgImportSuccess(result, line)
	for _, class := range major.Classes {
		s.importClassNode(ctx, q, tenantID, row.ID, class, result)
	}
}

// importClassNode 导入一个班级。
func (s *Service) importClassNode(ctx context.Context, q *sqlcgen.Queries, tenantID, majorID int64, class OrgImportClass, result *OrgImportResult) {
	line := len(result.Rows) + 1
	if class.Name == "" || class.EnrollmentYear <= 0 {
		appendOrgImportFailure(result, line, "班级名称或入学年份不能为空")
		return
	}
	_, err := q.CreateClass(ctx, sqlcgen.CreateClassParams{
		ID: s.idgen.Generate(), TenantID: tenantID, MajorID: majorID, Name: class.Name, EnrollmentYear: class.EnrollmentYear,
	})
	if err != nil {
		appendOrgImportFailure(result, line, appErrorMessage(err, "班级写入失败"))
		return
	}
	appendOrgImportSuccess(result, line)
}

// countOrgImportRows 统计组织导入总节点数。
func countOrgImportRows(req OrgImportRequest) int {
	total := 0
	for _, dept := range req.Departments {
		total++
		for _, major := range dept.Majors {
			total++
			total += len(major.Classes)
		}
	}
	return total
}

// appendOrgImportSuccess 记录组织导入成功行。
func appendOrgImportSuccess(result *OrgImportResult, line int) {
	result.Success++
	result.Rows = append(result.Rows, ImportPreviewRow{Line: line})
}

// appendOrgImportFailure 记录组织导入失败行。
func appendOrgImportFailure(result *OrgImportResult, line int, message string) {
	result.Failed++
	result.Rows = append(result.Rows, ImportPreviewRow{Line: line, Error: message})
}

// appErrorMessage 将应用错误转为用户向文案。
func appErrorMessage(err error, defaultMessage string) string {
	if ae, ok := apperr.As(err); ok {
		return ae.Message
	}
	return defaultMessage
}

// toAppErr 把 repo 返回错误统一转应用错误。
func toAppErr(err error) error {
	return toAppErrWith(err, apperr.ErrIdentityDataQueryFailed)
}

// toAppErrWith 保留已有应用错误,把底层错误转为调用场景指定的 M1 专属错误码。
func toAppErrWith(err error, code *apperr.Error) error {
	if ae, ok := apperr.As(err); ok {
		return ae
	}
	return code.WithCause(err)
}
