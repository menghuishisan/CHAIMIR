// experiment service_report 文件实现实验报告提交、批改和分数重算。
package experiment

import (
	"bytes"
	"context"
	"errors"
	"path"
	"path/filepath"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// SubmitReport 校验并上传实验报告,对象引用只由服务端生成后写入报告记录。
func (s *Service) SubmitReport(ctx context.Context, instanceID int64, req ReportUploadRequest) (ReportDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ReportDTO{}, err
	}
	contentType, err := normalizedReportContentType(req.FileName, req.ContentType, req.Content)
	if err != nil {
		return ReportDTO{}, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	if err := s.ensureReportUploadAllowed(ctx, id.TenantID, id.AccountID, instanceID); err != nil {
		return ReportDTO{}, err
	}

	uploadID := s.ids.Generate()
	storedFileName := "report-" + ids.Format(uploadID) + strings.ToLower(filepath.Ext(strings.TrimSpace(req.FileName)))
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:          id.TenantID,
		AccountID:         id.AccountID,
		Module:            experimentModuleName,
		ResourceType:      experimentReportResource,
		ResourceID:        ids.Format(instanceID),
		NestedResourceIDs: []string{ids.Format(id.AccountID)},
		FileName:          storedFileName,
		ContentType:       contentType,
		Size:              int64(len(req.Content)),
		MaxBytes:          s.reportMaxBytes,
		ExpectedBucket:    s.storage.BucketReport(),
		AllowedFileName:   true,
		Content:           req.Content,
		KindValidator:     reportFileKindValid,
		ScanPolicy:        s.reportScanPolicy,
	})
	if err != nil {
		return ReportDTO{}, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	if err := validateReportObjectRef(s.storage.BucketReport(), id.TenantID, instanceID, id.AccountID, plan.ObjectRef); err != nil {
		return ReportDTO{}, err
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(req.Content), plan.Size, plan.ContentType); err != nil {
		return ReportDTO{}, apperr.ErrExperimentReportInvalid.WithCause(err)
	}

	var report ExperimentReport
	var previousContentRef string
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		inst, err := tx.GetInstanceForUpdate(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, inst); err != nil {
			return err
		}
		if inst.Status == InstanceStatusRecycled || inst.Status == InstanceStatusError {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		previous, previousErr := tx.GetReportByInstanceStudent(ctx, id.TenantID, instanceID, id.AccountID)
		if previousErr == nil {
			previousContentRef = previous.ContentRef
		} else if !errors.Is(previousErr, apperr.ErrExperimentReportNotFound) {
			return previousErr
		}
		report, err = tx.UpsertReport(ctx, ExperimentReport{ID: s.ids.Generate(), TenantID: id.TenantID, InstanceID: instanceID, StudentID: id.AccountID, ContentRef: plan.ObjectRef})
		return err
	}); err != nil {
		s.deleteUploadedReportObject(ctx, plan.Bucket, plan.Key, "清理未关联实验报告失败")
		return ReportDTO{}, err
	}
	if previousContentRef != "" && previousContentRef != plan.ObjectRef {
		s.deletePreviousReportObject(ctx, id.TenantID, instanceID, id.AccountID, previousContentRef)
	}
	profiles, err := s.reportProfiles(ctx, []ExperimentReport{report})
	if err != nil {
		return ReportDTO{}, err
	}
	return reportDTOFromModel(report, profiles), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "experiment.report.submit", auditTargetReport, report.ID, map[string]any{"instance_id": instanceID})
}

// ensureReportUploadAllowed 在写对象前复用实例访问规则,阻止越权或终态上传。
func (s *Service) ensureReportUploadAllowed(ctx context.Context, tenantID, accountID, instanceID int64) error {
	return s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
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
	})
}

// normalizedReportContentType 校验声明 MIME 与文件扩展名一致,再以内容签名确认真实类型。
func normalizedReportContentType(fileName, declared string, content []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	var expected string
	switch ext {
	case ".pdf":
		expected = "application/pdf"
	case ".md", ".markdown":
		expected = "text/markdown"
	case ".txt":
		expected = "text/plain"
	default:
		return "", errors.New("实验报告文件类型不受支持")
	}
	declared = strings.ToLower(strings.TrimSpace(strings.SplitN(declared, ";", 2)[0]))
	if declared != "" && declared != "application/octet-stream" && declared != expected {
		return "", errors.New("实验报告声明类型与扩展名不一致")
	}
	if !reportFileKindValid(fileName, expected, content) {
		return "", errors.New("实验报告内容与文件类型不一致")
	}
	return expected, nil
}

// reportFileKindValid 把通用附件校验收窄为实验报告允许的三类文档。
func reportFileKindValid(fileName, contentType string, content []byte) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(fileName))) {
	case ".pdf", ".md", ".markdown", ".txt":
		return upload.AttachmentKindValid(fileName, contentType, content)
	default:
		return false
	}
}

// deleteUploadedReportObject 记录补偿删除失败,避免用清理错误覆盖原始业务错误链。
func (s *Service) deleteUploadedReportObject(ctx context.Context, bucket, key, message string) {
	if err := s.storage.Delete(ctx, bucket, key); err != nil {
		logging.ErrorContext(ctx, message, err.Error())
	}
}

// deletePreviousReportObject 只删除当前实例和学生受控前缀内的旧报告对象。
func (s *Service) deletePreviousReportObject(ctx context.Context, tenantID, instanceID, studentID int64, raw string) {
	if err := validateReportObjectRef(s.storage.BucketReport(), tenantID, instanceID, studentID, raw); err != nil {
		logging.ErrorContext(ctx, "旧实验报告引用不在受控路径,跳过清理", apperr.AsAppError(err).LogString())
		return
	}
	ref, err := storage.ParseObjectRef(raw)
	if err != nil {
		logging.ErrorContext(ctx, "解析旧实验报告引用失败", err.Error())
		return
	}
	s.deleteUploadedReportObject(ctx, ref.Bucket, ref.Key, "清理已替换实验报告失败")
}

