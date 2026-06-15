// identity service_org 文件实现组织架构 CRUD、班级归档和升级业务编排。
package identity

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"

	"github.com/xuri/excelize/v2"
)

// orgImportRow 表示组织导入 CSV 的单行解析结果。
type orgImportRow struct {
	Line           int    `json:"line"`
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	Code           string `json:"code,omitempty"`
	ParentID       int64  `json:"parent_id,omitempty"`
	EnrollmentYear int16  `json:"enrollment_year,omitempty"`
	Error          string `json:"error,omitempty"`
}

const importSheetOrg = "org"

// ListDepartmentsByAdmin 读取当前租户院系列表。
func (s *Service) ListDepartmentsByAdmin(ctx context.Context) ([]DepartmentDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return nil, err
	}
	var rows []Department
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		items, err := tx.ListDepartments(ctx)
		if err != nil {
			return err
		}
		rows = items
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]DepartmentDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToDepartmentDTO(row))
	}
	return out, nil
}

// CreateDepartmentByAdmin 创建院系,名称和短码必须在入口处显式校验。
func (s *Service) CreateDepartmentByAdmin(ctx context.Context, req DepartmentRequest) (DepartmentDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return DepartmentDTO{}, err
	}
	if err := validateDepartmentRequest(req); err != nil {
		return DepartmentDTO{}, err
	}
	var row Department
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		item, err := tx.CreateDepartment(ctx, id.TenantID, s.ids.Generate(), normalizeDepartmentRequest(req))
		if err != nil {
			return err
		}
		row = item
		return s.writeOrgAuditInTx(ctx, tx, id, "org.department.create", "identity.department", row.ID, map[string]any{"code": row.Code})
	}); err != nil {
		return DepartmentDTO{}, apperr.ErrInternal.WithCause(err)
	}
	return ToDepartmentDTO(row), nil
}

// UpdateDepartmentByAdmin 更新院系基础信息。
func (s *Service) UpdateDepartmentByAdmin(ctx context.Context, departmentID int64, req DepartmentRequest) (DepartmentDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return DepartmentDTO{}, err
	}
	if err := validateDepartmentRequest(req); err != nil {
		return DepartmentDTO{}, err
	}
	var row Department
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		item, err := tx.UpdateDepartment(ctx, id.TenantID, departmentID, normalizeDepartmentRequest(req))
		if err != nil {
			return err
		}
		row = item
		return s.writeOrgAuditInTx(ctx, tx, id, "org.department.update", "identity.department", departmentID, map[string]any{"code": row.Code})
	}); err != nil {
		return DepartmentDTO{}, apperr.ErrInternal.WithCause(err)
	}
	return ToDepartmentDTO(row), nil
}

// DeleteDepartmentByAdmin 软删除院系,数据库外键负责阻止仍被引用的非法删除。
func (s *Service) DeleteDepartmentByAdmin(ctx context.Context, departmentID int64) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.DeleteDepartment(ctx, id.TenantID, departmentID); err != nil {
			return err
		}
		return s.writeOrgAuditInTx(ctx, tx, id, "org.department.delete", "identity.department", departmentID, map[string]any{})
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return nil
}

// ListMajorsByAdmin 读取专业列表,可按院系过滤。
func (s *Service) ListMajorsByAdmin(ctx context.Context, departmentID int64) ([]MajorDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return nil, err
	}
	var rows []Major
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		items, err := tx.ListMajors(ctx, departmentID)
		if err != nil {
			return err
		}
		rows = items
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]MajorDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToMajorDTO(row))
	}
	return out, nil
}

