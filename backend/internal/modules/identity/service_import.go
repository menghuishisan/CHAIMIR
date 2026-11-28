// identity service_import 文件实现账号 CSV/XLSX 导入预览、模板生成和提交,预览中间态持久化到服务端。
package identity

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/logging"
	"github.com/xuri/excelize/v2"
)

const (
	importTemplateFormatCSV  = "csv"
	importTemplateFormatXLSX = "xlsx"
	importSheetAccounts      = "accounts"
)

// ImportTemplate 生成账号导入模板文件,默认 Excel,也支持文档声明的 CSV。
func (s *Service) ImportTemplate(targetType int16, format string) (ImportTemplateFile, error) {
	normalizedFormat := strings.ToLower(strings.TrimSpace(format))
	if normalizedFormat == "" {
		normalizedFormat = importTemplateFormatXLSX
	}
	records, filePrefix, err := importTemplateRecords(targetType)
	if err != nil {
		return ImportTemplateFile{}, err
	}
	switch normalizedFormat {
	case importTemplateFormatCSV:
		content, err := encodeImportCSV(records)
		if err != nil {
			return ImportTemplateFile{}, err
		}
		return ImportTemplateFile{FileName: filePrefix + ".csv", ContentType: "text/csv; charset=utf-8", Content: content}, nil
	case importTemplateFormatXLSX:
		content, err := encodeNamedXLSX(importSheetAccounts, records)
		if err != nil {
			return ImportTemplateFile{}, err
		}
		return ImportTemplateFile{FileName: filePrefix + ".xlsx", ContentType: upload.XLSXContentType, Content: content}, nil
	default:
		return ImportTemplateFile{}, apperr.ErrIdentityImportFormatInvalid
	}
}

// importTemplateRecords 返回不同导入目标的模板表头和示例行。
func importTemplateRecords(targetType int16) ([][]string, string, error) {
	switch targetType {
	case ImportTargetTeacher:
		return [][]string{
			{"phone", "name", "no", "org_id", "title", "initial_password"},
			{"13800000000", "张老师", "T001", "1001", "讲师", ""},
		}, "teacher_import_template", nil
	case ImportTargetStudent:
		return [][]string{
			{"phone", "name", "no", "org_id", "enrollment_year", "initial_password"},
			{"13800000001", "张三", "20240001", "2001", "2024", ""},
		}, "student_import_template", nil
	default:
		return nil, "", apperr.ErrIdentityImportTypeInvalid
	}
}

// encodeImportCSV 使用标准库生成 CSV 模板,避免手写转义遗漏。
func encodeImportCSV(records [][]string) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.WriteAll(records); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	return buf.Bytes(), nil
}

// encodeNamedXLSX 使用指定工作表名生成账号与组织导入模板。
func encodeNamedXLSX(sheetName string, records [][]string) (content []byte, err error) {
	workbook := excelize.NewFile()
	defer func() {
		if closeErr := workbook.Close(); closeErr != nil {
			content = nil
			err = apperr.ErrInternal.WithCause(errors.Join(err, closeErr))
		}
	}()
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
	return buf.Bytes(), nil
}

type importRow struct {
	Line  int    `json:"line"`
	Phone string `json:"phone"`
	Name  string `json:"name"`
	No    string `json:"no"`
	// OrgID 允许为空,这样组织 ID 无效的错误行仍可持久化为预览结果。
	OrgID           *ids.ID `json:"org_id,omitempty"`
	BaseIdentity    int16   `json:"base_identity"`
	EnrollmentYear  int16   `json:"enrollment_year,omitempty"`
	Title           string  `json:"title,omitempty"`
	InitialPassword string  `json:"-"`
	PasswordHash    string  `json:"password_hash,omitempty"`
	Error           string  `json:"error,omitempty"`
}

