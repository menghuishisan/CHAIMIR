// M7 服务测试替身:为服务规则测试提供内存 store、引擎 contracts 和事件总线。
package experiment

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/tenant"
)

// fixedIDGen 返回固定雪花 ID,让测试断言稳定。
type fixedIDGen int64

// Generate 返回固定 ID。
func (g fixedIDGen) Generate() int64 { return int64(g) }

// fakeExperimentStore 是 M7 服务测试用内存数据访问替身。
type fakeExperimentStore struct {
	experiment        ExperimentDTO
	instance          ExperimentInstanceDTO
	instanceStatus    int16
	checkpointResults []ScorePart
	reportScore       *float64
	scoreWriteDrops   bool
	pendingJudge      PendingCheckpoint
	lastCheckpoint    CheckpointResultDTO
	releasedSandboxID int64
}

// ListExperiments 返回测试实验列表。
func (s *fakeExperimentStore) ListExperiments(context.Context, int64, int16, int, int) ([]ExperimentDTO, int64, error) {
	return []ExperimentDTO{s.experiment}, 1, nil
}

// CreateExperiment 保存测试实验定义。
func (s *fakeExperimentStore) CreateExperiment(_ context.Context, id tenant.Identity, experimentID int64, req ExperimentRequest) (ExperimentDTO, error) {
	s.experiment = ExperimentDTO{ID: ids.Format(experimentID), TenantID: ids.Format(id.TenantID), AuthorID: ids.Format(id.AccountID), Name: req.Name, Components: req.Components, CollabMode: normalizedCollabMode(req.CollabMode), Status: ExperimentStatusDraft}
	return s.experiment, nil
}

// GetExperiment 返回测试实验定义。
func (s *fakeExperimentStore) GetExperiment(context.Context, int64) (ExperimentDTO, error) {
	return s.experiment, nil
}

// UpdateExperiment 更新测试实验定义。
func (s *fakeExperimentStore) UpdateExperiment(_ context.Context, id int64, req ExperimentRequest) (ExperimentDTO, error) {
	s.experiment.ID = ids.Format(id)
	s.experiment.Name = req.Name
	s.experiment.Components = req.Components
	return s.experiment, nil
}

// UpdateExperimentStatus 更新测试实验状态。
func (s *fakeExperimentStore) UpdateExperimentStatus(_ context.Context, id int64, status int16) (ExperimentDTO, error) {
	s.experiment.ID = ids.Format(id)
	s.experiment.Status = status
	return s.experiment, nil
}

// CreateInstance 保存测试实例。
func (s *fakeExperimentStore) CreateInstance(_ context.Context, id tenant.Identity, instanceID, experimentID, groupID int64, sourceRef string) (ExperimentInstanceDTO, error) {
	s.instance = ExperimentInstanceDTO{ID: ids.Format(instanceID), TenantID: ids.Format(id.TenantID), ExperimentID: ids.Format(experimentID), OwnerAccountID: ids.Format(id.AccountID), GroupID: optionalTestID(groupID), SourceRef: sourceRef, Status: InstanceStatusCreating}
	return s.instance, nil
}

// GetInstance 返回测试实例。
func (s *fakeExperimentStore) GetInstance(context.Context, int64) (ExperimentInstanceDTO, error) {
	return s.instance, nil
}

// UpdateInstanceResources 更新测试实例资源引用。
func (s *fakeExperimentStore) UpdateInstanceResources(_ context.Context, id int64, sandboxes []SandboxRef, sims []SimSessionRef, status int16) (ExperimentInstanceDTO, error) {
	s.instance.ID = ids.Format(id)
	s.instance.Sandboxes = sandboxes
	s.instance.Sims = sims
	s.instance.Status = status
	s.instanceStatus = status
	return s.instance, nil
}