// CreateMajorByAdmin 创建专业并绑定院系。
func (s *Service) CreateMajorByAdmin(ctx context.Context, req MajorRequest) (MajorDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return MajorDTO{}, err
	}
	if err := validateMajorRequest(req); err != nil {
		return MajorDTO{}, err
	}
	var row Major
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := validateMajorParent(ctx, tx, id.TenantID, req.DepartmentID); err != nil {
			return err
		}
		item, err := tx.CreateMajor(ctx, id.TenantID, s.ids.Generate(), normalizeMajorRequest(req))
		if err != nil {
			return err
		}
		row = item
		return s.writeOrgAuditInTx(ctx, tx, id, "org.major.create", "identity.major", row.ID, map[string]any{"department_id": req.DepartmentID})
	}); err != nil {
		return MajorDTO{}, apperr.ErrInternal.WithCause(err)
	}
	return ToMajorDTO(row), nil
}

// UpdateMajorByAdmin 更新专业名称和所属院系。
func (s *Service) UpdateMajorByAdmin(ctx context.Context, majorID int64, req MajorRequest) (MajorDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return MajorDTO{}, err
	}
	if err := validateMajorRequest(req); err != nil {
		return MajorDTO{}, err
	}
	var row Major
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := validateMajorParent(ctx, tx, id.TenantID, req.DepartmentID); err != nil {
			return err
		}
		item, err := tx.UpdateMajor(ctx, id.TenantID, majorID, normalizeMajorRequest(req))
		if err != nil {
			return err
		}
		row = item
		return s.writeOrgAuditInTx(ctx, tx, id, "org.major.update", "identity.major", majorID, map[string]any{"department_id": req.DepartmentID})
	}); err != nil {
		return MajorDTO{}, apperr.ErrInternal.WithCause(err)
	}
	return ToMajorDTO(row), nil
}

// DeleteMajorByAdmin 软删除专业记录。
func (s *Service) DeleteMajorByAdmin(ctx context.Context, majorID int64) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.DeleteMajor(ctx, id.TenantID, majorID); err != nil {
			return err
		}
		return s.writeOrgAuditInTx(ctx, tx, id, "org.major.delete", "identity.major", majorID, map[string]any{})
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return nil
}

// ListClassesByAdmin 读取班级列表,可按专业过滤。
func (s *Service) ListClassesByAdmin(ctx context.Context, majorID int64) ([]ClassDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return nil, err
	}
	var rows []Class
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		items, err := tx.ListClasses(ctx, majorID)
		if err != nil {
			return err
		}
		rows = items
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]ClassDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToClassDTO(row))
	}
	return out, nil
}

// CreateClassByAdmin 创建班级并绑定专业和入学年份。
func (s *Service) CreateClassByAdmin(ctx context.Context, req ClassRequest) (ClassDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return ClassDTO{}, err
	}
	if err := validateClassRequest(req); err != nil {
		return ClassDTO{}, err
	}
	var row Class
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := validateClassParent(ctx, tx, id.TenantID, req.MajorID); err != nil {
			return err
		}
		item, err := tx.CreateClass(ctx, id.TenantID, s.ids.Generate(), normalizeClassRequest(req))
		if err != nil {
			return err
		}
		row = item
		return s.writeOrgAuditInTx(ctx, tx, id, "org.class.create", "identity.class", row.ID, map[string]any{"major_id": req.MajorID, "enrollment_year": req.EnrollmentYear})
	}); err != nil {
		return ClassDTO{}, apperr.ErrInternal.WithCause(err)
	}
	return ToClassDTO(row), nil
}

// UpdateClassByAdmin 更新班级信息和状态。
func (s *Service) UpdateClassByAdmin(ctx context.Context, classID int64, req ClassRequest) (ClassDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return ClassDTO{}, err
	}
	if err := validateClassRequest(req); err != nil {
		return ClassDTO{}, err
	}
	var row Class
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := validateClassParent(ctx, tx, id.TenantID, req.MajorID); err != nil {
			return err
		}
		item, err := tx.UpdateClass(ctx, id.TenantID, classID, normalizeClassRequest(req))
		if err != nil {
			return err
		}
		row = item
		return s.writeOrgAuditInTx(ctx, tx, id, "org.class.update", "identity.class", classID, map[string]any{"major_id": req.MajorID, "enrollment_year": req.EnrollmentYear})
	}); err != nil {
		return ClassDTO{}, apperr.ErrInternal.WithCause(err)
	}
	return ToClassDTO(row), nil
}