// PreviewAccountImport 解析上传文件并把预览行和校验结果持久化到 import_preview。
func (s *Service) PreviewAccountImport(ctx context.Context, req ImportPreviewRequest) (ImportPreviewResponse, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return ImportPreviewResponse{}, err
	}
	if req.TargetType != ImportTargetTeacher && req.TargetType != ImportTargetStudent {
		return ImportPreviewResponse{}, apperr.ErrIdentityImportTypeInvalid
	}
	rows, results, err := s.parseImportFile(ctx, req.Content, req.FileName, req.ContentType, req.TargetType)
	if err != nil {
		return ImportPreviewResponse{}, err
	}
	// 解析后再进入租户内校验,确保文件格式问题和业务唯一性问题分别给出用户向错误。
	if err := s.applyAccountImportOpeningRules(ctx, id.TenantID, rows, results); err != nil {
		return ImportPreviewResponse{}, err
	}
	// 预览结果必须服务端持久化,保证刷新页面或换设备后仍能继续提交同一批次。
	rowsJSON, err := jsonx.AnyBytes(rows, apperr.ErrInternal)
	if err != nil {
		return ImportPreviewResponse{}, err
	}
	resultJSON, err := jsonx.AnyBytes(results, apperr.ErrInternal)
	if err != nil {
		return ImportPreviewResponse{}, err
	}
	previewID := s.ids.Generate()
	var saved ImportPreview
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		saved, err = tx.CreateImportPreview(ctx, CreateImportPreviewInput{
			ID:            previewID,
			TenantID:      id.TenantID,
			OperatorID:    id.AccountID,
			TargetType:    req.TargetType,
			FileName:      req.FileName,
			Rows:          rowsJSON,
			PreviewResult: resultJSON,
			ExpireAt:      timex.Now().Add(time.Duration(s.cfg.ImportPreviewTTLHours) * time.Hour),
		})
		return err
	}); err != nil {
		return ImportPreviewResponse{}, apperr.ErrInternal.WithCause(err)
	}
	return importPreviewResponse(saved)
}

