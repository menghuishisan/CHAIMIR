// M7 服务入口:定义实验编排服务依赖、构造函数与核心业务操作。
package experiment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

// Service 是 M7 实验模块服务。
type Service struct {
	store    experimentStore
	idgen    snowflake.Generator
	auditor  audit.Writer
	identity contracts.IdentityService
	content  contracts.ContentReadService
	sandbox  contracts.SandboxService
	judge    contracts.JudgeService
	sim      contracts.SimService
	bus      eventbus.Bus
}

// NewService 构造 M7 服务。
func NewService(database *db.DB, idgen *snowflake.Node, auditor audit.Writer, identity contracts.IdentityService, content contracts.ContentReadService, sandbox contracts.SandboxService, judge contracts.JudgeService, sim contracts.SimService, bus eventbus.Bus) *Service {
	return &Service{store: newRepo(database), idgen: idgen, auditor: auditor, identity: identity, content: content, sandbox: sandbox, judge: judge, sim: sim, bus: bus}
}

// ListExperiments 查询当前租户实验列表。
func (s *Service) ListExperiments(ctx context.Context, courseID int64, status int16, page, size int) ([]ExperimentDTO, int64, error) {
	return s.store.ListExperiments(ctx, courseID, status, page, size)
}

// CreateExperiment 创建实验向导草稿。
func (s *Service) CreateExperiment(ctx context.Context, req ExperimentRequest) (ExperimentDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ExperimentDTO{}, apperr.ErrUnauthorized
	}
	if err := validateExperimentDraftRequest(req); err != nil {
		return ExperimentDTO{}, err
	}
	out, err := s.store.CreateExperiment(ctx, id, s.nextID(), req)
	if err != nil {
		return ExperimentDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, auditActionExperimentCreate, auditTargetExperiment, ids.ParseOrZero(out.ID), map[string]any{"name": out.Name})
}

// UpdateExperiment 更新实验草稿和向导步骤。
func (s *Service) UpdateExperiment(ctx context.Context, experimentID int64, req ExperimentRequest) (ExperimentDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ExperimentDTO{}, apperr.ErrUnauthorized
	}
	if err := validateExperimentDraftRequest(req); err != nil {
		return ExperimentDTO{}, err
	}
	current, err := s.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return ExperimentDTO{}, err
	}
	if err := s.ensureExperimentManager(ctx, current); err != nil {
		return ExperimentDTO{}, err
	}
	if current.Status == ExperimentStatusPublished {
		return ExperimentDTO{}, apperr.ErrExperimentState
	}
	out, err := s.store.UpdateExperiment(ctx, experimentID, req)
	if err != nil {
		return ExperimentDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, auditActionExperimentUpdate, auditTargetExperiment, experimentID, map[string]any{"wizard_step": out.WizardStep})
}

// ValidateExperiment 执行发布前结构校验和可访问内容版本校验。
func (s *Service) ValidateExperiment(ctx context.Context, experimentID int64) (ValidationResult, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ValidationResult{}, apperr.ErrUnauthorized
	}
	experiment, err := s.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return ValidationResult{}, err
	}
	if err := s.ensureExperimentManager(ctx, experiment); err != nil {
		return ValidationResult{}, err
	}
	issues := validateExperimentComponentsDetailed(experiment.Components)
	for _, cp := range experiment.Components.Checkpoints {
		if s.content == nil {
			issues = append(issues, ValidationIssue{Level: "error", Message: "检查点题目版本校验服务不可用"})
			continue
		}
		_, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: cp.ItemCode, ItemVersion: cp.ItemVersion})
		if err != nil {
			issues = append(issues, ValidationIssue{Level: "error", Message: "检查点引用的题目版本不可用"})
		}
	}
	return ValidationResult{OK: !hasErrorIssue(issues), Issues: issues}, nil
}

