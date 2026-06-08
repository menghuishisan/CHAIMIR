// M1 账号导入服务:预览(不落库)+ 提交(仅写校验通过行)。两步流程。
// 依据 docs/01 §3 接口、§5 §2 导入流程(逐行校验,错误收集到 import_batch.error_detail)。
package identity

import (
	"context"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// PreviewImport 导入预览:逐行校验,返回结果但不落库。
func (s *Service) PreviewImport(ctx context.Context, req ImportRequest) (*ImportPreviewResult, error) {
	if err := ensureImportRowsLimit(req.Rows, s.importMaxRows); err != nil {
		return nil, err
	}
	result := &ImportPreviewResult{Total: len(req.Rows)}
	seen := map[string]bool{} // 文件内手机号与学号/工号去重检查。

	// 在租户上下文内逐行校验(需查组织存在性、库内唯一性)。
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		for i, row := range req.Rows {
			line := i + 1
			if msg := s.validateImportRow(ctx, q, req.TargetType, row, seen); msg != "" {
				result.Invalid++
				result.Rows = append(result.Rows, ImportPreviewRow{Line: line, Error: msg})
			} else {
				result.Valid++
				result.Rows = append(result.Rows, ImportPreviewRow{Line: line})
			}
		}
		return nil
	}); err != nil {
		return nil, apperr.ErrImportPreviewReadFailed.WithCause(err)
	}
	return result, nil
}

// CreateImportPreview 导入预览:校验文件行并把预览状态持久化到服务端。
func (s *Service) CreateImportPreview(ctx context.Context, operatorID int64, req ImportRequest) (*ImportPreviewResult, error) {
	result, err := s.PreviewImport(ctx, req)
	if err != nil {
		return nil, err
	}
	tenantID := tenantFromCtx(ctx)
	previewID := s.idgen.Generate()
	rowsJSON, err := marshalImportRows(req.Rows)
	if err != nil {
		return nil, toAppErr(err)
	}
	result.PreviewID = ids.Format(previewID)
	resultJSON, err := marshalImportPreviewResult(result)
	if err != nil {
		return nil, toAppErr(err)
	}

	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, e := q.CreateImportPreview(ctx, sqlcgen.CreateImportPreviewParams{
			ID: previewID, TenantID: tenantID, OperatorID: operatorID, TargetType: req.TargetType,
			FileName: req.FileName, Rows: rowsJSON, PreviewResult: resultJSON,
			ExpireAt: timex.RequiredTimestamptz(timex.Now().Add(s.importPreviewTTL)),
		})
		return e
	}); err != nil {
		return nil, apperr.ErrImportPreviewStoreFailed.WithCause(err)
	}
	return result, nil
}

// CommitImportPreview 导入提交:读取服务端预览结果,仅写入仍然校验通过的行。
func (s *Service) CommitImportPreview(ctx context.Context, operatorID int64, req ImportCommitRequest) (*ImportCommitResult, error) {
	previewID, ok := ids.Parse(req.PreviewID)
	if !ok {
		return nil, apperr.ErrImportCommitInvalid
	}
	tenantID := tenantFromCtx(ctx)
	enableActivationCode, err := s.activationCodeEnabled(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	batchID := s.idgen.Generate()
	var result ImportCommitResult
	var errDetails []rowErr
	var activationCodes []ActivationCodeIssued
	var initPasswords []InitialPasswordIssued
	var total, success, failed int

	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.GetPendingImportPreview(ctx, sqlcgen.GetPendingImportPreviewParams{ID: previewID, OperatorID: operatorID})
		if e != nil {
			return apperr.ErrImportPreviewNotFound
		}
		rows, e := unmarshalImportRows(row.Rows)
		if e != nil {
			return e
		}
		if err := ensureImportRowsLimit(rows, s.importMaxRows); err != nil {
			return err
		}
		total = len(rows)
		seen := map[string]bool{}
		for i, importRow := range rows {
			line := i + 1
			if msg := s.validateImportRow(ctx, q, row.TargetType, importRow, seen); msg != "" {
				failed++
				errDetails = append(errDetails, rowErr{Line: line, Error: msg})
				continue
			}
			issued, e := s.insertImportRow(ctx, q, tenantID, operatorID, row.TargetType, importRow, enableActivationCode)
			if e != nil {
				failed++
				errDetails = append(errDetails, rowErr{Line: line, Error: "写入失败"})
				continue
			}
			if issued.ActivationCode != "" {
				activationCodes = append(activationCodes, issued)
			}
			if issued.InitPassword != "" {
				initPasswords = append(initPasswords, InitialPasswordIssued{
					AccountID:    issued.AccountID,
					InitPassword: issued.InitPassword,
				})
			}
			success++
		}
		detailJSON, e := marshalImportRowErrors(errDetails)
		if e != nil {
			return e
		}
		if _, e := q.CreateImportBatch(ctx, sqlcgen.CreateImportBatchParams{
			ID: batchID, TenantID: tenantID, OperatorID: operatorID, TargetType: row.TargetType,
			FileName: row.FileName, Total: int32(total), Success: int32(success), Failed: int32(failed),
			ErrorDetail: detailJSON, Status: ImportDone,
		}); e != nil {
			return e
		}
		if e := s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionAccountImport, AuditTargetImportBatch, batchID, map[string]any{
			"target_type":            row.TargetType,
			"total":                  total,
			"success":                success,
			"failed":                 failed,
			"enable_activation_code": enableActivationCode,
			"preview_id":             ids.Format(previewID),
		}); e != nil {
			return e
		}
		if e := q.MarkImportPreviewSubmitted(ctx, sqlcgen.MarkImportPreviewSubmittedParams{ID: previewID, OperatorID: operatorID}); e != nil {
			return e
		}
		result = ImportCommitResult{
			BatchID: ids.Format(batchID), Total: total, Success: success, Failed: failed,
			ActivationCodes: activationCodes,
			InitPasswords:   initPasswords,
		}
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrImportPreviewReadFailed.WithCause(err)
	}
	return &result, nil
}

