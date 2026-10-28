// identity service_import_preview统一账号与组织导入预览的发现、恢复和明确放弃。
package identity

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// importPreviewResponse只解码服务端校验结果,从不把含密码的原始rows送入响应。
func importPreviewResponse(preview ImportPreview) (ImportPreviewResponse, error) {
	var rows []ImportRowResult
	if err := jsonx.DecodeStrictKnownFields(preview.PreviewResult, &rows); err != nil {
		return ImportPreviewResponse{}, apperr.ErrInternal.WithCause(err)
	}
	if rows == nil {
		rows = []ImportRowResult{}
	}
	valid := 0
	for _, row := range rows {
		if row.Error == "" {
			valid++
		}
	}
	return ImportPreviewResponse{ImportPreviewSummaryDTO: ImportPreviewSummaryDTO{PreviewID: ids.ID(preview.ID), TargetType: preview.TargetType, FileName: preview.FileName, ExpireAt: timex.RFC3339OrEmpty(preview.ExpireAt), Total: len(rows), Valid: valid, Invalid: len(rows) - valid}, Rows: rows}, nil
}

// ListPendingImportPreviews按当前租户/本人和目标分页发现尚未提交的预览。
func (s *Service) ListPendingImportPreviews(ctx context.Context, targetType int16, page, size int) ([]ImportPreviewSummaryDTO, int64, int, int, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if targetType != ImportTargetTeacher && targetType != ImportTargetStudent && targetType != ImportTargetOrg {
		return nil, 0, 0, 0, apperr.ErrIdentityImportTypeInvalid
	}
	page, size = pagex.Normalize(page, size)
	items := []ImportPreviewSummaryDTO{}
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		previews, count, err := tx.ListPendingImportPreviews(ctx, id.TenantID, id.AccountID, targetType, page, size)
		if err != nil {
			return err
		}
		total = count
		for _, preview := range previews {
			dto, err := importPreviewResponse(preview)
			if err != nil {
				return err
			}
			items = append(items, dto.ImportPreviewSummaryDTO)
		}
		return nil
	}); err != nil {
		return nil, 0, 0, 0, apperr.AsAppError(err)
	}
	return items, total, page, size, nil
}

// GetImportPreviewByAdmin恢复本人有效预览,不接受客户端提供租户或操作人。
func (s *Service) GetImportPreviewByAdmin(ctx context.Context, previewID int64) (ImportPreviewResponse, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return ImportPreviewResponse{}, err
	}
	var result ImportPreviewResponse
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		preview, err := tx.GetImportPreview(ctx, id.TenantID, id.AccountID, previewID)
		if err != nil {
			return err
		}
		if preview.Status != ImportPreviewPending || !timex.Now().Before(preview.ExpireAt) {
			return apperr.ErrIdentityImportPreviewExpired
		}
		result, err = importPreviewResponse(preview)
		return err
	}); err != nil {
		if isNoRows(err) {
			return ImportPreviewResponse{}, apperr.ErrIdentityImportPreviewExpired
		}
		return ImportPreviewResponse{}, apperr.AsAppError(err)
	}
	return result, nil
}

// DiscardImportPreviewByAdmin在删除待提交预览的同一事务内写唯一审计表。
func (s *Service) DiscardImportPreviewByAdmin(ctx context.Context, previewID int64) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	return s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.DiscardImportPreview(ctx, id.TenantID, id.AccountID, previewID); err != nil {
			return apperr.AsAppError(err)
		}
		entry, err := audit.BuildEntry(ctx, id.TenantID, id.AccountID, contracts.RoleNumSchoolAdmin, "import.preview.discard", "identity.import_preview", previewID, map[string]any{})
		if err != nil {
			return err
		}
		return tx.WriteAudit(ctx, WriteAuditInput{ID: s.ids.Generate(), TenantID: entry.TenantID, ActorID: entry.ActorID, ActorRole: entry.ActorRole, Action: entry.Action, TargetType: entry.TargetType, TargetID: entry.TargetID, Detail: []byte(entry.Detail), IP: entry.IP, TraceID: entry.TraceID})
	})
}