// UpdateInstanceStatus 更新测试实例状态。
func (s *fakeExperimentStore) UpdateInstanceStatus(_ context.Context, id int64, status int16) (ExperimentInstanceDTO, error) {
	s.instance.ID = ids.Format(id)
	s.instance.Status = status
	s.instanceStatus = status
	return s.instance, nil
}

// UpdateInstanceScore 更新测试实例总分。
func (s *fakeExperimentStore) UpdateInstanceScore(_ context.Context, id int64, score float64) (ExperimentInstanceDTO, error) {
	s.instance.ID = ids.Format(id)
	if s.scoreWriteDrops {
		s.instance.Score = nil
	} else {
		s.instance.Score = &score
	}
	s.instance.Status = InstanceStatusCompleted
	return s.instance, nil
}

// MarkInstancesReleasedBySandbox 记录被释放的沙箱 ID。
func (s *fakeExperimentStore) MarkInstancesReleasedBySandbox(_ context.Context, _ int64, sandboxID int64) ([]ExperimentInstanceDTO, error) {
	s.releasedSandboxID = sandboxID
	s.instance.Status = InstanceStatusReleased
	return []ExperimentInstanceDTO{s.instance}, nil
}

// UpsertCheckpointResult 保存最近一次检查点结果。
func (s *fakeExperimentStore) UpsertCheckpointResult(_ context.Context, result CheckpointResultDTO) (CheckpointResultDTO, error) {
	s.lastCheckpoint = result
	return result, nil
}

// PendingCheckpointByJudgeTask 返回等待回写的检查点定位。
func (s *fakeExperimentStore) PendingCheckpointByJudgeTask(context.Context, int64, int64) (PendingCheckpoint, error) {
	return s.pendingJudge, nil
}

// ListCheckpointScores 返回测试检查点分值。
func (s *fakeExperimentStore) ListCheckpointScores(context.Context, int64) ([]ScorePart, error) {
	return s.checkpointResults, nil
}

// LatestReportScore 返回测试报告分。
func (s *fakeExperimentStore) LatestReportScore(context.Context, int64) (*float64, error) {
	return s.reportScore, nil
}

// CreateReport 保存测试报告。
func (s *fakeExperimentStore) CreateReport(_ context.Context, id tenant.Identity, reportID, instanceID int64, contentRef string) (ReportDTO, error) {
	return ReportDTO{ID: ids.Format(reportID), InstanceID: ids.Format(instanceID), StudentID: ids.Format(id.AccountID), ContentRef: contentRef, Status: ReportStatusSubmitted}, nil
}

// ListReports 返回空报告列表。
func (s *fakeExperimentStore) ListReports(context.Context, int64, int, int) ([]ReportDTO, error) {
	return []ReportDTO{}, nil
}

// GradeReportAuthorized 返回已批改报告。
func (s *fakeExperimentStore) GradeReportAuthorized(_ context.Context, _ tenant.Identity, _ bool, reportID int64, score float64, comment string) (ReportDTO, error) {
	return ReportDTO{ID: ids.Format(reportID), ManualScore: &score, Comment: comment, Status: ReportStatusGraded}, nil
}

// CreateGroup 返回测试协作小组。
func (s *fakeExperimentStore) CreateGroup(_ context.Context, _ tenant.Identity, groupID, experimentID int64, name string) (GroupDTO, error) {
	return GroupDTO{ID: ids.Format(groupID), ExperimentID: ids.Format(experimentID), Name: name}, nil
}

// AddGroupMemberAuthorized 返回测试小组成员。
func (s *fakeExperimentStore) AddGroupMemberAuthorized(_ context.Context, _ tenant.Identity, _ bool, memberID, groupID, studentID int64, role string) (GroupMemberDTO, error) {
	return GroupMemberDTO{ID: ids.Format(memberID), GroupID: ids.Format(groupID), StudentID: ids.Format(studentID), Role: role}, nil
}

