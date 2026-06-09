// M1 组织架构服务:院系/专业/班级 CRUD + 班级归档/升级。
// 依据 docs/01 §3 接口、§5 §7 流程(归档级联学生、升级)。
package identity

import (
	"context"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// ---- 院系 ----

// CreateDepartment 建院系。
func (s *Service) CreateDepartment(ctx context.Context, req CreateDepartmentRequest) (*OrgNode, error) {
	id := s.idgen.Generate()
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
		"operation": "department.create",
	})
	if err != nil {
		return nil, err
	}
	node, err := s.repo.createDepartmentWithAudit(ctx, id, tenantFromCtx(ctx), req, buildAuditLogCreate(s.idgen.Generate(), entry))
	if err != nil {
		return nil, toAppErr(err)
	}
	return &node, nil
}

// ListDepartments 列院系。
func (s *Service) ListDepartments(ctx context.Context) ([]OrgNode, error) {
	out, err := s.repo.listDepartments(ctx)
	if err != nil {
		return nil, toAppErr(err)
	}
	return out, nil
}

// UpdateDepartment 改院系。
func (s *Service) UpdateDepartment(ctx context.Context, id int64, req UpdateDepartmentRequest) error {
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
		"operation": "department.update",
		"fields":    []string{"name", "code"},
	})
	if err != nil {
		return err
	}
	return toAppErrWith(s.repo.updateDepartmentWithAudit(ctx, id, req, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrOrgMutationFailed)
}

// DeleteDepartment 软删院系。
func (s *Service) DeleteDepartment(ctx context.Context, id int64) error {
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
		"operation": "department.delete",
	})
	if err != nil {
		return err
	}
	return toAppErrWith(s.repo.deleteDepartmentWithAudit(ctx, id, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrOrgMutationFailed)
}

// ---- 专业 ----

// CreateMajor 建专业。
func (s *Service) CreateMajor(ctx context.Context, req CreateMajorRequest) (*OrgNode, error) {
	deptID, ok := ids.Parse(req.DepartmentID)
	if !ok {
		return nil, apperr.ErrOrgParentIDInvalid
	}
	id := s.idgen.Generate()
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
		"operation":     "major.create",
		"department_id": ids.Format(deptID),
	})
	if err != nil {
		return nil, err
	}
	node, err := s.repo.createMajorWithAudit(ctx, id, tenantFromCtx(ctx), deptID, req, buildAuditLogCreate(s.idgen.Generate(), entry))
	if err != nil {
		return nil, toAppErr(err)
	}
	return &node, nil
}

// ListMajorsByDepartment 按院系列专业。
func (s *Service) ListMajorsByDepartment(ctx context.Context, deptID int64) ([]OrgNode, error) {
	out, err := s.repo.listMajorsByDepartment(ctx, deptID)
	if err != nil {
		return nil, toAppErr(err)
	}
	return out, nil
}

// UpdateMajor 改专业名称。
func (s *Service) UpdateMajor(ctx context.Context, id int64, req UpdateMajorRequest) error {
	// 专业变更影响组织树展示和人员挂靠路径,必须留下组织变更审计。
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
		"operation": "major.update",
		"fields":    []string{"name"},
	})
	if err != nil {
		return err
	}
	return toAppErrWith(s.repo.updateMajorWithAudit(ctx, id, req, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrOrgMutationFailed)
}

// DeleteMajor 软删专业。
func (s *Service) DeleteMajor(ctx context.Context, id int64) error {
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
		"operation": "major.delete",
	})
	if err != nil {
		return err
	}
	return toAppErrWith(s.repo.deleteMajorWithAudit(ctx, id, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrOrgMutationFailed)
}

// ---- 班级 ----

// CreateClass 建班级。
func (s *Service) CreateClass(ctx context.Context, req CreateClassRequest) (*OrgNode, error) {
	majorID, ok := ids.Parse(req.MajorID)
	if !ok {
		return nil, apperr.ErrOrgParentIDInvalid
	}
	id := s.idgen.Generate()
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
		"operation":       "class.create",
		"major_id":        ids.Format(majorID),
		"enrollment_year": req.EnrollmentYear,
	})
	if err != nil {
		return nil, err
	}
	node, err := s.repo.createClassWithAudit(ctx, id, tenantFromCtx(ctx), majorID, req, buildAuditLogCreate(s.idgen.Generate(), entry))
	if err != nil {
		return nil, toAppErr(err)
	}
	return &node, nil
}

// ListClassesByMajor 按专业列班级。
func (s *Service) ListClassesByMajor(ctx context.Context, majorID int64) ([]OrgNode, error) {
	out, err := s.repo.listClassesByMajor(ctx, majorID)
	if err != nil {
		return nil, toAppErr(err)
	}
	return out, nil
}

// UpdateClass 改班级名称与入学年份。
func (s *Service) UpdateClass(ctx context.Context, id int64, req UpdateClassRequest) error {
	if req.EnrollmentYear <= 0 {
		return apperr.ErrClassEnrollmentYearInvalid
	}
	// 班级名称/入学年份会影响批量归档与升级判断,更新后写组织变更审计。
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
		"operation":       "class.update",
		"fields":          []string{"name", "enrollment_year"},
		"enrollment_year": req.EnrollmentYear,
	})
	if err != nil {
		return err
	}
	return toAppErrWith(s.repo.updateClassWithAudit(ctx, id, req.Name, req.EnrollmentYear, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrOrgMutationFailed)
}

// ArchiveClass 归档班级:班级置归档 + 级联归档其在读学生账号(docs/01 §7)。
func (s *Service) ArchiveClass(ctx context.Context, classID int64) error {
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, classID, map[string]any{
		"operation": "class.archive",
		"cascade":   "student_accounts",
	})
	if err != nil {
		return err
	}
	return toAppErrWith(s.repo.archiveClassWithAudit(ctx, classID, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrOrgMutationFailed)
}

// PromoteClass 班级升级:调整 enrollment_year/名称(学生随班上移,账号状态不变)。
func (s *Service) PromoteClass(ctx context.Context, classID int64, name string, year int16) error {
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, classID, map[string]any{
		"operation":       "class.promote",
		"enrollment_year": year,
	})
	if err != nil {
		return err
	}
	return toAppErrWith(s.repo.updateClassWithAudit(ctx, classID, name, year, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrOrgMutationFailed)
}

// DeleteClass 软删班级。
func (s *Service) DeleteClass(ctx context.Context, id int64) error {
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgChange, AuditTargetOrg, id, map[string]any{
		"operation": "class.delete",
	})
	if err != nil {
		return err
	}
	return toAppErrWith(s.repo.deleteClassWithAudit(ctx, id, buildAuditLogCreate(s.idgen.Generate(), entry)), apperr.ErrOrgMutationFailed)
}

// ImportOrg 批量导入院系、专业和班级,逐节点记录成功/失败并生成可追溯批次。
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
	// 组织导入必须在 repo 事务内写节点、批次和审计;service 只准备审计与结果容器。
	if err := s.repo.importOrgWithAudit(ctx, tenantFromCtx(ctx), operatorID, batchID, req, result, func() ([]byte, error) {
		return jsonx.AnyBytes(result.Rows, apperr.ErrOrgImportRowsInvalid)
	}, func() (AuditLogCreate, error) {
		entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionOrgImport, AuditTargetImportBatch, batchID, map[string]any{
			"total":   result.Total,
			"success": result.Success,
			"failed":  result.Failed,
		})
		if err != nil {
			return AuditLogCreate{}, err
		}
		return buildAuditLogCreate(s.idgen.Generate(), entry), nil
	}, s.idgen.Generate); err != nil {
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