// DeleteClassByAdmin 软删除班级记录。
func (s *Service) DeleteClassByAdmin(ctx context.Context, classID int64) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.DeleteClass(ctx, id.TenantID, classID); err != nil {
			return err
		}
		return s.writeOrgAuditInTx(ctx, tx, id, "org.class.delete", "identity.class", classID, map[string]any{})
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return nil
}

// OrgImportTemplate 生成组织结构导入模板文件,默认 Excel,也支持 CSV。
func (s *Service) OrgImportTemplate(format string) (ImportTemplateFile, error) {
	normalizedFormat := strings.ToLower(strings.TrimSpace(format))
	if normalizedFormat == "" {
		normalizedFormat = importTemplateFormatXLSX
	}
	records := [][]string{
		{"kind", "name", "code_or_parent_id", "enrollment_year"},
		{"department", "计算机学院", "CS", ""},
		{"major", "区块链工程", "1001", ""},
		{"class", "区块链 2024-1 班", "2001", "2024"},
	}
	switch normalizedFormat {
	case importTemplateFormatCSV:
		content, err := encodeImportCSV(records)
		if err != nil {
			return ImportTemplateFile{}, err
		}
		return ImportTemplateFile{FileName: "org_import_template.csv", ContentType: "text/csv; charset=utf-8", Content: content}, nil
	case importTemplateFormatXLSX:
		content, err := encodeNamedXLSX(importSheetOrg, records)
		if err != nil {
			return ImportTemplateFile{}, err
		}
		return ImportTemplateFile{FileName: "org_import_template.xlsx", ContentType: upload.XLSXContentType, Content: content}, nil
	default:
		return ImportTemplateFile{}, apperr.ErrIdentityImportFormatInvalid
	}
}

// PreviewOrgImportByAdmin 解析组织导入文件并把预览结果持久化到服务端。
func (s *Service) PreviewOrgImportByAdmin(ctx context.Context, req ImportPreviewRequest) (ImportPreviewResponse, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return ImportPreviewResponse{}, err
	}
	rows, results, err := s.parseOrgImportFile(req.Content, req.FileName, req.ContentType)
	if err != nil {
		return ImportPreviewResponse{}, err
	}
	if len(rows) > s.cfg.ImportMaxRows {
		return ImportPreviewResponse{}, apperr.ErrIdentityImportTooManyRows
	}
	previewID := s.ids.Generate()
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		// 预览阶段必须给出逐行错误,因此先校验数据库中已存在的上级组织,不能留到提交阶段静默失败。
		if err := s.validateOrgImportParents(ctx, tx, id.TenantID, rows, results); err != nil {
			return err
		}
		// 组织导入同样是两步流程,预览必须落库,避免前端刷新后只能重新上传文件。
		rowsJSON, err := jsonx.AnyBytes(rows, apperr.ErrInternal)
		if err != nil {
			return err
		}
		resultJSON, err := jsonx.AnyBytes(results, apperr.ErrInternal)
		if err != nil {
			return err
		}
		_, err = tx.CreateImportPreview(ctx, CreateImportPreviewInput{
			ID:            previewID,
			TenantID:      id.TenantID,
			OperatorID:    id.AccountID,
			TargetType:    ImportTargetOrg,
			FileName:      req.FileName,
			Rows:          rowsJSON,
			PreviewResult: resultJSON,
			ExpireAt:      timex.Now().Add(time.Duration(s.cfg.ImportPreviewTTLHours) * time.Hour),
		})
		return err
	}); err != nil {
		return ImportPreviewResponse{}, apperr.ErrInternal.WithCause(err)
	}
	valid := 0
	for _, row := range rows {
		if row.Error == "" {
			valid++
		}
	}
	return ImportPreviewResponse{PreviewID: previewID, Total: len(rows), Valid: valid, Invalid: len(rows) - valid, Rows: results}, nil
}

