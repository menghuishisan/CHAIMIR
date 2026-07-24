// experiment service_report 文件实现实验报告提交、批改和分数重算。
package experiment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

const (
	experimentReportModule       = "experiment"
	experimentReportResourceType = "report"
)

// SubmitReport 校验实例权限后上传并提交 Markdown 报告，不接受客户端对象引用。
func (s *Service) SubmitReport(ctx context.Context, instanceID int64, req ReportUploadRequest) (ReportDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ReportDTO{}, err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		return validateReportSubmission(ctx, tx, id.AccountID, id.TenantID, instanceID)
	}); err != nil {
		return ReportDTO{}, err
	}
	uploadID := s.ids.Generate()
	resourceID := fmt.Sprintf("%d-%d-%d", instanceID, id.AccountID, uploadID)
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:        id.TenantID,
		AccountID:       id.AccountID,
		Module:          experimentReportModule,
		ResourceType:    experimentReportResourceType,
		ResourceID:      resourceID,
		FileName:        req.FileName,
		ContentType:     req.ContentType,
		Size:            int64(len(req.Content)),
		MaxBytes:        s.reportMaxBytes,
		ExpectedBucket:  s.storage.BucketReport(),
		AllowedFileName: true,
		Content:         req.Content,
		KindValidator:   upload.MarkdownKindValid,
		ScanPolicy:      s.reportScanPolicy,
	})
	if err != nil {
		return ReportDTO{}, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(req.Content), plan.Size, plan.ContentType); err != nil {
		return ReportDTO{}, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	var report ExperimentReport
	var previous ExperimentReport
	var hasPrevious bool
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := validateReportSubmission(ctx, tx, id.AccountID, id.TenantID, instanceID); err != nil {
			return err
		}
		previous, hasPrevious, err = tx.FindReportByInstanceStudent(ctx, id.TenantID, instanceID, id.AccountID)
		if err != nil {
			return err
		}
		report, err = tx.UpsertReport(ctx, ExperimentReport{ID: uploadID, TenantID: id.TenantID, InstanceID: instanceID, StudentID: id.AccountID, ContentRef: plan.ObjectRef})
		return err
	}); err != nil {
		if cleanupErr := s.storage.Delete(ctx, plan.Bucket, plan.Key); cleanupErr != nil {
			return ReportDTO{}, apperr.ErrExperimentReportInvalid.WithCause(errors.Join(err, cleanupErr))
		}
		return ReportDTO{}, err
	}
	if hasPrevious && previous.ContentRef != plan.ObjectRef {
		if ref, parseErr := storage.ParseObjectRef(previous.ContentRef); parseErr != nil {
			logging.ErrorContext(ctx, "旧实验报告对象引用损坏", parseErr.Error(), slog.Int64("tenant_id", id.TenantID), slog.Int64("report_id", previous.ID))
		} else if cleanupErr := s.storage.Delete(ctx, ref.Bucket, ref.Key); cleanupErr != nil {
			logging.ErrorContext(ctx, "清理已替换实验报告失败", cleanupErr.Error(), slog.Int64("tenant_id", id.TenantID), slog.Int64("report_id", previous.ID))
		}
	}
	out, err := reportDTOFromModel(report)
	if err != nil {
		return ReportDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "experiment.report.submit", auditTargetReport, report.ID, map[string]any{"instance_id": instanceID, "file_name": out.FileName})
}

// validateReportSubmission 校验学生仍可访问且实例允许提交报告。
func validateReportSubmission(ctx context.Context, tx TxStore, accountID, tenantID, instanceID int64) error {
	inst, err := tx.GetInstance(ctx, tenantID, instanceID)
	if err != nil {
		return err
	}
	if err := ensureInstanceAccess(ctx, tx, accountID, inst); err != nil {
		return err
	}
	if inst.Status == InstanceStatusRecycled || inst.Status == InstanceStatusError {
		return apperr.ErrExperimentInstanceStateInvalid
	}
	return nil
}

// ListReports 查询某实验下的报告列表。
func (s *Service) ListReports(ctx context.Context, experimentID int64, page, size int) ([]ReportDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	var items []ExperimentReport
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		exp, err := tx.GetExperiment(ctx, id.TenantID, experimentID)
		if err != nil {
			return err
		}
		if err := ensureTeacherCanManage(id.AccountID, s.isSchoolAdmin(ctx, id.AccountID), exp); err != nil {
			return err
		}
		items, total, err = tx.ListReports(ctx, id.TenantID, experimentID, page, size)
		return err
	}); err != nil {
		return nil, 0, 0, 0, err
	}
	out := make([]ReportDTO, 0, len(items))
	if len(items) == 0 {
		return out, total, page, size, nil
	}
	accountIDs := make([]int64, 0, len(items))
	for _, item := range items {
		accountIDs = append(accountIDs, item.StudentID)
	}
	accounts, err := s.roles.BatchGetAccounts(ctx, accountIDs)
	if err != nil {
		return nil, 0, 0, 0, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	accountByID := make(map[int64]contracts.AccountInfo, len(accounts))
	for _, account := range accounts {
		accountByID[account.AccountID] = account
	}
	for _, item := range items {
		dto, err := reportDTOFromModel(item)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		account, ok := accountByID[item.StudentID]
		if !ok {
			return nil, 0, 0, 0, apperr.ErrExperimentReportInvalid
		}
		dto.StudentName = account.Name
		dto.StudentNo = account.No
		out = append(out, dto)
	}
	return out, total, page, size, nil
}

