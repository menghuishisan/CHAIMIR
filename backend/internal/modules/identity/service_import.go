// M1 账号导入服务:预览(不落库)+ 提交(仅写校验通过行)。两步流程。
// 依据 docs/01 §3 接口、§5 §2 导入流程(逐行校验,错误收集到 import_batch.error_detail)。
package identity

import (
	"context"
	"time"

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

	for i, row := range req.Rows {
		line := i + 1
		if msg := s.validateImportRow(ctx, req.TargetType, row, seen); msg != "" {
			result.Invalid++
			result.Rows = append(result.Rows, ImportPreviewRow{Line: line, Error: msg})
			continue
		}
		result.Valid++
		result.Rows = append(result.Rows, ImportPreviewRow{Line: line})
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

	if err := s.repo.createImportPreview(ctx, previewID, tenantID, operatorID, req, rowsJSON, resultJSON, timex.Now().Add(s.importPreviewTTL)); err != nil {
		return nil, apperr.ErrImportPreviewStoreFailed.WithCause(err)
	}
	return result, nil
}

// CommitImportPreview 提交服务端持久化的导入预览,逐行复验后只写入仍然合法的账号。
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
	var createRows []ImportAccountCreate

	// 重新读取服务端预览,防止客户端提交阶段篡改导入行或绕过过期校验。
	preview, err := s.repo.getPendingImportPreviewForUpdate(ctx, previewID, operatorID)
	if err != nil {
		return nil, toAppErrWith(err, apperr.ErrImportPreviewReadFailed)
	}
	rows, err := unmarshalImportRows(preview.Rows)
	if err != nil {
		return nil, toAppErr(err)
	}
	if err := ensureImportRowsLimit(rows, s.importMaxRows); err != nil {
		return nil, err
	}
	total = len(rows)
	seen := map[string]bool{}
	// 提交前逐行复验并生成持久化输入,账号写入仍由 repo 统一放进一个事务。
	for i, importRow := range rows {
		line := i + 1
		if msg := s.validateImportRow(ctx, preview.TargetType, importRow, seen); msg != "" {
			failed++
			errDetails = append(errDetails, rowErr{Line: line, Error: msg})
			continue
		}
		createRow, issued, err := s.buildImportAccountCreate(ctx, tenantID, operatorID, preview.TargetType, importRow, enableActivationCode)
		if err != nil {
			failed++
			errDetails = append(errDetails, rowErr{Line: line, Error: "写入失败"})
			continue
		}
		createRows = append(createRows, createRow)
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
	detailJSON, err := marshalImportRowErrors(errDetails)
	if err != nil {
		return nil, toAppErr(err)
	}
	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionAccountImport, AuditTargetImportBatch, batchID, map[string]any{
		"target_type":            preview.TargetType,
		"total":                  total,
		"success":                success,
		"failed":                 failed,
		"enable_activation_code": enableActivationCode,
		"preview_id":             ids.Format(previewID),
	})
	if err != nil {
		return nil, err
	}
	// 落库批次摘要和审计后再标记预览已提交,保证刷新页面也能追溯提交结果。
	if err := s.repo.commitImportPreviewWithAudit(ctx, previewID, operatorID, batchID, tenantID, preview, createRows, detailJSON, total, success, failed, buildAuditLogCreate(s.idgen.Generate(), entry), s.idgen.Generate); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrImportPreviewReadFailed.WithCause(err)
	}
	result = ImportCommitResult{
		BatchID: ids.Format(batchID), Total: total, Success: success, Failed: failed,
		ActivationCodes: activationCodes,
		InitPasswords:   initPasswords,
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
	rows, total, err := s.repo.listImportBatches(ctx, page, size)
	if err != nil {
		return nil, 0, apperr.ErrImportBatchQueryFailed.WithCause(err)
	}
	views := make([]ImportBatchView, 0, len(rows))
	for _, row := range rows {
		views = append(views, ImportBatchView{
			ID: ids.Format(row.ID), OperatorID: ids.Format(row.OperatorID), TargetType: row.TargetType,
			FileName: row.FileName, Total: row.Total, Success: row.Success, Failed: row.Failed,
			Status: row.Status, CreatedAt: row.CreatedAt.Format(time.RFC3339),
		})
	}
	return views, total, nil
}

// validateImportRow 校验单行基础格式和数据库约束;返回错误文案(空串=通过)。
func (s *Service) validateImportRow(ctx context.Context, targetType int16, row ImportRowInput, seen map[string]bool) string {
	if msg := validateImportRowBasic(targetType, row, seen); msg != "" {
		return msg
	}
	return s.repo.validateImportRowInStore(ctx, targetType, row)
}

// validateImportRowBasic 校验导入行的纯格式约束,不访问数据库。
func validateImportRowBasic(targetType int16, row ImportRowInput, seen map[string]bool) string {
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
	_ = orgID
	return ""
}

// buildImportAccountCreate 构造单个导入账号的持久化输入和一次性开通结果。
func (s *Service) buildImportAccountCreate(ctx context.Context, tenantID, operatorID int64, targetType int16, row ImportRowInput, enableActivationCode bool) (ImportAccountCreate, ActivationCodeIssued, error) {
	// 第一步按导入目标确定基础身份和角色,避免调用方自行映射角色码。
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
		return ImportAccountCreate{}, ActivationCodeIssued{}, apperr.ErrAccountCredentialFailed.WithCause(err)
	}

	// 第二步生成开通凭据并加密手机号,账号主表不保存明文敏感信息。
	credential, err := buildAccountOpeningCredential(enableActivationCode, initPlain)
	if err != nil {
		return ImportAccountCreate{}, ActivationCodeIssued{}, err
	}
	phoneEnc, err := s.encryptPhone(row.Phone)
	if err != nil {
		return ImportAccountCreate{}, ActivationCodeIssued{}, err
	}
	create := ImportAccountCreate{
		AccountID: accountID, PhoneEnc: phoneEnc, PhoneHash: s.phoneHash(row.Phone),
		PasswordHash: credential.PasswordHash, Name: row.Name, BaseIdentity: baseIdentity,
		MustChangePwd: credential.MustChangePassword, No: row.No, OrgID: orgID,
		EnrollmentYear: row.EnrollmentYear, Title: row.Title, Role: baseRole,
	}
	// 第三步按配置返回激活码或临时密码,调用方只负责聚合导入结果。
	issued := ActivationCodeIssued{AccountID: ids.Format(accountID)}
	if credential.NeedsActivationCode {
		code, err := s.genActivationCode()
		if err != nil {
			return ImportAccountCreate{}, ActivationCodeIssued{}, apperr.ErrActivationCodeIssueFailed.WithCause(err)
		}
		create.HasActivation = true
		create.ActivationHash = s.activationCodeHash(code)
		create.ActivationAt = timex.Now().Add(s.activationCodeTTL)
		issued.ActivationCode = code
		return create, issued, nil
	}
	issued.InitPassword = credential.InitPassword
	return create, issued, nil
}