// CommitOrgImportByAdmin 读取服务端预览并仅提交校验通过的组织结构行。
func (s *Service) CommitOrgImportByAdmin(ctx context.Context, req ImportCommitRequest) (ImportBatch, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return ImportBatch{}, err
	}
	var batch ImportBatch
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		preview, err := tx.GetImportPreview(ctx, id.TenantID, req.PreviewID)
		if err != nil {
			return err
		}
		if preview.TargetType != ImportTargetOrg || preview.Status != ImportPreviewPending || timex.Now().After(preview.ExpireAt) {
			return apperr.ErrIdentityImportPreviewExpired
		}
		var rows []orgImportRow
		if err := jsonx.DecodeStrict(preview.Rows, &rows); err != nil {
			return fmt.Errorf("解析组织导入预览失败: %w", err)
		}
		success, failed := int32(0), int32(0)
		for _, row := range rows {
			if row.Error != "" {
				failed++
				continue
			}
			// 只提交预览阶段校验通过的行,数据库唯一约束仍作为并发导入的最后防线。
			if err := s.createOrgImportRow(ctx, tx, id.TenantID, row); err != nil {
				failed++
				continue
			}
			success++
		}
		// 提交状态和批次记录在同一事务内完成,保证导入中心历史与预览消费状态一致。
		if err := tx.MarkImportPreviewSubmitted(ctx, id.TenantID, req.PreviewID); err != nil {
			return err
		}
		errorDetail, err := jsonx.AnyBytes(rows, apperr.ErrInternal)
		if err != nil {
			return err
		}
		created, err := tx.CreateImportBatch(ctx, CreateImportBatchInput{
			ID:          s.ids.Generate(),
			TenantID:    id.TenantID,
			OperatorID:  id.AccountID,
			TargetType:  ImportTargetOrg,
			FileName:    preview.FileName,
			Total:       int32(len(rows)),
			Success:     success,
			Failed:      failed,
			ErrorDetail: errorDetail,
			Status:      ImportBatchCompleted,
		})
		if err != nil {
			return err
		}
		batch = created
		entry, err := audit.BuildEntry(ctx, id.TenantID, id.AccountID, contracts.RoleNumSchoolAdmin, "org.import", "identity.import_batch", created.ID, map[string]any{"success": success, "failed": failed})
		if err != nil {
			return err
		}
		return tx.WriteAudit(ctx, WriteAuditInput{
			ID:         s.ids.Generate(),
			TenantID:   entry.TenantID,
			ActorID:    entry.ActorID,
			ActorRole:  entry.ActorRole,
			Action:     entry.Action,
			TargetType: entry.TargetType,
			TargetID:   entry.TargetID,
			Detail:     []byte(entry.Detail),
			IP:         entry.IP,
			TraceID:    entry.TraceID,
		})
	}); err != nil {
		return ImportBatch{}, apperr.AsAppError(err)
	}
	return batch, nil
}

// createOrgImportRow 按导入行类型创建组织结构记录。
func (s *Service) createOrgImportRow(ctx context.Context, tx TxStore, tenantID int64, row orgImportRow) error {
	switch row.Kind {
	case "department":
		_, err := tx.CreateDepartment(ctx, tenantID, s.ids.Generate(), DepartmentRequest{Name: row.Name, Code: row.Code})
		return err
	case "major":
		_, err := tx.CreateMajor(ctx, tenantID, s.ids.Generate(), MajorRequest{DepartmentID: row.ParentID, Name: row.Name})
		return err
	case "class":
		_, err := tx.CreateClass(ctx, tenantID, s.ids.Generate(), ClassRequest{MajorID: row.ParentID, Name: row.Name, EnrollmentYear: row.EnrollmentYear, Status: ClassStatusActive})
		return err
	default:
		return apperr.ErrIdentityOrgInvalidInput
	}
}