// ensureImportRowsLimit 统一校验导入行数,用于文件解析后和提交前双重保护。
func ensureImportRowsLimit(rows []ImportRowInput, maxRows int) error {
	if len(rows) == 0 {
		return apperr.ErrImportEmpty
	}
	if len(rows) > maxRows {
		return apperr.ErrImportTooLarge
	}
	return nil
}

// rowErr 是导入提交落库的逐行错误结构。
type rowErr struct {
	Line  int    `json:"line"`
	Error string `json:"error"`
}

// marshalImportRowErrors 序列化导入提交错误明细;无错误时保存空数组。
func marshalImportRowErrors(rows []rowErr) ([]byte, error) {
	if rows == nil {
		rows = []rowErr{}
	}
	return jsonx.AnyBytes(rows, apperr.ErrImportRowsInvalid)
}

// marshalImportRows 序列化导入文件行,用于预览状态服务端持久化。
func marshalImportRows(rows []ImportRowInput) ([]byte, error) {
	return jsonx.AnyBytes(rows, apperr.ErrImportRowsInvalid)
}

// unmarshalImportRows 读取服务端暂存的导入文件行。
func unmarshalImportRows(data []byte) ([]ImportRowInput, error) {
	var rows []ImportRowInput
	if err := jsonx.DecodeStrict(data, &rows); err != nil {
		return nil, apperr.ErrImportRowsInvalid.WithCause(err)
	}
	return rows, nil
}

// marshalImportPreviewResult 序列化逐行预览结果,用于刷新后恢复确认页面。
func marshalImportPreviewResult(result *ImportPreviewResult) ([]byte, error) {
	return jsonx.AnyBytes(result, apperr.ErrImportRowsInvalid)
}

// unmarshalImportPreviewResult 读取服务端暂存的预览结果。
func unmarshalImportPreviewResult(data []byte) (*ImportPreviewResult, error) {
	var result ImportPreviewResult
	if err := jsonx.DecodeStrict(data, &result); err != nil {
		return nil, apperr.ErrImportRowsInvalid.WithCause(err)
	}
	return &result, nil
}

// ListImportBatches 查询导入批次历史。
func (s *Service) ListImportBatches(ctx context.Context, page, size int) ([]ImportBatchView, int64, error) {
	var views []ImportBatchView
	var total int64
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		cnt, e := q.CountImportBatches(ctx)
		if e != nil {
			return e
		}
		total = cnt
		rows, e := q.ListImportBatches(ctx, sqlcgen.ListImportBatchesParams{
			Limit:  int32(size),
			Offset: int32((page - 1) * size),
		})
		if e != nil {
			return e
		}
		for _, row := range rows {
			views = append(views, ImportBatchView{
				ID:         ids.Format(row.ID),
				OperatorID: ids.Format(row.OperatorID),
				TargetType: row.TargetType,
				FileName:   row.FileName,
				Total:      row.Total,
				Success:    row.Success,
				Failed:     row.Failed,
				Status:     row.Status,
				CreatedAt:  timex.FromTimestamptz(row.CreatedAt).Format(time.RFC3339),
			})
		}
		return nil
	}); err != nil {
		return nil, 0, apperr.ErrImportBatchQueryFailed.WithCause(err)
	}
	return views, total, nil
}

