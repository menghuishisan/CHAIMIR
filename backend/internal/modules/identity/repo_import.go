// M1 导入数据访问:集中处理账号导入预览、提交、批次和逐行写账号事务。
package identity

import (
	"context"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// validateImportRowInStore 校验导入行依赖数据库的组织存在性和学工号唯一性。
func (r *repo) validateImportRowInStore(ctx context.Context, targetType int16, row ImportRowInput) string {
	orgID, ok := ids.Parse(row.OrgID)
	if !ok {
		return "组织 ID 非法"
	}
	var message string
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if targetType == ImportTargetStudent {
			if _, err := q.GetClassByID(ctx, orgID); err != nil {
				message = "班级不存在"
				return nil
			}
		} else {
			if _, err := q.GetDepartmentByID(ctx, orgID); err != nil {
				message = "院系不存在"
				return nil
			}
		}
		if _, err := q.GetAccountProfileByNo(ctx, row.No); err == nil {
			message = "学工号已存在"
			return nil
		} else if !db.IsNoRows(err) {
			return err
		}
		return nil
	}); err != nil {
		return "导入校验失败"
	}
	return message
}

// createImportPreview 持久化导入预览状态。
func (r *repo) createImportPreview(ctx context.Context, previewID, tenantID, operatorID int64, req ImportRequest, rowsJSON, resultJSON []byte, expireAt time.Time) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, err := q.CreateImportPreview(ctx, sqlcgen.CreateImportPreviewParams{
			ID: previewID, TenantID: tenantID, OperatorID: operatorID, TargetType: req.TargetType,
			FileName: req.FileName, Rows: rowsJSON, PreviewResult: resultJSON,
			ExpireAt: timex.RequiredTimestamptz(expireAt),
		})
		return err
	})
}

// getPendingImportPreviewForUpdate 读取并锁定待提交预览,防止重复提交或客户端篡改行数据。
func (r *repo) getPendingImportPreviewForUpdate(ctx context.Context, previewID, operatorID int64) (ImportPreviewSnapshot, error) {
	var row sqlcgen.ImportPreview
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.GetPendingImportPreview(ctx, sqlcgen.GetPendingImportPreviewParams{ID: previewID, OperatorID: operatorID})
		if err != nil {
			return apperr.ErrImportPreviewNotFound
		}
		row = found
		return nil
	}); err != nil {
		return ImportPreviewSnapshot{}, err
	}
	return ImportPreviewSnapshot{ID: row.ID, TargetType: row.TargetType, FileName: row.FileName, Rows: row.Rows}, nil
}

// commitImportPreviewWithAudit 写入导入账号、批次、审计并标记预览已提交。
func (r *repo) commitImportPreviewWithAudit(ctx context.Context, previewID, operatorID, batchID, tenantID int64, preview ImportPreviewSnapshot, rows []ImportAccountCreate, errorsJSON []byte, total, success, failed int, auditLog AuditLogCreate, nextID func() int64) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		// 再次锁定预览并确认仍可提交,使账号写入、批次和预览状态保持单事务一致。
		if _, err := q.GetPendingImportPreview(ctx, sqlcgen.GetPendingImportPreviewParams{ID: previewID, OperatorID: operatorID}); err != nil {
			return apperr.ErrImportPreviewNotFound
		}
		for _, row := range rows {
			if _, err := q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
				ID: row.AccountID, TenantID: tenantID, PhoneEnc: row.PhoneEnc, PhoneHash: row.PhoneHash,
				PasswordHash: row.PasswordHash, Name: row.Name, BaseIdentity: row.BaseIdentity,
				Status: AccountPending, MustChangePwd: row.MustChangePwd,
			}); err != nil {
				return err
			}
			if _, err := q.CreateAccountProfile(ctx, sqlcgen.CreateAccountProfileParams{
				AccountID: row.AccountID, TenantID: tenantID, No: row.No, OrgID: row.OrgID,
				EnrollmentYear: pgtypex.Int2When(row.EnrollmentYear, row.BaseIdentity == BaseIdentityStudent),
				Title:          pgtypex.Text(row.Title),
			}); err != nil {
				return err
			}
			if err := q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{ID: nextID(), TenantID: tenantID, AccountID: row.AccountID, Role: row.Role}); err != nil {
				return err
			}
			if row.HasActivation {
				if err := r.createActivationCodeInTx(ctx, q, nextID(), tenantID, row.AccountID, row.ActivationHash, row.ActivationAt, operatorID); err != nil {
					return err
				}
			}
		}
		if _, err := q.CreateImportBatch(ctx, sqlcgen.CreateImportBatchParams{
			ID: batchID, TenantID: tenantID, OperatorID: operatorID, TargetType: preview.TargetType,
			FileName: preview.FileName, Total: int32(total), Success: int32(success), Failed: int32(failed),
			ErrorDetail: errorsJSON, Status: ImportDone,
		}); err != nil {
			return err
		}
		if err := q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog)); err != nil {
			return err
		}
		return q.MarkImportPreviewSubmitted(ctx, sqlcgen.MarkImportPreviewSubmittedParams{ID: previewID, OperatorID: operatorID})
	})
}

// listImportBatches 分页读取导入批次。
func (r *repo) listImportBatches(ctx context.Context, page, size int) ([]ImportBatchSnapshot, int64, error) {
	var rows []sqlcgen.ImportBatch
	var total int64
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		count, err := q.CountImportBatches(ctx)
		if err != nil {
			return err
		}
		total = count
		found, err := q.ListImportBatches(ctx, sqlcgen.ListImportBatchesParams{Limit: int32(size), Offset: int32((page - 1) * size)})
		if err != nil {
			return err
		}
		rows = found
		return nil
	}); err != nil {
		return nil, 0, err
	}
	out := make([]ImportBatchSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, ImportBatchSnapshot{
			ID: row.ID, OperatorID: row.OperatorID, TargetType: row.TargetType, FileName: row.FileName,
			Total: row.Total, Success: row.Success, Failed: row.Failed, Status: row.Status,
			CreatedAt: timex.FromTimestamptz(row.CreatedAt),
		})
	}
	return out, total, nil
}