// ArchiveClassesByAdmin 按入学年份归档当前租户班级,并同步归档该学年正常学生账号。
func (s *Service) ArchiveClassesByAdmin(ctx context.Context, req ArchiveClassesRequest) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	if req.EnrollmentYear <= 0 {
		return apperr.ErrIdentityOrgInvalidInput
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		// 先归档班级,再归档同学年的正常学生账号,保证文档定义的毕业状态一致落库。
		if err := tx.ArchiveClassesByEnrollmentYear(ctx, id.TenantID, req.EnrollmentYear); err != nil {
			return err
		}
		if err := tx.ArchiveStudentAccountsByEnrollmentYear(ctx, id.TenantID, req.EnrollmentYear); err != nil {
			return err
		}
		// 被归档学生不可继续使用旧登录态,因此状态变更后必须同步吊销服务端会话。
		if err := tx.RevokeStudentSessionsByEnrollmentYear(ctx, id.TenantID, req.EnrollmentYear); err != nil {
			return err
		}
		entry, err := audit.BuildEntry(ctx, id.TenantID, id.AccountID, contracts.RoleNumSchoolAdmin, "org.class.archive", "identity.class", 0, map[string]any{"enrollment_year": req.EnrollmentYear})
		if err != nil {
			return err
		}
		return tx.WriteAudit(ctx, WriteAuditInput{
			ID:         s.ids.Generate(),
			TenantID:   entry.TenantID,
			ActorID:    entry.ActorID,
			ActorRole:  entry.ActorRole,
			Action:     entry.Action,
			TargetType: entry.TargetType,
			TargetID:   entry.TargetID,
			Detail:     []byte(entry.Detail),
			IP:         entry.IP,
			TraceID:    entry.TraceID,
		})
	}); err != nil {
		return apperr.AsAppError(err)
	}
	return nil
}

// parseOrgImportFile 按统一上传安全原语识别 CSV/XLSX,再进入对应解析器。
func (s *Service) parseOrgImportFile(raw []byte, fileName, contentType string) ([]orgImportRow, []ImportRowResult, error) {
	if filepath.Base(fileName) != fileName || strings.TrimSpace(fileName) == "" {
		return nil, nil, apperr.ErrIdentityImportUnsupportedFile
	}
	switch upload.CheckSize(int64(len(raw)), s.uploadCfg.ImportMaxBytes) {
	case upload.SizeEmpty:
		return nil, nil, apperr.ErrIdentityImportContentInvalid
	case upload.SizeTooLarge:
		return nil, nil, apperr.ErrIdentityImportFileTooLarge
	}
	switch upload.CSVOrXLSXKind(fileName, contentType, raw) {
	case upload.KindCSV:
		return parseOrgImportCSV(raw)
	case upload.KindXLSX:
		return parseOrgImportXLSX(raw)
	default:
		return nil, nil, apperr.ErrIdentityImportUnsupportedFile
	}
}

// parseOrgImportCSV 解析组织导入 CSV,表头后每行格式为 kind,name,code_or_parent_id,enrollment_year。
func parseOrgImportCSV(raw []byte) ([]orgImportRow, []ImportRowResult, error) {
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, apperr.ErrIdentityImportCSVFormatInvalid
	}
	if len(records) < 2 {
		return nil, nil, apperr.ErrIdentityImportEmpty
	}
	return parseOrgImportRecords(records)
}

// parseOrgImportXLSX 解析组织导入 Excel,只读取模板工作表或首个工作表。
func parseOrgImportXLSX(raw []byte) ([]orgImportRow, []ImportRowResult, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, apperr.ErrIdentityImportCSVFormatInvalid.WithCause(err)
	}
	sheet := importSheetOrg
	if index, err := workbook.GetSheetIndex(sheet); err != nil || index < 0 {
		sheets := workbook.GetSheetList()
		if len(sheets) == 0 {
			if closeErr := workbook.Close(); closeErr != nil {
				return nil, nil, apperr.ErrInternal.WithCause(closeErr)
			}
			return nil, nil, apperr.ErrIdentityImportEmpty
		}
		sheet = sheets[0]
	}
	records, err := workbook.GetRows(sheet)
	if err != nil {
		if closeErr := workbook.Close(); closeErr != nil {
			return nil, nil, apperr.ErrInternal.WithCause(closeErr)
		}
		return nil, nil, apperr.ErrIdentityImportCSVFormatInvalid.WithCause(err)
	}
	if err := workbook.Close(); err != nil {
		return nil, nil, apperr.ErrInternal.WithCause(err)
	}
	if len(records) < 2 {
		return nil, nil, apperr.ErrIdentityImportEmpty
	}
	return parseOrgImportRecords(records)
}

