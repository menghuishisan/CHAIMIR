// experiment service_definition 文件实现实验定义、向导草稿和发布前校验。
package experiment

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
)

// ListExperiments 查询当前租户实验定义列表。
func (s *Service) ListExperiments(ctx context.Context, courseID int64, status int16, page, size int) ([]ExperimentDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	items := []Experiment{}
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListExperiments(ctx, id.TenantID, courseID, status, page, size)
		return err
	}); err != nil {
		return nil, 0, 0, 0, err
	}
	out := make([]ExperimentDTO, 0, len(items))
	for _, item := range items {
		out = append(out, experimentDTOFromModel(item))
	}
	return out, total, page, size, nil
}

// CreateExperiment 创建服务端持久化的实验编排向导草稿。
func (s *Service) CreateExperiment(ctx context.Context, req ExperimentRequest) (ExperimentDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ExperimentDTO{}, err
	}
	req, err = validateExperimentRequest(req)
	if err != nil {
		return ExperimentDTO{}, err
	}
	item := Experiment{ID: s.ids.Generate(), TenantID: id.TenantID, CourseID: req.CourseID, AuthorID: id.AccountID, TemplateRef: req.TemplateRef, TemplateVersion: req.TemplateVersion, Name: req.Name, Description: req.Description, Components: req.Components, CollabMode: req.CollabMode, GroupConfig: req.GroupConfig, RequireReport: req.RequireReport, WizardStep: req.WizardStep}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.CreateExperiment(ctx, item)
		return err
	}); err != nil {
		return ExperimentDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.create", auditTargetExperiment, item.ID, map[string]any{"course_id": item.CourseID}); err != nil {
		return ExperimentDTO{}, err
	}
	return experimentDTOFromModel(item), nil
}

// UpdateExperiment 保存实验向导草稿当前步骤和组件编排。
func (s *Service) UpdateExperiment(ctx context.Context, experimentID int64, req ExperimentRequest) (ExperimentDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ExperimentDTO{}, err
	}
	req, err = validateExperimentRequest(req)
	if err != nil {
		return ExperimentDTO{}, err
	}
	var item Experiment
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetExperiment(ctx, id.TenantID, experimentID)
		if err != nil {
			return err
		}
		if err := ensureTeacherCanManage(id.AccountID, s.isSchoolAdmin(ctx, id.AccountID), current); err != nil {
			return err
		}
		current.CourseID = req.CourseID
		current.TemplateRef = req.TemplateRef
		current.TemplateVersion = req.TemplateVersion
		current.Name = req.Name
		current.Description = req.Description
		current.Components = req.Components
		current.CollabMode = req.CollabMode
		current.GroupConfig = req.GroupConfig
		current.RequireReport = req.RequireReport
		current.WizardStep = req.WizardStep
		item, err = tx.UpdateExperiment(ctx, current)
		return err
	}); err != nil {
		return ExperimentDTO{}, err
	}
	return experimentDTOFromModel(item), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.update", auditTargetExperiment, item.ID, map[string]any{"wizard_step": item.WizardStep})
}

// ValidateExperiment 执行发布前校验并返回所有阻断和提醒问题。
func (s *Service) ValidateExperiment(ctx context.Context, experimentID int64) (ValidationResultDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ValidationResultDTO{}, err
	}
	var item Experiment
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.GetExperiment(ctx, id.TenantID, experimentID)
		return err
	}); err != nil {
		return ValidationResultDTO{}, err
	}
	if err := ensureTeacherCanManage(id.AccountID, s.isSchoolAdmin(ctx, id.AccountID), item); err != nil {
		return ValidationResultDTO{}, err
	}
	return s.validateExperimentComponents(ctx, item), nil
}

// PublishExperiment 发布实验定义,并登记其 M5 模板和检查点引用。
func (s *Service) PublishExperiment(ctx context.Context, experimentID int64) (ExperimentDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ExperimentDTO{}, err
	}
	var item Experiment
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetExperiment(ctx, id.TenantID, experimentID)
		if err != nil {
			return err
		}
		if err := ensureTeacherCanManage(id.AccountID, s.isSchoolAdmin(ctx, id.AccountID), current); err != nil {
			return err
		}
		if current.Status != ExperimentStatusDraft && current.Status != ExperimentStatusUnpublished {
			return apperr.ErrExperimentStateInvalid
		}
		item = current
		return nil
	}); err != nil {
		return ExperimentDTO{}, err
	}
	result := s.validateExperimentComponents(ctx, item)
	if err := validatePublishResult(result); err != nil {
		return ExperimentDTO{}, err
	}
	if err := s.refreshContentUsageRefs(ctx, item); err != nil {
		return ExperimentDTO{}, err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.SetExperimentStatus(ctx, id.TenantID, experimentID, ExperimentStatusPublished)
		return err
	}); err != nil {
		return ExperimentDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.publish", auditTargetExperiment, item.ID, map[string]any{"course_id": item.CourseID}); err != nil {
		return ExperimentDTO{}, err
	}
	return experimentDTOFromModel(item), nil
}