// CommitAccountImport 读取服务端预览并仅提交校验通过的行,激活码明文只在本次响应返回。
func (s *Service) CommitAccountImport(ctx context.Context, req ImportCommitRequest) (AccountImportCommitResponse, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return AccountImportCommitResponse{}, err
	}
	var batch ImportBatch
	activationCodes := make([]ImportActivationCodeDTO, 0)
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		preview, err := tx.GetImportPreview(ctx, id.TenantID, id.AccountID, req.PreviewID.Int64())
		if err != nil {
			return err
		}
		if (preview.TargetType != ImportTargetTeacher && preview.TargetType != ImportTargetStudent) || preview.Status != ImportPreviewPending || !timex.Now().Before(preview.ExpireAt) {
			return apperr.ErrIdentityImportPreviewExpired
		}
		var rows []importRow
		if err := jsonx.DecodeStrictKnownFields(preview.Rows, &rows); err != nil {
			return fmt.Errorf("解析导入预览失败: %w", err)
		}
		currentTenant, err := tx.GetTenantByID(ctx, id.TenantID)
		if err != nil {
			return err
		}
		if !currentTenant.EnableActivationCode {
			for _, row := range rows {
				if row.Error == "" && strings.TrimSpace(row.PasswordHash) == "" {
					return apperr.ErrIdentityActivationDisabled
				}
			}
		}
		success, failed := int32(0), int32(0)
		for _, row := range rows {
			if row.Error != "" {
				failed++
				continue
			}
			passwordHash := row.PasswordHash
			mustChange := passwordHash != ""
			phoneEnc, phoneHash, err := s.protectPhone(row.Phone)
			if err != nil {
				return err
			}
			role, err := baseRoleNumber(row.BaseIdentity)
			if err != nil {
				return err
			}
			accountID := s.ids.Generate()
			var activation *ImportActivationCodeDTO
			rowErr, transactionErr := tx.ImportRowTx(ctx, func(ctx context.Context, rowTx TxStore) error {
				_, err := rowTx.CreateAccount(ctx, CreateAccountInput{
					ID:            accountID,
					TenantID:      id.TenantID,
					PhoneEnc:      phoneEnc,
					PhoneHash:     phoneHash,
					PasswordHash:  passwordHash,
					Name:          row.Name,
					BaseIdentity:  row.BaseIdentity,
					Status:        AccountStatusPending,
					MustChangePwd: mustChange,
					Roles:         []RoleCreateInput{{ID: s.ids.Generate(), Role: role}},
					Profile:       &CreateProfileInput{No: row.No, OrgID: row.OrgID.Int64(), EnrollmentYear: row.EnrollmentYear, Title: row.Title},
				})
				if err != nil {
					return err
				}
				if passwordHash == "" {
					code, err := s.createActivationCodeInTx(ctx, rowTx, id.TenantID, accountID, id.AccountID)
					if err != nil {
						return err
					}
					// 激活码明文只在本次响应返回,落库只保存哈希,后续无法从数据库恢复明文。
					activation = &ImportActivationCodeDTO{AccountID: ids.ID(accountID), No: row.No, Name: row.Name, ActivationCode: code}
				}
				return nil
			})
			if transactionErr != nil {
				return transactionErr
			}
			if rowErr != nil {
				failed++
				if idx := row.Line - 2; idx >= 0 && idx < len(rows) {
					rows[idx].Error = "账号写入失败,请检查是否与现有账号冲突"
				}
				logging.ErrorContext(ctx, "账号导入行写入失败", rowErr.Error(), slog.Int64("tenant_id", id.TenantID), slog.Int("line", row.Line))
				continue
			}
			if activation != nil {
				activationCodes = append(activationCodes, *activation)
			}
			success++
		}
		// 标记预览已提交后再写批次,防止同一个 preview_id 被重复消费。
		if err := tx.MarkImportPreviewSubmitted(ctx, id.TenantID, id.AccountID, req.PreviewID.Int64()); err != nil {
			return err
		}
		results := make([]ImportRowResult, len(rows))
		for i, row := range rows {
			results[i] = ImportRowResult{Line: row.Line, Error: row.Error}
		}
		errorDetail, err := jsonx.AnyBytes(results, apperr.ErrInternal)
		if err != nil {
			return err
		}
		total, totalOK := intx.Int32(len(rows))
		if !totalOK {
			return apperr.ErrIdentityImportContentInvalid
		}
		b, err := tx.CreateImportBatch(ctx, CreateImportBatchInput{
			ID:          s.ids.Generate(),
			TenantID:    id.TenantID,
			OperatorID:  id.AccountID,
			TargetType:  preview.TargetType,
			FileName:    preview.FileName,
			Total:       total,
			Success:     success,
			Failed:      failed,
			ErrorDetail: errorDetail,
			Status:      ImportBatchCompleted,
		})
		if err != nil {
			return err
		}
		batch = b
		entry, err := audit.BuildEntry(ctx, id.TenantID, id.AccountID, contracts.RoleNumSchoolAdmin, "account.import", "identity.import_batch", b.ID, map[string]any{"success": success, "failed": failed})
		if err != nil {
			return err
		}
		return s.auditWriter.writeInTx(ctx, tx, entry)
	}); err != nil {
		return AccountImportCommitResponse{}, apperr.AsAppError(err)
	}
	return AccountImportCommitResponse{Batch: ToImportBatchDTO(batch), ActivationCodes: activationCodes}, nil
}