// parseOrgImportRecords 把 CSV/XLSX 的二维记录统一转换为组织导入行。
func parseOrgImportRecords(records [][]string) ([]orgImportRow, []ImportRowResult, error) {
	rows := make([]orgImportRow, 0, len(records)-1)
	results := make([]ImportRowResult, 0, len(records)-1)
	for i, record := range records[1:] {
		row := orgImportRow{Line: i + 2}
		result := ImportRowResult{Line: row.Line}
		// 模板解析阶段只做格式和字段完整性校验,数据库存在性留给预览事务统一处理。
		if len(record) < 3 {
			result.Error = "请按模板填写组织类型、名称和编码或上级 ID"
			row.Error = result.Error
			rows = append(rows, row)
			results = append(results, result)
			continue
		}
		row.Kind = strings.ToLower(strings.TrimSpace(record[0]))
		row.Name = strings.TrimSpace(record[1])
		if row.Kind == "department" {
			// 院系没有父级,第三列按院系编码解释。
			row.Code = strings.TrimSpace(record[2])
			if err := validateDepartmentRequest(DepartmentRequest{Name: row.Name, Code: row.Code}); err != nil {
				result.Error = "院系名称和编码不能为空"
			}
		} else {
			// 专业和班级第三列都是上级 ID,后续按 kind 区分院系或专业。
			parentID, err := strconv.ParseInt(strings.TrimSpace(record[2]), 10, 64)
			row.ParentID = parentID
			if err != nil || row.ParentID <= 0 {
				result.Error = "上级组织 ID 不正确"
			}
			if row.Kind == "class" {
				if len(record) < 4 {
					result.Error = "班级必须填写入学年份"
				} else {
					year, err := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 16)
					row.EnrollmentYear = int16(year)
					if err != nil || row.EnrollmentYear <= 0 {
						result.Error = "入学年份不正确"
					}
				}
			}
			if row.Kind != "major" && row.Kind != "class" {
				result.Error = "组织类型不正确"
			}
			if strings.TrimSpace(row.Name) == "" {
				result.Error = "组织名称不能为空"
			}
		}
		row.Error = result.Error
		rows = append(rows, row)
		results = append(results, result)
	}
	return rows, results, nil
}