// PublishExperiment 发布通过校验的实验。
func (s *Service) PublishExperiment(ctx context.Context, experimentID int64) (ExperimentDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ExperimentDTO{}, apperr.ErrUnauthorized
	}
	experiment, err := s.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return ExperimentDTO{}, err
	}
	if err := s.ensureExperimentManager(ctx, experiment); err != nil {
		return ExperimentDTO{}, err
	}
	result, err := s.ValidateExperiment(ctx, experimentID)
	if err != nil {
		return ExperimentDTO{}, err
	}
	if !result.OK {
		return ExperimentDTO{}, apperr.ErrExperimentInvalid
	}
	out, err := s.store.UpdateExperimentStatus(ctx, experimentID, ExperimentStatusPublished)
	if err != nil {
		return ExperimentDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, auditActionExperimentPublish, auditTargetExperiment, experimentID, map[string]any{"status": out.Status})
}

// UnpublishExperiment 下架实验定义,不影响历史实例。
func (s *Service) UnpublishExperiment(ctx context.Context, experimentID int64) (ExperimentDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ExperimentDTO{}, apperr.ErrUnauthorized
	}
	experiment, err := s.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return ExperimentDTO{}, err
	}
	if err := s.ensureExperimentManager(ctx, experiment); err != nil {
		return ExperimentDTO{}, err
	}
	out, err := s.store.UpdateExperimentStatus(ctx, experimentID, ExperimentStatusUnpublished)
	if err != nil {
		return ExperimentDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, auditActionExperimentUnpublish, auditTargetExperiment, experimentID, map[string]any{"status": out.Status})
}

// StartInstance 创建实验实例并编排 M2 沙箱与 M4 仿真资源。
func (s *Service) StartInstance(ctx context.Context, experimentID int64, req StartInstanceRequest) (ExperimentInstanceDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ExperimentInstanceDTO{}, apperr.ErrUnauthorized
	}
	experiment, err := s.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	if experiment.Status != ExperimentStatusPublished {
		return ExperimentInstanceDTO{}, apperr.ErrExperimentState
	}
	groupID, err := s.authorizeStartInstance(ctx, experiment, req.GroupID)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	instanceID := s.nextID()
	sourceRef := sourceRefForInstance(instanceID)
	if _, err := s.store.CreateInstance(ctx, id, instanceID, experimentID, groupID, sourceRef); err != nil {
		return ExperimentInstanceDTO{}, err
	}
	sandboxes, sims, err := s.createEngineResources(ctx, id, sourceRef, experiment.Components)
	if err != nil {
		return ExperimentInstanceDTO{}, s.failInstanceWithCompensation(ctx, instanceID, id.TenantID, sourceRef, err)
	}
	return s.store.UpdateInstanceResources(ctx, instanceID, sandboxes, sims, InstanceStatusRunning)
}

// GetInstance 读取实例工作台摘要。
func (s *Service) GetInstance(ctx context.Context, instanceID int64) (ExperimentInstanceDTO, error) {
	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	if err := s.ensureInstanceAccess(ctx, instance); err != nil {
		return ExperimentInstanceDTO{}, err
	}
	return instance, nil
}

// PauseInstance 暂停运行中的实例。
func (s *Service) PauseInstance(ctx context.Context, instanceID int64) (ExperimentInstanceDTO, error) {
	instance, err := s.GetInstance(ctx, instanceID)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	if err := validateInstanceTransition(instance.Status, InstanceStatusPaused); err != nil {
		return ExperimentInstanceDTO{}, err
	}
	return s.store.UpdateInstanceStatus(ctx, instanceID, InstanceStatusPaused)
}

// ResumeInstance 恢复已暂停或环境已释放的实例;环境释放态会重新编排资源。
func (s *Service) ResumeInstance(ctx context.Context, instanceID int64) (ExperimentInstanceDTO, error) {
	instance, err := s.GetInstance(ctx, instanceID)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	if err := validateInstanceTransition(instance.Status, InstanceStatusRunning); err != nil {
		return ExperimentInstanceDTO{}, err
	}
	if instance.Status == InstanceStatusReleased {
		return s.rebuildReleasedInstance(ctx, instance)
	}
	return s.store.UpdateInstanceStatus(ctx, instanceID, InstanceStatusRunning)
}