// IssueReportDownloadGrant 校验教师管理权限后签发一次性报告下载授权。
func (s *Service) IssueReportDownloadGrant(ctx context.Context, reportID int64) (ReportDownloadGrantDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ReportDownloadGrantDTO{}, err
	}
	var report ExperimentReport
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		report, err = tx.GetReport(ctx, id.TenantID, reportID)
		if err != nil {
			return err
		}
		inst, err := tx.GetInstance(ctx, id.TenantID, report.InstanceID)
		if err != nil {
			return err
		}
		exp, err := tx.GetExperiment(ctx, id.TenantID, inst.ExperimentID)
		if err != nil {
			return err
		}
		return ensureTeacherCanManage(id.AccountID, s.isSchoolAdmin(ctx, id.AccountID), exp)
	}); err != nil {
		return ReportDownloadGrantDTO{}, err
	}
	resourceID, err := reportResourceID(report.ContentRef)
	if err != nil {
		return ReportDownloadGrantDTO{}, err
	}
	token, grant, err := s.files.IssueDownloadGrant(storage.IssueDownloadGrantRequest{TenantID: id.TenantID, AccountID: id.AccountID, ObjectRef: report.ContentRef, Module: experimentReportModule, ResourceType: experimentReportResourceType, ResourceID: resourceID})
	if err != nil {
		return ReportDownloadGrantDTO{}, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	dto, err := reportDTOFromModel(report)
	if err != nil {
		return ReportDownloadGrantDTO{}, err
	}
	return ReportDownloadGrantDTO{Token: token, FileName: dto.FileName, ExpiresAt: grant.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")}, s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.report.download", auditTargetReport, report.ID, nil)
}

// reportResourceID 从统一对象 key 提取报告资源作用域，拒绝非 M7 报告路径。
func reportResourceID(objectRef string) (string, error) {
	ref, err := storage.ParseObjectRef(strings.TrimSpace(objectRef))
	if err != nil {
		return "", apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	parts := strings.Split(ref.Key, "/")
	if len(parts) != 5 || parts[1] != experimentReportModule || parts[2] != experimentReportResourceType {
		return "", apperr.ErrExperimentReportInvalid
	}
	ids := strings.Split(parts[3], "-")
	if len(ids) != 3 {
		return "", apperr.ErrExperimentReportInvalid
	}
	for _, value := range ids {
		parsed, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil {
			return "", apperr.ErrExperimentReportInvalid.WithCause(parseErr)
		}
		if parsed <= 0 {
			return "", apperr.ErrExperimentReportInvalid
		}
	}
	return parts[3], nil
}

// GradeReport 批改实验报告并重算对应实例得分。
func (s *Service) GradeReport(ctx context.Context, reportID int64, req GradeReportRequest) (ReportDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ReportDTO{}, err
	}
	req.Comment = strings.TrimSpace(req.Comment)
	if err := validateManualScore(req.ManualScore); err != nil {
		return ReportDTO{}, err
	}
	var report ExperimentReport
	var inst ExperimentInstance
	shouldPublish := false
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetReport(ctx, id.TenantID, reportID)
		if err != nil {
			return err
		}
		inst, err = tx.GetInstance(ctx, id.TenantID, current.InstanceID)
		if err != nil {
			return err
		}
		exp, err := tx.GetExperiment(ctx, id.TenantID, inst.ExperimentID)
		if err != nil {
			return err
		}
		if err := ensureTeacherCanManage(id.AccountID, s.isSchoolAdmin(ctx, id.AccountID), exp); err != nil {
			return err
		}
		report, err = tx.GradeReport(ctx, id.TenantID, reportID, req.ManualScore, req.Comment)
		if err != nil {
			return err
		}
		if inst.Status != InstanceStatusFinished {
			return nil
		}
		score, err := tx.SumScores(ctx, id.TenantID, inst.ID)
		if err != nil {
			return err
		}
		inst, err = tx.UpdateInstanceScore(ctx, id.TenantID, inst.ID, score)
		if err != nil {
			return err
		}
		shouldPublish = true
		return s.enqueueExperimentScoreOutbox(ctx, tx, inst)
	}); err != nil {
		return ReportDTO{}, err
	}
	if shouldPublish {
		s.drainExperimentScoreOutboxBestEffort(ctx)
	}
	out, err := reportDTOFromModel(report)
	if err != nil {
		return ReportDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.report.grade", auditTargetReport, report.ID, map[string]any{"manual_score": req.ManualScore})
}