// encodeNamedXLSX 使用指定工作表名生成真实 xlsx 模板。
func encodeNamedXLSX(sheetName string, records [][]string) ([]byte, error) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", sheetName); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	for rowIndex, record := range records {
		for colIndex, value := range record {
			cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			if err != nil {
				return nil, apperr.ErrInternal.WithCause(err)
			}
			if err := workbook.SetCellValue(sheetName, cell, value); err != nil {
				return nil, apperr.ErrInternal.WithCause(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := workbook.Write(&buf); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	if err := workbook.Close(); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	return buf.Bytes(), nil
}

// PromoteClassesByAdmin 批量升级当前租户正常班级。
func (s *Service) PromoteClassesByAdmin(ctx context.Context) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.PromoteClasses(ctx, id.TenantID); err != nil {
			return err
		}
		// 班级升级会批量改变组织口径,必须与变更一起写审计,避免后续无法追踪批量来源。
		return s.writeOrgAuditInTx(ctx, tx, id, "org.class.promote", "identity.class", 0, map[string]any{})
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return nil
}

// validateOrgImportParents 在组织导入预览阶段校验上级组织存在性,保证用户提交前能看到逐行原因。
func (s *Service) validateOrgImportParents(ctx context.Context, tx TxStore, tenantID int64, rows []orgImportRow, results []ImportRowResult) error {
	for i := range rows {
		if rows[i].Error != "" {
			continue
		}
		switch rows[i].Kind {
		case "major":
			ok, err := tx.DepartmentExists(ctx, tenantID, rows[i].ParentID)
			if err != nil {
				return err
			}
			if !ok {
				rows[i].Error = "上级院系不存在"
				results[i].Error = rows[i].Error
			}
		case "class":
			ok, err := tx.MajorExists(ctx, tenantID, rows[i].ParentID)
			if err != nil {
				return err
			}
			if !ok {
				rows[i].Error = "上级专业不存在"
				results[i].Error = rows[i].Error
			}
		}
	}
	return nil
}

// writeOrgAuditInTx 在组织架构事务内写审计,确保敏感变更与审计记录同成同败。
func (s *Service) writeOrgAuditInTx(ctx context.Context, tx TxStore, id tenant.Identity, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := audit.BuildEntry(ctx, id.TenantID, id.AccountID, contracts.RoleNumSchoolAdmin, action, targetType, targetID, detail)
	if err != nil {
		return err
	}
	return tx.WriteAudit(ctx, WriteAuditInput{
		ID:         s.ids.Generate(),
		TenantID:   entry.TenantID,
		ActorID:    entry.ActorID,
		ActorRole:  entry.ActorRole,
		Action:     entry.Action,
		TargetType: entry.TargetType,
		TargetID:   entry.TargetID,
		Detail:     []byte(entry.Detail),
		IP:         entry.IP,
		TraceID:    entry.TraceID,
	})
}

// validateMajorParent 校验专业所属院系属于当前租户,补足普通外键无法表达的租户一致性。
func validateMajorParent(ctx context.Context, tx TxStore, tenantID, departmentID int64) error {
	ok, err := tx.DepartmentExists(ctx, tenantID, departmentID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrIdentityOrgInvalidInput
	}
	return nil
}

// validateClassParent 校验班级所属专业属于当前租户,避免跨租户父级 ID 被普通外键接受。
func validateClassParent(ctx context.Context, tx TxStore, tenantID, majorID int64) error {
	ok, err := tx.MajorExists(ctx, tenantID, majorID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrIdentityOrgInvalidInput
	}
	return nil
}

// validateDepartmentRequest 校验院系输入的必填字段。
func validateDepartmentRequest(req DepartmentRequest) error {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Code) == "" {
		return apperr.ErrIdentityOrgInvalidInput
	}
	return nil
}

// validateMajorRequest 校验专业输入的必填字段。
func validateMajorRequest(req MajorRequest) error {
	if req.DepartmentID <= 0 || strings.TrimSpace(req.Name) == "" {
		return apperr.ErrIdentityOrgInvalidInput
	}
	return nil
}

// validateClassRequest 校验班级输入和状态机取值。
func validateClassRequest(req ClassRequest) error {
	if req.MajorID <= 0 || strings.TrimSpace(req.Name) == "" || req.EnrollmentYear <= 0 {
		return apperr.ErrIdentityOrgInvalidInput
	}
	if req.Status != 0 && req.Status != ClassStatusActive && req.Status != ClassStatusArchived {
		return apperr.ErrIdentityOrgInvalidInput
	}
	return nil
}

// normalizeDepartmentRequest 清理院系输入空白。
func normalizeDepartmentRequest(req DepartmentRequest) DepartmentRequest {
	return DepartmentRequest{Name: strings.TrimSpace(req.Name), Code: strings.TrimSpace(req.Code)}
}

// normalizeMajorRequest 清理专业输入空白。
func normalizeMajorRequest(req MajorRequest) MajorRequest {
	return MajorRequest{DepartmentID: req.DepartmentID, Name: strings.TrimSpace(req.Name)}
}

// normalizeClassRequest 清理班级输入空白并补齐默认状态。
func normalizeClassRequest(req ClassRequest) ClassRequest {
	status := req.Status
	if status == 0 {
		status = ClassStatusActive
	}
	return ClassRequest{MajorID: req.MajorID, Name: strings.TrimSpace(req.Name), EnrollmentYear: req.EnrollmentYear, Status: status}
}