// FinishInstance 汇总检查点和报告分,写入总分并发布 experiment.scored 事件。
func (s *Service) FinishInstance(ctx context.Context, instanceID int64) (ExperimentInstanceDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ExperimentInstanceDTO{}, apperr.ErrUnauthorized
	}
	instance, err := s.GetInstance(ctx, instanceID)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	if instance.Status != InstanceStatusRunning && instance.Status != InstanceStatusPaused && instance.Status != InstanceStatusReleased {
		return ExperimentInstanceDTO{}, apperr.ErrExperimentInstanceState
	}
	parts, err := s.store.ListCheckpointScores(ctx, instanceID)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	reportScore, err := s.store.LatestReportScore(ctx, instanceID)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	score, err := computeExperimentScore(parts, reportScore)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	out, err := s.store.UpdateInstanceScore(ctx, instanceID, score)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	if err := s.publishScored(ctx, id.TenantID, out); err != nil {
		return ExperimentInstanceDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, auditActionInstanceFinish, auditTargetInstance, instanceID, map[string]any{"score": score})
}

// RecycleInstance 释放实例底层沙箱和仿真资源,并保留结果。
func (s *Service) RecycleInstance(ctx context.Context, instanceID int64) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	instance, err := s.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	if instance.Status == InstanceStatusRecycled {
		return nil
	}
	sourceRef := instance.SourceRef
	if err := s.recycleEnginesForInstance(ctx, id.TenantID, instance, "manual"); err != nil {
		return err
	}
	if _, err := s.store.UpdateInstanceStatus(ctx, instanceID, InstanceStatusRecycled); err != nil {
		return err
	}
	return s.writeAudit(ctx, id.TenantID, auditActionInstanceRecycle, auditTargetInstance, instanceID, map[string]any{"source_ref": sourceRef})
}

// JudgeCheckpoint 提交一次检查点判题任务,并预登记等待事件回写的检查点结果。
func (s *Service) JudgeCheckpoint(ctx context.Context, instanceID int64, checkpointID string) (CheckpointResultDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return CheckpointResultDTO{}, apperr.ErrUnauthorized
	}
	instance, err := s.GetInstance(ctx, instanceID)
	if err != nil {
		return CheckpointResultDTO{}, err
	}
	experimentID, _ := ids.Parse(instance.ExperimentID)
	experiment, err := s.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return CheckpointResultDTO{}, err
	}
	cp, ok := findCheckpoint(experiment.Components, checkpointID)
	if !ok {
		return CheckpointResultDTO{}, apperr.ErrCheckpointResultNotFound
	}
	if s.judge == nil {
		return CheckpointResultDTO{}, apperr.ErrCheckpointJudgeUnavailable
	}
	task, err := s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{
		TenantID: id.TenantID, JudgerCode: cp.JudgerCode, ItemCode: cp.ItemCode, ItemVersion: cp.ItemVersion,
		SubmitterID: id.AccountID, SourceRef: instance.SourceRef, SandboxMode: "reuse",
		TargetSandboxRef: firstSandboxRef(instance.Sandboxes), ExtraInput: cp.ExtraInput, Priority: 5,
	})
	if err != nil {
		return CheckpointResultDTO{}, apperr.ErrCheckpointJudgeFailed.WithCause(err)
	}
	return s.store.UpsertCheckpointResult(ctx, CheckpointResultDTO{
		ID: ids.Format(s.nextID()), TenantID: id.TenantID, InstanceID: instanceID, CheckpointID: checkpointID,
		JudgeTaskRef: ids.Format(task.TaskID), Passed: false, Score: 0,
	})
}

// HandleJudgeCompleted 处理 M3 判题完成事件并回写检查点结果。
func (s *Service) HandleJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	pending, err := s.store.PendingCheckpointByJudgeTask(ctx, event.TenantID, event.TaskID)
	if err != nil {
		return err
	}
	if pending.SourceRef != event.SourceRef {
		return apperr.ErrExperimentEventUnmatched.WithCause(fmt.Errorf("judge completed source_ref mismatch: pending=%s event=%s", pending.SourceRef, event.SourceRef))
	}
	_, err = s.store.UpsertCheckpointResult(ctx, CheckpointResultDTO{
		ID: ids.Format(s.nextID()), TenantID: event.TenantID, InstanceID: pending.InstanceID, CheckpointID: pending.CheckpointID,
		JudgeTaskRef: ids.Format(event.TaskID), Passed: event.Score > 0, Score: float64(event.Score),
	})
	return err
}