// validateImportRow 校验单行;返回错误文案(空串=通过)。
func (s *Service) validateImportRow(ctx context.Context, q *sqlcgen.Queries, targetType int16, row ImportRowInput, seen map[string]bool) string {
	if targetType != ImportTargetTeacher && targetType != ImportTargetStudent {
		return "导入类型不正确"
	}
	if row.Phone == "" || row.Name == "" || row.No == "" || row.OrgID == "" {
		return "必填项缺失(手机号/姓名/学工号/组织)"
	}
	if !validCNPhone(row.Phone) {
		return "手机号格式不正确"
	}
	phoneKey := "phone:" + row.Phone
	if seen[phoneKey] {
		return "文件内手机号重复"
	}
	seen[phoneKey] = true
	noKey := "no:" + row.No
	if seen[noKey] {
		return "文件内学工号重复"
	}
	seen[noKey] = true

	orgID, ok := ids.Parse(row.OrgID)
	if !ok {
		return "组织 ID 非法"
	}
	// 组织存在性(学生→班级,教师→院系)。
	if targetType == ImportTargetStudent {
		if _, e := q.GetClassByID(ctx, orgID); e != nil {
			return "班级不存在"
		}
	} else {
		if _, e := q.GetDepartmentByID(ctx, orgID); e != nil {
			return "院系不存在"
		}
	}
	// 库内学工号唯一。
	if _, e := q.GetAccountProfileByNo(ctx, row.No); e == nil {
		return "学工号已存在"
	}
	return ""
}

// insertImportRow 写入单个导入账号,并返回一次性开通凭据。
func (s *Service) insertImportRow(ctx context.Context, q *sqlcgen.Queries, tenantID, operatorID int64, targetType int16, row ImportRowInput, enableActivationCode bool) (ActivationCodeIssued, error) {
	baseIdentity := BaseIdentityStudent
	baseRole := RoleStudent
	if targetType == ImportTargetTeacher {
		baseIdentity = BaseIdentityTeacher
		baseRole = RoleTeacher
	}
	orgID, _ := ids.Parse(row.OrgID)
	accountID := s.idgen.Generate()
	initPlain, err := s.genTempPassword()
	if err != nil {
		return ActivationCodeIssued{}, apperr.ErrAccountCredentialFailed.WithCause(err)
	}

	credential, err := buildAccountOpeningCredential(enableActivationCode, initPlain)
	if err != nil {
		return ActivationCodeIssued{}, err
	}
	phoneEnc, err := s.encryptPhone(row.Phone)
	if err != nil {
		return ActivationCodeIssued{}, err
	}
	if _, err := q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
		ID: accountID, TenantID: tenantID, PhoneEnc: phoneEnc, PhoneHash: s.phoneHash(row.Phone),
		PasswordHash: credential.PasswordHash, Name: row.Name, BaseIdentity: baseIdentity,
		Status: AccountPending, MustChangePwd: credential.MustChangePassword,
	}); err != nil {
		return ActivationCodeIssued{}, err
	}
	if _, err := q.CreateAccountProfile(ctx, sqlcgen.CreateAccountProfileParams{
		AccountID: accountID, TenantID: tenantID, No: row.No, OrgID: orgID,
		EnrollmentYear: pgInt2(row.EnrollmentYear, targetType == ImportTargetStudent),
		Title:          pgText(row.Title),
	}); err != nil {
		return ActivationCodeIssued{}, err
	}
	if err := q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{
		ID: s.idgen.Generate(), TenantID: tenantID, AccountID: accountID, Role: baseRole,
	}); err != nil {
		return ActivationCodeIssued{}, err
	}
	issued := ActivationCodeIssued{AccountID: ids.Format(accountID)}
	if credential.NeedsActivationCode {
		code, err := s.CreateActivationCode(ctx, q, tenantID, accountID, operatorID)
		if err != nil {
			return ActivationCodeIssued{}, err
		}
		issued.ActivationCode = code
		return issued, nil
	}
	issued.InitPassword = credential.InitPassword
	return issued, nil
}
