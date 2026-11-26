package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/apperr"
)

// PrepareExperimentClone 仅供 M6 在来源课程授权和锁定期间准备独立实验。
func (s *Service) PrepareExperimentClone(ctx context.Context, req contracts.ExperimentCloneRequest) (plan contracts.ExperimentClonePlan, err error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return plan, err
	}
	if id.IsSystem || id.IsPlatform || req.SourceTenantID <= 0 || req.SourceCourseID <= 0 || req.SourceTeacherID <= 0 || req.TargetCourseID <= 0 {
		return plan, apperr.ErrExperimentInvalid
	}
	plan.IDs = make(map[int64]int64)
	prefix, err := storage.ObjectKey(id.TenantID, experimentModuleName, "clone", ids.Format(s.ids.Generate()))
	if err != nil {
		return plan, err
	}
	copies, err := storage.NewCopyBatch(s.storage, prefix, s.cloneMaxBytes)
	if err != nil {
		return plan, err
	}
	var items []Experiment
	var content contracts.ContentClonePlan
	var environments []contracts.SandboxCompositionClonePlan
	plan.Discard = func(ctx context.Context) error {
		errs := []error{copies.Discard(ctx)}
		if content.Discard != nil {
			errs = append(errs, content.Discard(ctx))
		}
		for _, env := range environments {
			if env.Discard != nil {
				errs = append(errs, env.Discard(ctx))
			}
		}
		return errors.Join(errs...)
	}
	defer func() {
		if err != nil {
			err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, false, plan.Discard)
		}
	}()
	err = s.store.TenantTx(ctx, req.SourceTenantID, func(ctx context.Context, tx TxStore) error {
		sources, err := tx.ListCourseExperimentsForClone(ctx, req.SourceTenantID, req.SourceCourseID, req.SourceTeacherID, req.LessonExperimentIDs)
		if err != nil {
			return err
		}
		var refs []contracts.ContentItemRef
		for _, source := range sources {
			if source.TenantID != req.SourceTenantID || (source.CourseID != req.SourceCourseID && (source.CourseID != 0 || source.AuthorID != req.SourceTeacherID)) {
				return apperr.ErrExperimentNotFound
			}
			plan.IDs[source.ID] = s.ids.Generate()
			if source.TemplateRef != "" {
				refs = append(refs, contracts.ContentItemRef{ItemCode: source.TemplateRef, ItemVersion: source.TemplateVersion})
			}
			for _, cp := range source.Components.Checkpoints {
				refs = append(refs, contracts.ContentItemRef{ItemCode: cp.ItemCode, ItemVersion: cp.ItemVersion})
			}
		}
		for _, sourceID := range req.LessonExperimentIDs {
			if sourceID <= 0 || plan.IDs[sourceID] == 0 {
				return apperr.ErrExperimentNotFound
			}
		}
		if len(refs) > 0 {
			content, err = s.contentClone.PrepareContentClone(ctx, req.SourceTenantID, refs)
			if err != nil {
				content = contracts.ContentClonePlan{}
				return err
			}
		}
		validatedSims := make(map[string]bool)
		for _, source := range sources {
			item := source
			item.ID, item.TenantID, item.CourseID, item.AuthorID, item.Status = plan.IDs[source.ID], id.TenantID, req.TargetCourseID, id.AccountID, ExperimentStatusDraft
			// 独立复制配置，后续引用重写不能修改来源或共享 map/slice。
			raw, err := json.Marshal(source.Components)
			if err != nil {
				return err
			}
			item.Components = ComponentConfig{}
			if err := json.Unmarshal(raw, &item.Components); err != nil {
				return err
			}
			item.GroupConfig.Roles = append([]string(nil), source.GroupConfig.Roles...)
			// 冻结环境由 M2 单独验证和复制，其余自由参数也须成为本校独立素材。
			if err := s.cloneComponentObjects(ctx, &item.Components, req.SourceTenantID, copies); err != nil {
				return err
			}
			if source.TemplateRef != "" {
				ref, ok := content.Refs[contracts.ContentItemRef{ItemCode: source.TemplateRef, ItemVersion: source.TemplateVersion}]
				if !ok {
					return apperr.ErrExperimentInvalid
				}
				item.TemplateRef, item.TemplateVersion = ref.ItemCode, ref.ItemVersion
			}
			for i := range item.Components.Checkpoints {
				cp := &item.Components.Checkpoints[i]
				ref, ok := content.Refs[contracts.ContentItemRef{ItemCode: cp.ItemCode, ItemVersion: cp.ItemVersion}]
				if !ok {
					return apperr.ErrExperimentInvalid
				}
				cp.ItemCode, cp.ItemVersion = ref.ItemCode, ref.ItemVersion
			}
			for i := range item.Components.Envs {
				env := &item.Components.Envs[i]
				prepared, err := s.sandboxClone.PrepareCompositionClone(ctx, req.SourceTenantID, env.CompositionSnapshot, env.InitCodeRef, env.InitScriptRef)
				environments = append(environments, prepared)
				if err != nil {
					return err
				}
				env.CompositionSnapshot, env.InitCodeRef, env.InitScriptRef = prepared.Snapshot, prepared.InitCodeRef, prepared.InitScriptRef
			}
			for _, sim := range item.Components.Sims {
				key := sim.PackageCode + "\x00" + sim.Version
				if validatedSims[key] {
					continue
				}
				if err := s.sim.ValidatePackage(ctx, id.TenantID, sim.PackageCode, sim.Version); err != nil {
					return err
				}
				validatedSims[key] = true
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		return plan, err
	}
	plan.Save = func(ctx context.Context) error {
		return s.store.CloneTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
			if content.Save != nil {
				if err := content.Save(ctx); err != nil {
					return err
				}
			}
			for _, env := range environments {
				if err := env.Save(ctx); err != nil {
					return err
				}
			}
			for _, item := range items {
				created, err := tx.CreateExperiment(ctx, item)
				if err != nil {
					return err
				}
				if err := s.enqueueExperimentSourceEvent(ctx, tx, created); err != nil {
					return err
				}
				if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.course.clone", auditTargetExperiment, created.ID, map[string]any{"source_course_id": req.SourceCourseID, "source_tenant_id": req.SourceTenantID}); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return plan, nil
}

var _ contracts.ExperimentCloneService = (*Service)(nil)

func (s *Service) cloneComponentObjects(ctx context.Context, cfg *ComponentConfig, sourceTenantID int64, copies *storage.CopyBatch) error {
	rewrite := func(ref storage.ObjectRef) (string, error) {
		if ref.Bucket != s.storage.BucketCode() && ref.Bucket != s.storage.BucketAttach() {
			return "", apperr.ErrExperimentInvalid
		}
		if strings.HasPrefix(ref.Key, "0/") {
			return storage.ObjectRefString(ref.Bucket, ref.Key)
		}
		if !strings.HasPrefix(ref.Key, ids.Format(sourceTenantID)+"/") {
			return "", apperr.ErrExperimentInvalid
		}
		return copies.Copy(ctx, ref)
	}
	for i := range cfg.Checkpoints {
		if _, err := storage.RewriteObjectRefs(cfg.Checkpoints[i].ExtraInput, rewrite); err != nil {
			return err
		}
	}
	for i := range cfg.Sims {
		if _, err := storage.RewriteObjectRefs(cfg.Sims[i].Params, rewrite); err != nil {
			return err
		}
	}
	for i := range cfg.Stages {
		for j := range cfg.Stages[i].ParamBindings {
			binding := &cfg.Stages[i].ParamBindings[j]
			value, err := storage.RewriteObjectRefs(binding.ConstantValue, rewrite)
			if err != nil {
				return err
			}
			binding.ConstantValue = value
		}
	}
	return nil
}
