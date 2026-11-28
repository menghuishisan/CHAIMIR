// 持久化审计导出请求，使用统一任务租约生成和保存产物。
package admin

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/transfer"
	"chaimir/pkg/apperr"
)

const auditExportSubject = "admin.audit_export"

// ExportAuditCSV 创建可重放的审计导出任务，产物由 M9 worker 异步生成。
func (s *Service) ExportAuditCSV(ctx context.Context, query contracts.AuditQuery) (transfer.TaskDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return transfer.TaskDTO{}, err
	}
	query = scopedAuditQuery(id, query)
	snapshot, err := json.Marshal(query)
	if err != nil {
		return transfer.TaskDTO{}, apperr.ErrAdminAuditExportTaskCreateFailed.WithCause(err)
	}
	fileName := "audit.csv"
	var task transfer.Task
	if err := s.runWrite(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		task, err = s.transfers.CreateTask(ctx, transfer.NewTaskRequest{
			TenantID:    id.TenantID,
			AccountID:   id.AccountID,
			Channel:     transfer.ChannelExport,
			Subject:     auditExportSubject,
			FileName:    fileName,
			ContentType: "text/csv; charset=utf-8",
		})
		if err != nil {
			return err
		}
		if _, err := tx.CreateAuditExportRequest(ctx, AuditExportRequest{TransferTaskID: task.TaskID, TenantID: id.TenantID, AccountID: id.AccountID, QuerySnapshot: snapshot}); err != nil {
			return err
		}
		return s.writeAudit(ctx, id, "admin.audit.export", "audit_log", 0, map[string]any{"transfer_task_id": task.TaskID})
	}); err != nil {
		return transfer.TaskDTO{}, apperr.ErrAdminAuditExportTaskCreateFailed.WithCause(err)
	}
	return transfer.TaskToDTO(task), nil
}

// RunAuditExportOnce 执行一次 M9 审计导出请求扫描，供统一 background runner 调用。
func (s *Service) RunAuditExportOnce(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	limit, ok := intx.Int32(s.cfg.AuditExportWorkerBatchSize)
	if !ok || limit <= 0 {
		return apperr.ErrAdminAuditExportTaskCreateFailed
	}
	var requests []AuditExportRequest
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		requests, err = tx.ListDueAuditExportRequests(ctx, timex.Now(), limit)
		return err
	}); err != nil {
		return errors.Join(apperr.ErrAdminAuditExportTaskCreateFailed.WithCause(err), ctx.Err())
	}
	var failures error
	for _, req := range requests {
		if err := ctx.Err(); err != nil {
			return errors.Join(failures, err)
		}
		if err := s.runAuditExportRequest(ctx, req); err != nil {
			failures = errors.Join(failures, fmt.Errorf("导出审计失败 tenant_id=%d task_id=%d: %w", req.TenantID, req.TransferTaskID, err))
		}
	}
	return errors.Join(failures, ctx.Err())
}

// runAuditExportRequest claims one request's transfer task and generates its CSV artifact.
func (s *Service) runAuditExportRequest(ctx context.Context, req AuditExportRequest) error {
	task, err := s.transfers.ClaimTask(ctx, req.TenantID, req.TransferTaskID)
	if errors.Is(err, transfer.ErrTaskNotClaimable) {
		return s.reconcileAuditExportRequest(ctx, req)
	}
	if err != nil {
		return errors.Join(err, s.deferAuditExportRequest(ctx, req, timex.Now().Add(time.Duration(s.cfg.AuditExportWorkerPollMs)*time.Millisecond)))
	}
	var data []byte
	var plan storage.UploadPlan
	commitAttempted := false
	err = s.transfers.RunWithLease(ctx, task, func(workCtx context.Context) error {
		var err error
		data, err = s.buildAuditExportCSV(workCtx, req)
		if err != nil {
			return err
		}
		artifactFileName, err := transfer.LeaseArtifactFileName(task.FileName, task.LeaseToken)
		if err != nil {
			return err
		}
		plan, err = s.files.PlanUpload(workCtx, storage.PlanUploadRequest{TenantID: req.TenantID, AccountID: req.AccountID, AllowPlatformScope: req.TenantID == 0, Module: "transfer", ResourceType: string(transfer.ChannelExport), ResourceID: ids.Format(task.TaskID), FileName: artifactFileName, ContentType: task.ContentType, ExpectedBucket: s.storage.BucketReport(), AllowedFileName: true, Content: data})
		if err != nil {
			return err
		}
		return s.storage.Put(workCtx, plan.Bucket, plan.Key, bytes.NewReader(data), int64(len(data)), task.ContentType)
	})
	if err == nil {
		commitAttempted = true
		_, err = s.transfers.CompleteTask(ctx, task, transfer.CompleteTaskRequest{ObjectRef: plan.ObjectRef, Size: int64(len(data))})
	}
	if err != nil {
		if plan.Key != "" {
			err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, commitAttempted, func(ctx context.Context) error { return s.storage.Delete(ctx, plan.Bucket, plan.Key) })
		}
		return s.failAuditExportRequest(ctx, req, task, err)
	}
	return s.deleteAuditExportRequest(ctx, req)
}