// GetGroup 返回包含当前测试账号的小组。
func (s *fakeExperimentStore) GetGroup(context.Context, int64) (GroupDTO, error) {
	return GroupDTO{ID: "3001", Members: []GroupMemberDTO{{StudentID: "200", Role: "leader"}}}, nil
}

// GetGroupForExperiment 返回指定实验下的测试小组。
func (s *fakeExperimentStore) GetGroupForExperiment(context.Context, int64, int64) (GroupDTO, error) {
	return s.GetGroup(context.Background(), 3001)
}

// Stats 返回测试统计。
func (s *fakeExperimentStore) Stats(_ context.Context, tenantID, courseID int64) (StatsDTO, error) {
	return StatsDTO{TenantID: ids.Format(tenantID), CourseID: optionalTestID(courseID), ExperimentCount: 1, ActiveInstanceCount: 1}, nil
}

// fakeSandboxService 是测试用 M2 contract 替身。
type fakeSandboxService struct {
	recycled []string
}

// CreateSandbox 返回测试沙箱摘要。
func (s *fakeSandboxService) CreateSandbox(_ context.Context, req contracts.SandboxCreateRequest) (contracts.SandboxInfo, error) {
	return contracts.SandboxInfo{SandboxID: 9001, TenantID: req.TenantID, SourceRef: req.SourceRef, OwnerID: req.OwnerAccountID}, nil
}

// GetSandbox 返回测试沙箱摘要。
func (s *fakeSandboxService) GetSandbox(context.Context, int64) (contracts.SandboxInfo, error) {
	return contracts.SandboxInfo{}, nil
}

// RecycleBySourceRef 记录回收请求。
func (s *fakeSandboxService) RecycleBySourceRef(_ context.Context, _ int64, sourceRef, _ string) error {
	s.recycled = append(s.recycled, sourceRef)
	return nil
}

// PutSandboxFile 是测试替身的文件写入入口。
func (s *fakeSandboxService) PutSandboxFile(context.Context, contracts.SandboxFileWrite) error {
	return nil
}

// SaveSandboxFiles 是测试替身的文件持久化入口。
func (s *fakeSandboxService) SaveSandboxFiles(context.Context, int64) (string, error) { return "", nil }

// ExecSandboxCommand 是测试替身的命令执行入口。
func (s *fakeSandboxService) ExecSandboxCommand(context.Context, contracts.SandboxExecRequest) (contracts.SandboxExecResult, error) {
	return contracts.SandboxExecResult{}, nil
}

// ChainDeploy 是测试替身的链上部署入口。
func (s *fakeSandboxService) ChainDeploy(context.Context, int64, map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}

// ChainSendTx 是测试替身的链上交易入口。
func (s *fakeSandboxService) ChainSendTx(context.Context, int64, map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}

// ChainQuery 是测试替身的链上查询入口。
func (s *fakeSandboxService) ChainQuery(context.Context, int64, string) (map[string]any, error) {
	return map[string]any{}, nil
}

// ChainReset 是测试替身的链上重置入口。
func (s *fakeSandboxService) ChainReset(context.Context, int64) error { return nil }

// Stats 返回测试沙箱统计。
func (s *fakeSandboxService) Stats(context.Context, int64) (contracts.SandboxStats, error) {
	return contracts.SandboxStats{}, nil
}

// fakeSimService 是测试用 M4 contract 替身。
type fakeSimService struct {
	createErr error
}

// CreateSimSession 返回测试仿真会话或注入错误。
func (s *fakeSimService) CreateSimSession(_ context.Context, req contracts.SimCreateSessionRequest) (contracts.SimSessionInfo, error) {
	if s.createErr != nil {
		return contracts.SimSessionInfo{}, s.createErr
	}
	return contracts.SimSessionInfo{SessionID: 7001, TenantID: req.TenantID, PackageCode: req.PackageCode, Version: req.Version, SourceRef: req.SourceRef}, nil
}