// applyAccountImportOpeningRules 根据租户开通模式校验初始密码和激活码路径。
func (s *Service) applyAccountImportOpeningRules(ctx context.Context, tenantID int64, rows []importRow, results []ImportRowResult) error {
	var current Tenant
	seenNo := map[string]int{}
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		// 先读取租户开通策略,后续逐行判断是否允许缺省初始密码走激活码。
		tenantRow, err := tx.GetTenantByID(ctx, tenantID)
		if err != nil {
			return err
		}
		current = tenantRow
		for i := range rows {
			if rows[i].Error != "" {
				continue
			}
			// 导入预览阶段必须发现文件内重复,避免提交时部分成功后用户才知道同批冲突。
			no := strings.TrimSpace(rows[i].No)
			if firstLine, ok := seenNo[no]; ok {
				rows[i].Error = fmt.Sprintf("学号或工号与第 %d 行重复", firstLine)
				results[i].Error = rows[i].Error
				continue
			}
			seenNo[no] = rows[i].Line
			existing, err := tx.GetAccountByNo(ctx, no)
			if err != nil && !isNoRows(err) {
				return err
			}
			if err == nil && existing.ID > 0 {
				rows[i].Error = "学号或工号已存在"
				results[i].Error = rows[i].Error
				continue
			}
			// 组织挂靠校验复用单账号规则,保证导入和手动创建没有两套口径。
			ok, err := accountImportOrgExists(ctx, tx, tenantID, rows[i])
			if err != nil {
				return err
			}
			if !ok {
				rows[i].Error = "组织不存在或类型不正确"
				results[i].Error = rows[i].Error
			}
		}
		return nil
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	for i := range rows {
		password := strings.TrimSpace(rows[i].InitialPassword)
		rows[i].InitialPassword = ""
		if rows[i].Error != "" {
			continue
		}
		// 密码强度和激活码开关在数据库校验后执行,避免无效组织行继续暴露更多无关错误。
		if password == "" {
			if !current.EnableActivationCode {
				rows[i].Error = "请填写初始密码"
				results[i].Error = rows[i].Error
			}
			continue
		}
		if err := ValidatePassword(password); err != nil {
			rows[i].Error = "初始密码强度不足"
			results[i].Error = rows[i].Error
			continue
		}
		hash, err := crypto.HashPassword(password)
		if err != nil {
			return apperr.ErrInternal.WithCause(err)
		}
		rows[i].PasswordHash = hash
	}
	return nil
}

// accountImportOrgExists 校验导入账号挂靠的组织类型,教师挂院系,学生挂班级。
func accountImportOrgExists(ctx context.Context, tx TxStore, tenantID int64, row importRow) (bool, error) {
	if row.OrgID == nil {
		return false, nil
	}
	err := validateAccountOrgForProfile(ctx, tx, tenantID, row.BaseIdentity, row.OrgID.Int64(), row.EnrollmentYear)
	if err == nil {
		return true, nil
	}
	if _, ok := apperr.As(err); ok {
		return false, nil
	}
	return false, err
}

// parseImportFile 把已验证的表格记录转换为账号导入行。
func (s *Service) parseImportFile(ctx context.Context, raw []byte, fileName, contentType string, targetType int16) ([]importRow, []ImportRowResult, error) {
	records, err := s.readImportFile(ctx, raw, fileName, contentType, importSheetAccounts)
	if err != nil {
		return nil, nil, err
	}
	return s.parseImportRecords(records, targetType)
}

// readImportFile 共用账号与组织导入的文件边界、扫描及格式读取。
func (s *Service) readImportFile(ctx context.Context, raw []byte, fileName, contentType, sheet string) ([][]string, error) {
	if filepath.Base(fileName) != fileName || strings.TrimSpace(fileName) == "" {
		return nil, apperr.ErrIdentityImportUnsupportedFile
	}
	switch upload.CheckSize(int64(len(raw)), s.uploadCfg.ImportMaxBytes) {
	case upload.SizeEmpty:
		return nil, apperr.ErrIdentityImportContentInvalid
	case upload.SizeTooLarge:
		return nil, apperr.ErrIdentityImportFileTooLarge
	}
	if err := upload.VerifyScan(ctx, s.scanner, upload.ScanPolicy{Required: s.uploadCfg.VirusScanRequired}, upload.ScanRequest{FileName: fileName, Content: raw}); err != nil {
		return nil, apperr.ErrIdentityImportUnsupportedFile.WithCause(err)
	}
	var records [][]string
	var err error
	switch upload.CSVOrXLSXKind(fileName, contentType, raw) {
	case upload.KindCSV:
		records, err = readImportCSV(raw, s.cfg.ImportMaxRows)
	case upload.KindXLSX:
		records, err = readImportXLSX(ctx, raw, sheet, s.cfg.ImportMaxRows)
	default:
		return nil, apperr.ErrIdentityImportUnsupportedFile
	}
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, apperr.ErrIdentityImportEmpty
	}
	return records, nil
}

func readImportCSV(raw []byte, maxRows int) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.TrimLeadingSpace = true
	records := make([][]string, 0, min(maxRows+2, 128))
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return records, nil
		}
		if err != nil {
			return nil, apperr.ErrIdentityImportCSVFormatInvalid
		}
		records = append(records, record)
		if len(records) > maxRows+1 {
			return nil, apperr.ErrIdentityImportTooManyRows
		}
	}
}

