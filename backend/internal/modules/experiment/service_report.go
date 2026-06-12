// experiment service_report 文件实现实验报告提交、批改和分数重算。
package experiment

import (
	"context"
	"strings"

	"chaimir/internal/platform/audit"
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
	return reportDTOFromModel(report), s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleStudent, "experiment.report.submit", auditTargetReport, report.ID, map[string]any{"instance_id": instanceID})
}

// ListReports 查询某实验下的报告列表。
func (s *Service) ListReports(ctx context.Context, experimentID int64, page, size int) ([]ReportDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
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
	for _, item := range items {
		out = append(out, reportDTOFromModel(item))
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
		shouldPublish = true
		return err
	}); err != nil {
		return ReportDTO{}, err
	}
	if shouldPublish {
		if err := s.publishScored(ctx, inst); err != nil {
			return ReportDTO{}, err
		}
	}
	return reportDTOFromModel(report), s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "experiment.report.grade", auditTargetReport, report.ID, map[string]any{"manual_score": req.ManualScore})
}