// reportProfiles 把报告提交者一次批量解析为账号档案,避免逐条调用形成 N+1。
func (s *Service) reportProfiles(ctx context.Context, reports []ExperimentReport) (map[int64]contracts.AccountInfo, error) {
	seen := make(map[int64]struct{}, len(reports))
	accountIDs := make([]int64, 0, len(reports))
	for _, report := range reports {
		if _, ok := seen[report.StudentID]; ok {
			continue
		}
		seen[report.StudentID] = struct{}{}
		accountIDs = append(accountIDs, report.StudentID)
	}
	if len(accountIDs) == 0 {
		return map[int64]contracts.AccountInfo{}, nil
	}
	accounts, err := s.roles.BatchGetAccounts(ctx, accountIDs)
	if err != nil {
		return nil, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	profiles := make(map[int64]contracts.AccountInfo, len(accounts))
	for _, account := range accounts {
		profiles[account.AccountID] = account
	}
	return profiles, nil
}

// validateReportObjectRef 校验报告对象引用必须绑定租户、实例和学生路径。
func validateReportObjectRef(bucket string, tenantID, instanceID, studentID int64, raw string) error {
	ref, err := storage.ParseObjectRef(strings.TrimSpace(raw))
	if err != nil {
		return apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	if ref.Bucket != bucket {
		return apperr.ErrExperimentReportInvalid
	}
	prefix, err := storage.ObjectKey(tenantID, "experiment", "report", ids.Format(instanceID), ids.Format(studentID))
	if err != nil {
		return apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	clean := path.Clean(ref.Key)
	if clean != ref.Key || !strings.HasPrefix(ref.Key, prefix+"/") {
		return apperr.ErrExperimentReportInvalid
	}
	return nil
}

// ListReports 查询某实验下的报告列表;status 传 0 表示不按批改状态过滤。
func (s *Service) ListReports(ctx context.Context, experimentID int64, status int16, page, size int) ([]ReportDTO, int64, int, int, error) {
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
		if err := s.ensureTeacherCanManage(ctx, id.AccountID, exp); err != nil {
			return err
		}
		items, total, err = tx.ListReports(ctx, id.TenantID, experimentID, status, page, size)
		return err
	}); err != nil {
		return nil, 0, 0, 0, err
	}
	profiles, err := s.reportProfiles(ctx, items)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	out := make([]ReportDTO, 0, len(items))
	for _, item := range items {
		out = append(out, reportDTOFromModel(item, profiles))
	}
	return out, total, page, size, nil
}

// IssueReportAccess 校验教师管理权限并签发一次性报告文件下载授权。
func (s *Service) IssueReportAccess(ctx context.Context, reportID int64) (ReportAccessDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ReportAccessDTO{}, err
	}
	var report ExperimentReport
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
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
		return s.ensureTeacherCanManage(ctx, id.AccountID, exp)
	}); err != nil {
		return ReportAccessDTO{}, err
	}
	if err := validateReportObjectRef(s.storage.BucketReport(), id.TenantID, report.InstanceID, report.StudentID, report.ContentRef); err != nil {
		return ReportAccessDTO{}, err
	}
	token, grant, err := s.files.IssueDownloadGrant(storage.IssueDownloadGrantRequest{
		TenantID:     id.TenantID,
		AccountID:    id.AccountID,
		ObjectRef:    report.ContentRef,
		Module:       experimentModuleName,
		ResourceType: experimentReportResource,
		ResourceID:   ids.Format(report.InstanceID),
		Mode:         storage.DownloadModeDownload,
	})
	if err != nil {
		return ReportAccessDTO{}, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.report.download", auditTargetReport, report.ID, map[string]any{"instance_id": report.InstanceID}); err != nil {
		return ReportAccessDTO{}, err
	}
	return ReportAccessDTO{Token: token, ExpiresAt: timex.RFC3339OrEmpty(grant.ExpiresAt)}, nil
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
		inst, err = tx.GetInstanceForUpdate(ctx, id.TenantID, current.InstanceID)
		if err != nil {
			return err
		}
		exp, err := tx.GetExperiment(ctx, id.TenantID, inst.ExperimentID)
		if err != nil {
			return err
		}
		if err := s.ensureTeacherCanManage(ctx, id.AccountID, exp); err != nil {
			return err
		}
		report, err = tx.GradeReport(ctx, id.TenantID, reportID, req.ManualScore, req.Comment)
		if err != nil {
			return err
		}
		if inst.Status != InstanceStatusFinished && inst.Status != InstanceStatusRecycled {
			return nil
		}
		score, err := tx.SumScores(ctx, id.TenantID, inst.ID)
		if err != nil {
			return err
		}
		var changed bool
		inst, changed, err = tx.UpdateInstanceScoreIfChanged(ctx, id.TenantID, inst.ID, score)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		shouldPublish = true
		return s.enqueueExperimentScoreOutbox(ctx, tx, inst)
	}); err != nil {
		return ReportDTO{}, err
	}
	if shouldPublish {
		s.drainExperimentScoreOutboxBestEffort(ctx)
	}
	profiles, err := s.reportProfiles(ctx, []ExperimentReport{report})
	if err != nil {
		return ReportDTO{}, err
	}
	return reportDTOFromModel(report, profiles), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.report.grade", auditTargetReport, report.ID, map[string]any{"manual_score": req.ManualScore})
}