// HandleJudgeFailed 处理 M3 判题失败事件并保留 0 分检查点结果。
func (s *Service) HandleJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	pending, err := s.store.PendingCheckpointByJudgeTask(ctx, event.TenantID, event.TaskID)
	if err != nil {
		return err
	}
	if pending.SourceRef != event.SourceRef {
		return apperr.ErrExperimentEventUnmatched.WithCause(fmt.Errorf("judge failed source_ref mismatch: pending=%s event=%s", pending.SourceRef, event.SourceRef))
	}
	_, err = s.store.UpsertCheckpointResult(ctx, CheckpointResultDTO{
		ID: ids.Format(s.nextID()), TenantID: event.TenantID, InstanceID: pending.InstanceID, CheckpointID: pending.CheckpointID,
		JudgeTaskRef: ids.Format(event.TaskID), Passed: false, Score: 0, DetailRef: "judge_failed",
	})
	return err
}

// HandleSandboxRecycled 处理 M2 沙箱回收事件并标记相关实例为环境已释放。
func (s *Service) HandleSandboxRecycled(ctx context.Context, event contracts.SandboxRecycledEvent) error {
	_, err := s.store.MarkInstancesReleasedBySandbox(ctx, event.TenantID, event.SandboxID)
	return err
}

// SubmitReport 保存学生实验报告对象引用。
func (s *Service) SubmitReport(ctx context.Context, instanceID int64, req ReportRequest) (ReportDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ReportDTO{}, apperr.ErrUnauthorized
	}
	if req.ContentRef == "" {
		return ReportDTO{}, apperr.ErrExperimentReportInvalid
	}
	instance, err := s.GetInstance(ctx, instanceID)
	if err != nil {
		return ReportDTO{}, err
	}
	if err := validateReportContentRef(id, instance, req.ContentRef); err != nil {
		return ReportDTO{}, err
	}
	if instance.Status == InstanceStatusCompleted || instance.Status == InstanceStatusRecycled {
		return ReportDTO{}, apperr.ErrExperimentInstanceState
	}
	return s.store.CreateReport(ctx, id, s.nextID(), instanceID, req.ContentRef)
}

// ListReports 查询某实验下的报告列表。
func (s *Service) ListReports(ctx context.Context, experimentID int64, page, size int) ([]ReportDTO, error) {
	experiment, err := s.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureExperimentManager(ctx, experiment); err != nil {
		return nil, err
	}
	return s.store.ListReports(ctx, experimentID, page, size)
}

// GradeReport 批改实验报告分。
func (s *Service) GradeReport(ctx context.Context, reportID int64, req ReportGradeRequest) (ReportDTO, error) {
	if err := validateScore(req.Score); err != nil {
		return ReportDTO{}, err
	}
	id, isSchoolAdmin, err := s.ensureGroupManager(ctx)
	if err != nil {
		return ReportDTO{}, err
	}
	return s.store.GradeReportAuthorized(ctx, id, isSchoolAdmin, reportID, req.Score, req.Comment)
}

// CreateGroup 创建协作小组。
func (s *Service) CreateGroup(ctx context.Context, experimentID int64, req GroupRequest) (GroupDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return GroupDTO{}, apperr.ErrUnauthorized
	}
	experiment, err := s.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return GroupDTO{}, err
	}
	if err := s.ensureExperimentManager(ctx, experiment); err != nil {
		return GroupDTO{}, err
	}
	if req.Name == "" {
		return GroupDTO{}, apperr.ErrExperimentGroupInvalid
	}
	return s.store.CreateGroup(ctx, id, s.nextID(), experimentID, req.Name)
}

// AddGroupMember 新增或更新小组成员角色。
func (s *Service) AddGroupMember(ctx context.Context, groupID int64, req GroupMemberRequest) (GroupMemberDTO, error) {
	studentID, ok := ids.Parse(req.StudentID)
	if !ok || req.Role == "" {
		return GroupMemberDTO{}, apperr.ErrExperimentGroupInvalid
	}
	id, isSchoolAdmin, err := s.ensureGroupManager(ctx)
	if err != nil {
		return GroupMemberDTO{}, err
	}
	return s.store.AddGroupMemberAuthorized(ctx, id, isSchoolAdmin, s.nextID(), groupID, studentID, req.Role)
}

// GetGroup 读取协作小组信息。
func (s *Service) GetGroup(ctx context.Context, groupID int64) (GroupDTO, error) {
	return s.store.GetGroup(ctx, groupID)
}