func readImportXLSX(ctx context.Context, raw []byte, sheet string, maxRows int) (records [][]string, err error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, apperr.ErrIdentityImportCSVFormatInvalid.WithCause(err)
	}
	defer func() {
		if closeErr := workbook.Close(); closeErr != nil {
			records = nil
			err = apperr.ErrInternal.WithCause(errors.Join(err, closeErr))
		}
	}()
	if index, err := workbook.GetSheetIndex(sheet); err != nil || index < 0 {
		sheets := workbook.GetSheetList()
		if len(sheets) == 0 {
			return nil, apperr.ErrIdentityImportEmpty
		}
		sheet = sheets[0]
	}
	rows, err := workbook.Rows(sheet)
	if err != nil {
		return nil, apperr.ErrIdentityImportCSVFormatInvalid.WithCause(err)
	}
	defer logging.CloseContext(ctx, "导入工作簿行迭代器关闭失败", rows)
	records = make([][]string, 0, min(maxRows+2, 128))
	for rows.Next() {
		record, err := rows.Columns()
		if err != nil {
			return nil, apperr.ErrIdentityImportCSVFormatInvalid.WithCause(err)
		}
		records = append(records, record)
		if len(records) > maxRows+1 {
			return nil, apperr.ErrIdentityImportTooManyRows
		}
	}
	if err := rows.Error(); err != nil {
		return nil, apperr.ErrIdentityImportCSVFormatInvalid.WithCause(err)
	}
	return records, nil
}

// parseImportRecords 把 CSV/XLSX 的二维记录统一转换为导入行和用户向行级错误。
func (s *Service) parseImportRecords(records [][]string, targetType int16) ([]importRow, []ImportRowResult, error) {
	rows := make([]importRow, 0, len(records)-1)
	results := make([]ImportRowResult, 0, len(records)-1)
	for i, record := range records[1:] {
		row := importRow{Line: i + 2, BaseIdentity: BaseIdentityStudent}
		if targetType == ImportTargetTeacher {
			row.BaseIdentity = BaseIdentityTeacher
		}
		if len(record) < 4 {
			row.Error = "请按模板填写手机号、姓名、学号或工号、组织 ID"
		} else {
			row.Phone = strings.TrimSpace(record[0])
			row.Name = strings.TrimSpace(record[1])
			row.No = strings.TrimSpace(record[2])
			orgID, ok := ids.Parse(strings.TrimSpace(record[3]))
			if ok {
				parsedOrgID := ids.ID(orgID)
				row.OrgID = &parsedOrgID
			}
			if row.OrgID == nil {
				row.Error = "组织 ID 不正确"
			}
			if err := ValidatePhone(row.Phone); err != nil && row.Error == "" {
				row.Error = "手机号格式不正确"
			}
			if row.Name == "" || row.No == "" {
				row.Error = "姓名和学号或工号不能为空"
			}
			s.applyImportOptionalFields(record, targetType, &row)
		}
		rows = append(rows, row)
		results = append(results, ImportRowResult{Line: row.Line, Error: row.Error})
	}
	return rows, results, nil
}

// applyImportOptionalFields 读取教师职称或学生入学年份,行级错误不覆盖更早的基础字段错误。
func (s *Service) applyImportOptionalFields(record []string, targetType int16, row *importRow) {
	if len(record) < 5 {
		return
	}
	switch targetType {
	case ImportTargetTeacher:
		row.Title = strings.TrimSpace(record[4])
		if len(record) >= 6 {
			row.InitialPassword = strings.TrimSpace(record[5])
		}
	case ImportTargetStudent:
		year, err := strconv.ParseInt(strings.TrimSpace(record[4]), 10, 16)
		if err != nil || year < 1900 || year > 2200 {
			if row.Error == "" {
				row.Error = "入学年份不正确"
			}
			return
		}
		row.EnrollmentYear = int16(year)
		if len(record) >= 6 {
			row.InitialPassword = strings.TrimSpace(record[5])
		}
	}
}