// GetSimReplay 返回测试回放。
func (s *fakeSimService) GetSimReplay(context.Context, int64, int64) (contracts.SimReplayInfo, error) {
	return contracts.SimReplayInfo{}, nil
}

// ReportSimCheckpoint 保存测试检查点。
func (s *fakeSimService) ReportSimCheckpoint(context.Context, contracts.SimCheckpointRequest) error {
	return nil
}

// RecycleSimBySourceRef 记录测试仿真回收。
func (s *fakeSimService) RecycleSimBySourceRef(context.Context, int64, string, string) error {
	return nil
}

// fakeEventBus 是测试用事件总线。
type fakeEventBus struct {
	published []publishedEvent
	subErr    error
}

// publishedEvent 是测试事件记录。
type publishedEvent struct {
	subject string
	payload any
}

// Publish 记录发布事件。
func (b *fakeEventBus) Publish(_ context.Context, subject string, payload any) error {
	b.published = append(b.published, publishedEvent{subject: subject, payload: payload})
	return nil
}

// Subscribe 返回测试订阅。
func (b *fakeEventBus) Subscribe(string, string, eventbus.Handler) (eventbus.Subscription, error) {
	if b.subErr != nil {
		return nil, b.subErr
	}
	return fakeSubscription{}, nil
}

// fakeContentReadService 是测试用 M5 内容读取 contract。
type fakeContentReadService struct {
	err error
}

// GetContentFace 返回测试题面或注入错误。
func (s *fakeContentReadService) GetContentFace(context.Context, int64, contracts.ContentItemRef) (contracts.ContentItemSnapshot, error) {
	if s.err != nil {
		return contracts.ContentItemSnapshot{}, s.err
	}
	return contracts.ContentItemSnapshot{ItemCode: "p1", ItemVersion: "1.0.0"}, nil
}

// GetContentFull 返回测试全量内容。
func (s *fakeContentReadService) GetContentFull(context.Context, int64, contracts.ContentItemRef) (contracts.ContentItemSnapshot, error) {
	return contracts.ContentItemSnapshot{}, nil
}

// BatchGetContentFace 返回空题面列表。
func (s *fakeContentReadService) BatchGetContentFace(context.Context, int64, []contracts.ContentItemRef) ([]contracts.ContentItemSnapshot, error) {
	return []contracts.ContentItemSnapshot{}, nil
}

// IncrementContentUsage 记录测试内容引用。
func (s *fakeContentReadService) IncrementContentUsage(context.Context, int64, contracts.ContentItemRef) error {
	return nil
}

// fakeJudgeService 是测试用 M3 判题 contract。
type fakeJudgeService struct{}

// SubmitJudgeTask 返回测试判题任务。
func (s *fakeJudgeService) SubmitJudgeTask(_ context.Context, req contracts.JudgeSubmitRequest) (contracts.JudgeTaskInfo, error) {
	return contracts.JudgeTaskInfo{TaskID: 3001, TenantID: req.TenantID, SourceRef: req.SourceRef}, nil
}

// GetJudgeTask 返回测试判题任务。
func (s *fakeJudgeService) GetJudgeTask(context.Context, int64) (contracts.JudgeTaskInfo, error) {
	return contracts.JudgeTaskInfo{}, nil
}

// Rejudge 返回测试重判任务。
func (s *fakeJudgeService) Rejudge(context.Context, int64) (contracts.JudgeTaskInfo, error) {
	return contracts.JudgeTaskInfo{}, nil
}

// Close 关闭测试事件总线。
func (b *fakeEventBus) Close() {}

// fakeSubscription 是测试订阅句柄。
type fakeSubscription struct{}

// Unsubscribe 取消测试订阅。
func (fakeSubscription) Unsubscribe() error { return nil }

// optionalTestID 转换测试可选 ID。
func optionalTestID(id int64) string {
	if id <= 0 {
		return ""
	}
	return ids.Format(id)
}