// UnpublishExperiment 下架实验定义,不影响已创建实例和结果。
func (s *Service) UnpublishExperiment(ctx context.Context, experimentID int64) (ExperimentDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ExperimentDTO{}, err
	}
	var item Experiment
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetExperiment(ctx, id.TenantID, experimentID)
		if err != nil {
			return err
		}
		if err := ensureTeacherCanManage(id.AccountID, s.isSchoolAdmin(ctx, id.AccountID), current); err != nil {
			return err
		}
		if current.Status != ExperimentStatusPublished {
			return apperr.ErrExperimentStateInvalid
		}
		item, err = tx.SetExperimentStatus(ctx, id.TenantID, experimentID, ExperimentStatusUnpublished)
		return err
	}); err != nil {
		return ExperimentDTO{}, err
	}
	return experimentDTOFromModel(item), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.unpublish", auditTargetExperiment, item.ID, nil)
}

// validateExperimentComponents 校验模板、检查点内容和分值结构。
func (s *Service) validateExperimentComponents(ctx context.Context, item Experiment) ValidationResultDTO {
	result := ValidationResultDTO{OK: true, Issues: []ValidationIssueDTO{}}
	add := func(level, message string) {
		result.Issues = append(result.Issues, ValidationIssueDTO{Level: level, Message: message})
		if level == ValidationLevelError {
			result.OK = false
		}
	}
	if err := validateComponentConfig(item.Components, item.CollabMode, item.GroupConfig); err != nil {
		add(ValidationLevelError, "实验组件配置不完整")
	}
	if s.sandbox == nil && len(item.Components.Envs) > 0 {
		add(ValidationLevelError, "实验环境服务暂时不可用")
	}
	if s.sim == nil && len(item.Components.Sims) > 0 {
		add(ValidationLevelError, "仿真服务暂时不可用")
	}
	if s.judge == nil && len(item.Components.Checkpoints) > 0 {
		add(ValidationLevelError, "检查点判分服务暂时不可用")
	}
	if item.TemplateRef != "" {
		if _, err := s.content.GetContentFace(ctx, item.TenantID, contracts.ContentItemRef{ItemCode: item.TemplateRef, ItemVersion: item.TemplateVersion}); err != nil {
			add(ValidationLevelError, "实验模板版本暂时无法引用")
		}
	}
	var total float64
	for _, cp := range item.Components.Checkpoints {
		total += cp.Score
		if _, err := s.content.GetContentFace(ctx, item.TenantID, contracts.ContentItemRef{ItemCode: cp.ItemCode, ItemVersion: cp.ItemVersion}); err != nil {
			add(ValidationLevelError, fmt.Sprintf("检查点 %s 引用的题目版本暂时无法使用", cp.ID))
		}
	}
	if len(item.Components.Checkpoints) > 0 && (total < 99.99 || total > 100.01) {
		add(ValidationLevelWarning, fmt.Sprintf("检查点分值合计 %.2f,非 100", total))
	}
	return result
}

// refreshContentUsageRefs 在发布时登记 M5 内容引用,用于删除保护和复用统计。
func (s *Service) refreshContentUsageRefs(ctx context.Context, item Experiment) error {
	refs := make([]contracts.ContentItemRef, 0, 1+len(item.Components.Checkpoints))
	seen := map[string]bool{}
	if item.TemplateRef != "" {
		refs = append(refs, contracts.ContentItemRef{ItemCode: item.TemplateRef, ItemVersion: item.TemplateVersion})
		seen[item.TemplateRef+"\x00"+item.TemplateVersion] = true
	}
	for _, cp := range item.Components.Checkpoints {
		key := cp.ItemCode + "\x00" + cp.ItemVersion
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, contracts.ContentItemRef{ItemCode: cp.ItemCode, ItemVersion: cp.ItemVersion})
	}
	sourceRef := fmt.Sprintf("experiment:%d:definition:%d", item.CreatedAt.Year(), item.ID)
	if err := s.content.ReplaceUsageRefs(ctx, item.TenantID, "experiment.definition", sourceRef, refs); err != nil {
		return apperr.ErrExperimentContentUsageFailed.WithCause(err)
	}
	return nil
}