// Stats 实现 M9 看板实验统计契约。
func (s *Service) Stats(ctx context.Context, tenantID, courseID int64) (contracts.ExperimentStats, error) {
	stats, err := s.store.Stats(ctx, tenantID, courseID)
	if err != nil {
		return contracts.ExperimentStats{}, err
	}
	return contracts.ExperimentStats{TenantID: tenantID, CourseID: courseID, ExperimentCount: stats.ExperimentCount, ActiveInstanceCount: stats.ActiveInstanceCount}, nil
}

// StatsDTO 返回 HTTP 内部统计 DTO。
func (s *Service) StatsDTO(ctx context.Context, tenantID, courseID int64) (StatsDTO, error) {
	return s.store.Stats(ctx, tenantID, courseID)
}

// nextID 从雪花节点生成 ID。
func (s *Service) nextID() int64 {
	return s.idgen.Generate()
}

// authorizeStartInstance 校验单人/小组实例启动边界。
func (s *Service) authorizeStartInstance(ctx context.Context, experiment ExperimentDTO, groupIDRaw string) (int64, error) {
	if experiment.CollabMode == CollabModeSingle {
		if groupIDRaw != "" {
			return 0, apperr.ErrExperimentGroupInvalid
		}
		return 0, nil
	}
	groupID, ok := ids.Parse(groupIDRaw)
	if !ok {
		return 0, apperr.ErrExperimentGroupInvalid
	}
	experimentID, _ := ids.Parse(experiment.ID)
	group, err := s.store.GetGroupForExperiment(ctx, groupID, experimentID)
	if err != nil {
		return 0, err
	}
	id, _ := tenantFromContext(ctx)
	for _, member := range group.Members {
		memberID, _ := ids.Parse(member.StudentID)
		if memberID == id.AccountID {
			return groupID, nil
		}
	}
	return 0, apperr.ErrExperimentForbidden
}

// createEngineResources 调用 M2/M4 创建实例所需引擎资源。
func (s *Service) createEngineResources(ctx context.Context, id tenant.Identity, sourceRef string, components ExperimentComponents) ([]SandboxRef, []SimSessionRef, error) {
	sandboxes := make([]SandboxRef, 0, len(components.Envs))
	for _, env := range components.Envs {
		if s.sandbox == nil {
			return sandboxes, nil, apperr.ErrExperimentEngineUnavailable
		}
		info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{
			TenantID: id.TenantID, RuntimeCode: env.RuntimeCode, ToolCodes: env.ToolCodes, InitCodeRef: env.InitCodeRef,
			InitScriptRef: env.InitScriptRef, OwnerAccountID: id.AccountID, SourceRef: sourceRef, KeepAlive: env.KeepAlive,
			KeepAliveMinutes: env.KeepAliveMinutes, SnapshotEnabled: env.SnapshotEnabled, SnapshotRetentionMinutes: env.SnapshotRetentionMinutes,
		})
		if err != nil {
			return sandboxes, nil, apperr.ErrExperimentEngineFailed.WithCause(err)
		}
		sandboxes = append(sandboxes, sandboxRefFromInfo(info, env.RuntimeCode))
	}
	sims := make([]SimSessionRef, 0, len(components.Sims))
	for _, sim := range components.Sims {
		if s.sim == nil {
			return sandboxes, sims, apperr.ErrExperimentEngineUnavailable
		}
		info, err := s.sim.CreateSimSession(ctx, contracts.SimCreateSessionRequest{
			TenantID: id.TenantID, PackageCode: sim.PackageCode, Version: sim.Version, Seed: sim.Seed,
			InitParams: sim.Params, OwnerAccountID: id.AccountID, SourceRef: sourceRef,
		})
		if err != nil {
			return sandboxes, sims, apperr.ErrExperimentEngineFailed.WithCause(err)
		}
		sims = append(sims, simRefFromInfo(info))
	}
	return sandboxes, sims, nil
}