// buildAuditExportCSV restores the persisted, already-authorized audit query for worker execution.
func (s *Service) buildAuditExportCSV(ctx context.Context, req AuditExportRequest) ([]byte, error) {
	var query contracts.AuditQuery
	if err := json.Unmarshal(req.QuerySnapshot, &query); err != nil {
		return nil, apperr.ErrAdminAuditExportCSVFailed.WithCause(err)
	}
	workerCtx := tenant.WithContext(ctx, tenant.Identity{TenantID: req.TenantID, AccountID: req.AccountID, IsPlatform: req.TenantID == 0})
	if req.TenantID > 0 {
		hasRole, err := s.roles.HasRole(workerCtx, req.AccountID, contracts.RoleSchoolAdmin)
		if err != nil {
			return nil, apperr.AsAppError(err)
		}
		if !hasRole {
			return nil, apperr.ErrForbidden
		}
	}
	result, err := s.collectAuditExportRows(workerCtx, query)
	if err != nil {
		return nil, err
	}
	return auditCSVBytes(result)
}

// reconcileAuditExportRequest clears terminal requests or waits until their task becomes claimable.
func (s *Service) reconcileAuditExportRequest(ctx context.Context, req AuditExportRequest) error {
	task, err := s.transfers.GetTask(ctx, req.TenantID, req.TransferTaskID)
	if err != nil {
		if errors.Is(err, apperr.ErrTransferTaskNotFound) {
			return s.deleteAuditExportRequest(ctx, req)
		}
		return errors.Join(err, s.deferAuditExportRequest(ctx, req, timex.Now().Add(time.Duration(s.cfg.AuditExportWorkerPollMs)*time.Millisecond)))
	}
	switch task.Status {
	case transfer.StatusSucceeded, transfer.StatusFailed:
		return s.deleteAuditExportRequest(ctx, req)
	case transfer.StatusRetrying:
		return s.deferAuditExportRequest(ctx, req, task.NextAttemptAfter)
	case transfer.StatusRunning:
		return s.deferAuditExportRequest(ctx, req, task.LeaseUntil)
	default:
		return s.deferAuditExportRequest(ctx, req, timex.Now().Add(time.Duration(s.cfg.AuditExportWorkerPollMs)*time.Millisecond))
	}
}

// failAuditExportRequest advances the leased task retry state and synchronizes its next M9 scan.
func (s *Service) failAuditExportRequest(ctx context.Context, req AuditExportRequest, task transfer.Task, cause error) error {
	failed, err := s.transfers.FailTask(ctx, task, cause)
	if err != nil {
		return errors.Join(cause, err, s.deferAuditExportRequest(ctx, req, timex.Now().Add(time.Duration(s.cfg.AuditExportWorkerPollMs)*time.Millisecond)))
	}
	if failed.Status == transfer.StatusFailed {
		return errors.Join(cause, s.deleteAuditExportRequest(ctx, req))
	}
	return errors.Join(cause, s.deferAuditExportRequest(ctx, req, failed.NextAttemptAfter))
}

// deferAuditExportRequest schedules the next attempt in the request table owned by M9.
func (s *Service) deferAuditExportRequest(ctx context.Context, req AuditExportRequest, next time.Time) error {
	if next.IsZero() || !next.After(timex.Now()) {
		next = timex.Now().Add(time.Duration(s.cfg.AuditExportWorkerPollMs) * time.Millisecond)
	}
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.SetAuditExportRequestNextCheck(ctx, req.TenantID, req.TransferTaskID, next)
		return err
	})
}

// deleteAuditExportRequest removes a terminal M9 request after its transfer task is durable.
func (s *Service) deleteAuditExportRequest(ctx context.Context, req AuditExportRequest) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		return tx.DeleteAuditExportRequest(ctx, req.TenantID, req.TransferTaskID)
	})
}

// collectAuditExportRows 按统一分页上限拉取当前权限范围内全部匹配审计记录。
func (s *Service) collectAuditExportRows(ctx context.Context, query contracts.AuditQuery) (contracts.AuditQueryResult, error) {
	var out contracts.AuditQueryResult
	for page := int32(1); ; page++ {
		query.Page = page
		query.Size = 0
		result, err := s.QueryAudit(ctx, query)
		if err != nil {
			return contracts.AuditQueryResult{}, err
		}
		if out.Page == 0 {
			out.Page = 1
			out.Size = result.Size
			out.Total = result.Total
		}
		out.List = append(out.List, result.List...)
		if len(result.List) == 0 || int64(len(out.List)) >= result.Total {
			break
		}
	}
	return out, nil
}

// auditCSVBytes 将审计查询结果编码为 CSV 文件内容。
func auditCSVBytes(result contracts.AuditQueryResult) ([]byte, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"id", "tenant_id", "actor_id", "action", "target_type", "target_id", "trace_id", "created_at"}); err != nil {
		return nil, fmt.Errorf("写入 CSV 表头失败: %w", err)
	}
	for _, row := range result.List {
		if err := w.Write([]string{ids.Format(row.ID), ids.Format(row.TenantID), ids.Format(row.ActorID), row.Action, row.TargetType, ids.Format(row.TargetID), row.TraceID, timex.RFC3339OrEmpty(row.CreatedAt)}); err != nil {
			return nil, fmt.Errorf("写入 CSV 行失败: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("刷新 CSV 内容失败: %w", err)
	}
	return []byte(b.String()), nil
}
