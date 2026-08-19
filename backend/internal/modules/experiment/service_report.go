// experiment service_report 文件实现实验报告提交、批改和分数重算。
package experiment

import (
	"context"
	"path"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/apperr"
)

// SubmitReport 提交实验报告对象引用,并校验对象 key 绑定当前租户、实例和学生。
func (s *Service) SubmitReport(ctx context.Context, instanceID int64, req SubmitReportRequest) (ReportDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ReportDTO{}, err
	}
	var report ExperimentReport
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		inst, err := tx.GetInstance(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, inst); err != nil {
			return err
		}
		if inst.Status == InstanceStatusRecycled || inst.Status == InstanceStatusError {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		if err := validateReportObjectRef(s.storage.BucketReport(), id.TenantID, instanceID, id.AccountID, req.ContentRef); err != nil {
			return err
		}
		report, err = tx.UpsertReport(ctx, ExperimentReport{ID: s.ids.Generate(), TenantID: id.TenantID, InstanceID: instanceID, StudentID: id.AccountID, ContentRef: req.ContentRef})
		return err
	}); err != nil {
		return ReportDTO{}, err
	}
	profiles, err := s.reportProfiles(ctx, []ExperimentReport{report})
	if err != nil {
		return ReportDTO{}, err
	}
	return reportDTOFromModel(report, profiles), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "experiment.report.submit", auditTargetReport, report.ID, map[string]any{"instance_id": instanceID})
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