// failInstanceWithCompensation 标记实例错误并补偿回收已创建资源。
func (s *Service) failInstanceWithCompensation(ctx context.Context, instanceID, tenantID int64, sourceRef string, cause error) error {
	recycleErr := s.recycleEngines(ctx, tenantID, sourceRef, "create-failed")
	_, statusErr := s.store.UpdateInstanceStatus(ctx, instanceID, InstanceStatusError)
	if ae, ok := apperr.As(cause); ok && ae.Code == apperr.ErrExperimentEngineUnavailable.Code {
		return apperr.ErrExperimentEngineUnavailable.WithCause(errors.Join(cause, recycleErr, statusErr))
	}
	return apperr.ErrExperimentEngineFailed.WithCause(errors.Join(cause, recycleErr, statusErr))
}

// rebuildReleasedInstance 为环境已释放的实例重新创建引擎资源。
func (s *Service) rebuildReleasedInstance(ctx context.Context, instance ExperimentInstanceDTO) (ExperimentInstanceDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ExperimentInstanceDTO{}, apperr.ErrUnauthorized
	}
	experimentID, _ := ids.Parse(instance.ExperimentID)
	instanceID, _ := ids.Parse(instance.ID)
	experiment, err := s.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	sourceRef := instance.SourceRef
	sandboxes, sims, err := s.createEngineResources(ctx, id, sourceRef, experiment.Components)
	if err != nil {
		return ExperimentInstanceDTO{}, s.failInstanceWithCompensation(ctx, instanceID, id.TenantID, sourceRef, err)
	}
	return s.store.UpdateInstanceResources(ctx, instanceID, sandboxes, sims, InstanceStatusRunning)
}

// recycleEngines 按 source_ref 回收沙箱与仿真资源。
func (s *Service) recycleEngines(ctx context.Context, tenantID int64, sourceRef, reason string) error {
	var errs []error
	if s.sandbox != nil {
		errs = append(errs, s.sandbox.RecycleBySourceRef(ctx, tenantID, sourceRef, reason))
	}
	if s.sim != nil {
		errs = append(errs, s.sim.RecycleSimBySourceRef(ctx, tenantID, sourceRef, reason))
	}
	return errors.Join(errs...)
}

// recycleEnginesForInstance 按实例中已持久化的资源引用校验契约后回收,避免缺引擎依赖时误报成功。
func (s *Service) recycleEnginesForInstance(ctx context.Context, tenantID int64, instance ExperimentInstanceDTO, reason string) error {
	if len(instance.Sandboxes) > 0 && s.sandbox == nil {
		return apperr.ErrExperimentEngineUnavailable
	}
	if len(instance.Sims) > 0 && s.sim == nil {
		return apperr.ErrExperimentEngineUnavailable
	}
	return s.recycleEngines(ctx, tenantID, instance.SourceRef, reason)
}

// ensureInstanceAccess 校验单人实例归属或小组成员访问权限。
func (s *Service) ensureInstanceAccess(ctx context.Context, instance ExperimentInstanceDTO) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	ownerID, _ := ids.Parse(instance.OwnerAccountID)
	if ownerID == id.AccountID {
		return nil
	}
	groupID, ok := ids.Parse(instance.GroupID)
	if !ok {
		return apperr.ErrExperimentForbidden
	}
	group, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return err
	}
	for _, member := range group.Members {
		memberID, _ := ids.Parse(member.StudentID)
		if memberID == id.AccountID {
			return nil
		}
	}
	return apperr.ErrExperimentForbidden
}

// ensureExperimentManager 校验当前账号是实验作者或学校管理员。
func (s *Service) ensureExperimentManager(ctx context.Context, experiment ExperimentDTO) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return nil
	}
	authorID, _ := ids.Parse(experiment.AuthorID)
	if authorID == id.AccountID {
		return nil
	}
	if s.identity != nil {
		allowed, err := s.identity.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
		if err != nil {
			return apperr.ErrExperimentForbidden.WithCause(err)
		}
		if allowed {
			return nil
		}
	}
	return apperr.ErrExperimentForbidden
}

// ensureGroupManager 读取服务端身份并确认是否为学校管理员,目标归属由 SQL 原子授权写入校验。
func (s *Service) ensureGroupManager(ctx context.Context) (tenant.Identity, bool, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return tenant.Identity{}, false, apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return id, false, nil
	}
	if s.identity == nil {
		return tenant.Identity{}, false, apperr.ErrExperimentForbidden
	}
	allowed, err := s.identity.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return tenant.Identity{}, false, apperr.ErrExperimentForbidden.WithCause(err)
	}
	return id, allowed, nil
}

// publishScored 发布实验得分事件给后续成绩或聚合流程消费。
func (s *Service) publishScored(ctx context.Context, tenantID int64, instance ExperimentInstanceDTO) error {
	if instance.Score == nil {
		return apperr.ErrExperimentScoreInvalid
	}
	if s.bus == nil {
		return apperr.ErrExperimentEventFailed
	}
	instanceID, _ := ids.Parse(instance.ID)
	experimentID, _ := ids.Parse(instance.ExperimentID)
	ownerID, _ := ids.Parse(instance.OwnerAccountID)
	if err := s.bus.Publish(ctx, contracts.SubjectExperimentScored, contracts.ExperimentScoredEvent{
		TenantID: tenantID, InstanceID: instanceID, ExperimentID: experimentID, StudentID: ownerID, Score: *instance.Score, ScoredAt: timex.Now(),
	}); err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	return nil
}

// validateReportContentRef 校验报告对象 key 绑定当前租户、实例和学生,拒绝客户端任意 key 越权。
func validateReportContentRef(id tenant.Identity, instance ExperimentInstanceDTO, contentRef string) error {
	instanceID, ok := ids.Parse(instance.ID)
	if !ok {
		return apperr.ErrExperimentInstanceInvalid
	}
	if id.AccountID <= 0 {
		return apperr.ErrExperimentInstanceInvalid
	}
	parts := strings.Split(contentRef, "/")
	if len(parts) != 6 || parts[5] == "" || strings.Contains(parts[5], "\\") {
		return apperr.ErrExperimentReportInvalid
	}
	expectedPrefix, err := storage.ObjectKey(id.TenantID, "experiment", "report", ids.Format(instanceID), ids.Format(id.AccountID), parts[5])
	if err != nil {
		return apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	if contentRef != expectedPrefix {
		return apperr.ErrExperimentReportInvalid
	}
	return nil
}

// sandboxRefFromInfo 转换 M2 沙箱摘要为 M7 持久化引用。
func sandboxRefFromInfo(info contracts.SandboxInfo, runtimeCode string) SandboxRef {
	tools := make([]SandboxToolAccessDTO, 0, len(info.ToolAccess))
	for _, tool := range info.ToolAccess {
		tools = append(tools, SandboxToolAccessDTO{Code: tool.ToolCode, Kind: tool.Kind, Endpoint: tool.Endpoint, Status: tool.Status})
	}
	return SandboxRef{ID: info.SandboxID, Ref: ids.Format(info.SandboxID), Runtime: runtimeCode, Tools: tools}
}

// simRefFromInfo 转换 M4 仿真摘要为 M7 持久化引用。
func simRefFromInfo(info contracts.SimSessionInfo) SimSessionRef {
	return SimSessionRef{ID: info.SessionID, Ref: ids.Format(info.SessionID), PackageCode: info.PackageCode, Version: info.Version, BundleRef: info.BundleRef}
}

// sourceRefForInstance 构造符合全局规范的实验实例来源引用。
func sourceRefForInstance(instanceID int64) string {
	return fmt.Sprintf("exp:%d:instance:%d", timex.Now().Year(), instanceID)
}

// firstSandboxRef 取首个沙箱作为现场判题目标。
func firstSandboxRef(refs []SandboxRef) string {
	if len(refs) == 0 {
		return ""
	}
	return refs[0].Ref
}

// findCheckpoint 根据 ID 查找检查点配置。
func findCheckpoint(components ExperimentComponents, checkpointID string) (CheckpointComponent, bool) {
	for _, cp := range components.Checkpoints {
		if cp.ID == checkpointID {
			return cp, true
		}
	}
	return CheckpointComponent{}, false
}

// hasErrorIssue 判断校验结果中是否包含阻断发布的问题。
func hasErrorIssue(issues []ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Level == "error" {
			return true
		}
	}
	return false
}

// validateScore 校验人工报告分。
func validateScore(score float64) error {
	if score < 0 || score > 100 {
		return apperr.ErrExperimentScoreInvalid
	}
	return nil
}

// tenantFromContext 读取当前请求租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) { return tenant.FromContext(ctx) }
