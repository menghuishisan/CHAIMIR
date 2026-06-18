// experiment repo 文件定义 M7 持久化接口和数据库事务边界,只操作实验模块自有表。
package experiment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chaimir/internal/modules/experiment/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// Store 定义 service 所需的 experiment 持久化事务入口。
type Store interface {
	// TenantTx 在注入 RLS 租户变量后访问 M7 租户表。
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
	// PrivilegedTx 在受控后台任务中跨租户扫描 M7 自有表。
	PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error
}

// TxStore 定义单个事务内可调用的数据访问能力,不暴露 sqlc 行类型。
type TxStore interface {
	CreateExperiment(context.Context, Experiment) (Experiment, error)
	GetExperiment(context.Context, int64, int64) (Experiment, error)
	ListExperiments(context.Context, int64, int64, int16, int, int) ([]Experiment, int64, error)
	UpdateExperiment(context.Context, Experiment) (Experiment, error)
	SetExperimentStatus(context.Context, int64, int64, int16) (Experiment, error)
	CreateGroup(context.Context, ExperimentGroup) (ExperimentGroup, error)
	GetGroup(context.Context, int64, int64) (ExperimentGroup, error)
	ListGroupMembers(context.Context, int64, int64) ([]GroupMember, error)
	GetGroupMember(context.Context, int64, int64, int64) (GroupMember, error)
	UpsertGroupMember(context.Context, GroupMember) (GroupMember, error)
	GetActiveGroupInstance(context.Context, int64, int64, int64) (ExperimentInstance, error)
	CreateInstance(context.Context, ExperimentInstance) (ExperimentInstance, error)
	GetInstance(context.Context, int64, int64) (ExperimentInstance, error)
	GetInstanceForUpdate(context.Context, int64, int64) (ExperimentInstance, error)
	GetInstanceBySourceRef(context.Context, int64, string) (ExperimentInstance, error)
	UpdateInstanceResources(context.Context, int64, int64, []SandboxRef, []SimSessionRef, int16) (ExperimentInstance, error)
	SetInstanceStatus(context.Context, int64, int64, int16) (ExperimentInstance, error)
	FinishInstance(context.Context, int64, int64, float64) (ExperimentInstance, error)
	UpdateInstanceScore(context.Context, int64, int64, float64) (ExperimentInstance, error)
	TouchInstance(context.Context, int64, int64) (ExperimentInstance, error)
	ClaimRecyclableInstances(context.Context, int, int, int32) ([]ExperimentInstance, error)
	UpsertCheckpoint(context.Context, CheckpointResult) (CheckpointResult, error)
	GetCheckpointByJudgeTask(context.Context, int64, string) (CheckpointResult, error)
	ListCheckpoints(context.Context, int64, int64) ([]CheckpointResult, error)
	UpsertReport(context.Context, ExperimentReport) (ExperimentReport, error)
	GradeReport(context.Context, int64, int64, float64, string) (ExperimentReport, error)
	GetReport(context.Context, int64, int64) (ExperimentReport, error)
	GetReportByInstanceStudent(context.Context, int64, int64, int64) (ExperimentReport, error)
	ListReports(context.Context, int64, int64, int, int) ([]ExperimentReport, int64, error)
	SumScores(context.Context, int64, int64) (float64, error)
	Stats(context.Context, int64, int64) (ExperimentStatsSnapshot, error)
	CreateExperimentScoreOutbox(context.Context, int64, ExperimentInstance, string, time.Time) (ExperimentScoreOutbox, error)
	ClaimPendingExperimentScoreOutbox(context.Context, int32, time.Time) ([]ExperimentScoreOutbox, error)
	MarkExperimentScoreOutboxPublished(context.Context, int64, int64) (ExperimentScoreOutbox, error)
	MarkExperimentScoreOutboxFailed(context.Context, int64, int64, string) (ExperimentScoreOutbox, error)
}

type store struct{ database *db.DB }
type txStore struct{ q *sqlcgen.Queries }

// NewStore 创建 experiment 模块持久化入口,仅装配层应调用。
func NewStore(database *db.DB) Store { return &store{database: database} }

// TenantTx 在当前租户事务中执行 M7 自有表读写。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("experiment store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// PrivilegedTx 在 experiment 模块自有表内执行受控后台扫描事务。
func (s *store) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("experiment store 未初始化")
	}
	return s.database.WithPrivilegedModuleTx(ctx, "experiment", func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// isNoRows 统一识别未命中错误,让 service 不直接依赖 pgx。
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// CreateExperiment 创建向导草稿。
func (tx *txStore) CreateExperiment(ctx context.Context, item Experiment) (Experiment, error) {
	components, err := encodeJSON(item.Components, apperr.ErrExperimentInvalid)
	if err != nil {
		return Experiment{}, err
	}
	groupConfig, err := encodeJSON(item.GroupConfig, apperr.ErrExperimentGroupInvalid)
	if err != nil {
		return Experiment{}, err
	}
	row, err := tx.q.CreateExperiment(ctx, sqlcgen.CreateExperimentParams{ID: item.ID, TenantID: item.TenantID, CourseID: pgtypex.Int8(item.CourseID), AuthorID: item.AuthorID, TemplateRef: pgtypex.Text(item.TemplateRef), TemplateVersion: pgtypex.Text(item.TemplateVersion), Name: item.Name, Description: item.Description, Components: components, CollabMode: item.CollabMode, GroupConfig: groupConfig, RequireReport: item.RequireReport, WizardStep: item.WizardStep})
	if err != nil {
		return Experiment{}, apperr.ErrExperimentInvalid.WithCause(err)
	}
	return experimentFromRow(row)
}

// GetExperiment 读取实验定义。
func (tx *txStore) GetExperiment(ctx context.Context, tenantID, id int64) (Experiment, error) {
	row, err := tx.q.GetExperiment(ctx, sqlcgen.GetExperimentParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Experiment{}, apperr.ErrExperimentNotFound.WithCause(err)
	}
	return experimentFromRow(row)
}

// ListExperiments 查询实验定义分页。
func (tx *txStore) ListExperiments(ctx context.Context, tenantID, courseID int64, status int16, page, size int) ([]Experiment, int64, error) {
	rows, err := tx.q.ListExperiments(ctx, sqlcgen.ListExperimentsParams{TenantID: tenantID, Column2: courseID, Column3: status, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, apperr.ErrExperimentInvalid.WithCause(err)
	}
	total, err := tx.q.CountExperiments(ctx, sqlcgen.CountExperimentsParams{TenantID: tenantID, Column2: courseID, Column3: status})
	if err != nil {
		return nil, 0, apperr.ErrExperimentInvalid.WithCause(err)
	}
	out := make([]Experiment, 0, len(rows))
	for _, row := range rows {
		item, err := experimentFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, nil
}

// UpdateExperiment 更新草稿实验定义。
func (tx *txStore) UpdateExperiment(ctx context.Context, item Experiment) (Experiment, error) {
	components, err := encodeJSON(item.Components, apperr.ErrExperimentInvalid)
	if err != nil {
		return Experiment{}, err
	}
	groupConfig, err := encodeJSON(item.GroupConfig, apperr.ErrExperimentGroupInvalid)
	if err != nil {
		return Experiment{}, err
	}
	row, err := tx.q.UpdateExperiment(ctx, sqlcgen.UpdateExperimentParams{TenantID: item.TenantID, ID: item.ID, CourseID: pgtypex.Int8(item.CourseID), TemplateRef: pgtypex.Text(item.TemplateRef), TemplateVersion: pgtypex.Text(item.TemplateVersion), Name: item.Name, Description: item.Description, Components: components, CollabMode: item.CollabMode, GroupConfig: groupConfig, RequireReport: item.RequireReport, WizardStep: item.WizardStep})
	if err != nil {
		return Experiment{}, apperr.ErrExperimentStateInvalid.WithCause(err)
	}
	return experimentFromRow(row)
}

// SetExperimentStatus 更新实验发布状态。
func (tx *txStore) SetExperimentStatus(ctx context.Context, tenantID, id int64, status int16) (Experiment, error) {
	row, err := tx.q.SetExperimentStatus(ctx, sqlcgen.SetExperimentStatusParams{TenantID: tenantID, ID: id, Status: status})
	if err != nil {
		return Experiment{}, apperr.ErrExperimentStateInvalid.WithCause(err)
	}
	return experimentFromRow(row)
}

// CreateGroup 创建多人协作小组。
func (tx *txStore) CreateGroup(ctx context.Context, item ExperimentGroup) (ExperimentGroup, error) {
	row, err := tx.q.CreateExperimentGroup(ctx, sqlcgen.CreateExperimentGroupParams{ID: item.ID, TenantID: item.TenantID, ExperimentID: item.ExperimentID, Name: item.Name})
	if err != nil {
		return ExperimentGroup{}, apperr.ErrExperimentGroupInvalid.WithCause(err)
	}
	return groupFromRows(row, nil), nil
}

// GetGroup 读取小组和成员。
func (tx *txStore) GetGroup(ctx context.Context, tenantID, id int64) (ExperimentGroup, error) {
	row, err := tx.q.GetExperimentGroup(ctx, sqlcgen.GetExperimentGroupParams{TenantID: tenantID, ID: id})
	if err != nil {
		return ExperimentGroup{}, apperr.ErrExperimentGroupNotFound.WithCause(err)
	}
	members, err := tx.q.ListGroupMembers(ctx, sqlcgen.ListGroupMembersParams{TenantID: tenantID, GroupID: id})
	if err != nil {
		return ExperimentGroup{}, apperr.ErrExperimentGroupInvalid.WithCause(err)
	}
	return groupFromRows(row, members), nil
}

// ListGroupMembers 查询小组成员列表。
func (tx *txStore) ListGroupMembers(ctx context.Context, tenantID, groupID int64) ([]GroupMember, error) {
	rows, err := tx.q.ListGroupMembers(ctx, sqlcgen.ListGroupMembersParams{TenantID: tenantID, GroupID: groupID})
	if err != nil {
		return nil, apperr.ErrExperimentGroupInvalid.WithCause(err)
	}
	out := make([]GroupMember, 0, len(rows))
	for _, row := range rows {
		out = append(out, groupMemberFromRow(row))
	}
	return out, nil
}

// GetGroupMember 读取单个小组成员。
func (tx *txStore) GetGroupMember(ctx context.Context, tenantID, groupID, studentID int64) (GroupMember, error) {
	row, err := tx.q.GetGroupMember(ctx, sqlcgen.GetGroupMemberParams{TenantID: tenantID, GroupID: groupID, StudentID: studentID})
	if err != nil {
		return GroupMember{}, apperr.ErrExperimentGroupMemberDenied.WithCause(err)
	}
	return groupMemberFromRow(row), nil
}

// UpsertGroupMember 新增或更新小组成员角色。
func (tx *txStore) UpsertGroupMember(ctx context.Context, item GroupMember) (GroupMember, error) {
	row, err := tx.q.UpsertGroupMember(ctx, sqlcgen.UpsertGroupMemberParams{ID: item.ID, TenantID: item.TenantID, GroupID: item.GroupID, StudentID: item.StudentID, Role: item.Role})
	if err != nil {
		return GroupMember{}, apperr.ErrExperimentGroupInvalid.WithCause(err)
	}
	return groupMemberFromRow(row), nil
}

// GetActiveGroupInstance 查询小组当前共享实例。
func (tx *txStore) GetActiveGroupInstance(ctx context.Context, tenantID, experimentID, groupID int64) (ExperimentInstance, error) {
	row, err := tx.q.GetActiveGroupInstance(ctx, sqlcgen.GetActiveGroupInstanceParams{TenantID: tenantID, ExperimentID: experimentID, GroupID: pgtypex.Int8(groupID)})
	if err != nil {
		return ExperimentInstance{}, err
	}
	return instanceFromFields(row.ID, row.TenantID, row.ExperimentID, row.OwnerAccountID, row.GroupID, row.SourceRef, row.SandboxRefs, row.SimSessionRefs, row.Status, row.Score, row.StartedAt, row.FinishedAt, row.LastActiveAt)
}

// CreateInstance 创建实验实例控制记录。
func (tx *txStore) CreateInstance(ctx context.Context, item ExperimentInstance) (ExperimentInstance, error) {
	row, err := tx.q.CreateExperimentInstance(ctx, sqlcgen.CreateExperimentInstanceParams{ID: item.ID, TenantID: item.TenantID, ExperimentID: item.ExperimentID, OwnerAccountID: item.OwnerAccountID, GroupID: pgtypex.Int8(item.GroupID), SourceRef: item.SourceRef})
	if err != nil {
		return ExperimentInstance{}, apperr.ErrExperimentInstanceInvalid.WithCause(err)
	}
	return instanceFromCreateRow(row)
}

// GetInstance 读取实验实例。
func (tx *txStore) GetInstance(ctx context.Context, tenantID, id int64) (ExperimentInstance, error) {
	row, err := tx.q.GetExperimentInstance(ctx, sqlcgen.GetExperimentInstanceParams{TenantID: tenantID, ID: id})
	if err != nil {
		return ExperimentInstance{}, apperr.ErrExperimentInstanceNotFound.WithCause(err)
	}
	return instanceFromGetRow(row)
}

// GetInstanceForUpdate 锁定实例行,用于显式阶段激活的幂等资源追加。
func (tx *txStore) GetInstanceForUpdate(ctx context.Context, tenantID, id int64) (ExperimentInstance, error) {
	row, err := tx.q.GetExperimentInstanceForUpdate(ctx, sqlcgen.GetExperimentInstanceForUpdateParams{TenantID: tenantID, ID: id})
	if err != nil {
		return ExperimentInstance{}, apperr.ErrExperimentInstanceNotFound.WithCause(err)
	}
	return instanceFromForUpdateRow(row)
}

// GetInstanceBySourceRef 按 source_ref 读取实验实例。
func (tx *txStore) GetInstanceBySourceRef(ctx context.Context, tenantID int64, sourceRef string) (ExperimentInstance, error) {
	row, err := tx.q.GetExperimentInstanceBySourceRef(ctx, sqlcgen.GetExperimentInstanceBySourceRefParams{TenantID: tenantID, SourceRef: sourceRef})
	if err != nil {
		return ExperimentInstance{}, apperr.ErrExperimentInstanceNotFound.WithCause(err)
	}
	return instanceFromSourceRefRow(row)
}

// UpdateInstanceResources 保存实例已创建的 M2/M4 引用。
func (tx *txStore) UpdateInstanceResources(ctx context.Context, tenantID, id int64, sandboxes []SandboxRef, sims []SimSessionRef, status int16) (ExperimentInstance, error) {
	sandboxRaw, err := encodeJSON(sandboxes, apperr.ErrExperimentInstanceInvalid)
	if err != nil {
		return ExperimentInstance{}, err
	}
	simRaw, err := encodeJSON(sims, apperr.ErrExperimentInstanceInvalid)
	if err != nil {
		return ExperimentInstance{}, err
	}
	row, err := tx.q.UpdateInstanceResources(ctx, sqlcgen.UpdateInstanceResourcesParams{TenantID: tenantID, ID: id, SandboxRefs: sandboxRaw, SimSessionRefs: simRaw, Status: status})
	if err != nil {
		return ExperimentInstance{}, apperr.ErrExperimentInstanceStateInvalid.WithCause(err)
	}
	return instanceFromUpdateResourcesRow(row)
}

// SetInstanceStatus 保存实例状态。
func (tx *txStore) SetInstanceStatus(ctx context.Context, tenantID, id int64, status int16) (ExperimentInstance, error) {
	row, err := tx.q.SetInstanceStatus(ctx, sqlcgen.SetInstanceStatusParams{TenantID: tenantID, ID: id, Status: status})
	if err != nil {
		return ExperimentInstance{}, apperr.ErrExperimentInstanceStateInvalid.WithCause(err)
	}
	return instanceFromStatusRow(row)
}

// FinishInstance 保存实例最终得分和完成态。
func (tx *txStore) FinishInstance(ctx context.Context, tenantID, id int64, score float64) (ExperimentInstance, error) {
	row, err := tx.q.FinishExperimentInstance(ctx, sqlcgen.FinishExperimentInstanceParams{TenantID: tenantID, ID: id, Column3: fmt.Sprintf("%.2f", score)})
	if err != nil {
		return ExperimentInstance{}, apperr.ErrExperimentScoreInvalid.WithCause(err)
	}
	return instanceFromFinishRow(row)
}

// UpdateInstanceScore 只刷新已完成实例的分数,不改变生命周期状态。
func (tx *txStore) UpdateInstanceScore(ctx context.Context, tenantID, id int64, score float64) (ExperimentInstance, error) {
	row, err := tx.q.UpdateExperimentInstanceScore(ctx, sqlcgen.UpdateExperimentInstanceScoreParams{TenantID: tenantID, ID: id, Column3: fmt.Sprintf("%.2f", score)})
	if err != nil {
		return ExperimentInstance{}, apperr.ErrExperimentScoreInvalid.WithCause(err)
	}
	return instanceFromFields(row.ID, row.TenantID, row.ExperimentID, row.OwnerAccountID, row.GroupID, row.SourceRef, row.SandboxRefs, row.SimSessionRefs, row.Status, row.Score, row.StartedAt, row.FinishedAt, row.LastActiveAt)
}

// TouchInstance 刷新实例活跃时间。
func (tx *txStore) TouchInstance(ctx context.Context, tenantID, id int64) (ExperimentInstance, error) {
	row, err := tx.q.TouchExperimentInstance(ctx, sqlcgen.TouchExperimentInstanceParams{TenantID: tenantID, ID: id})
	if err != nil {
		return ExperimentInstance{}, apperr.ErrExperimentInstanceNotFound.WithCause(err)
	}
	return instanceFromFields(row.ID, row.TenantID, row.ExperimentID, row.OwnerAccountID, row.GroupID, row.SourceRef, row.SandboxRefs, row.SimSessionRefs, row.Status, row.Score, row.StartedAt, row.FinishedAt, row.LastActiveAt)
}

// ClaimRecyclableInstances 跨租户认领需要回收的 M7 自有实例。
func (tx *txStore) ClaimRecyclableInstances(ctx context.Context, pausedTimeoutSeconds, idleTimeoutSeconds int, limit int32) ([]ExperimentInstance, error) {
	rows, err := tx.q.ClaimRecyclableInstancesAcrossTenants(ctx, sqlcgen.ClaimRecyclableInstancesAcrossTenantsParams{Column1: fmt.Sprintf("%d", pausedTimeoutSeconds), Column2: fmt.Sprintf("%d", idleTimeoutSeconds), Limit: limit})
	if err != nil {
		return nil, apperr.ErrExperimentRecycleFailed.WithCause(err)
	}
	out := make([]ExperimentInstance, 0, len(rows))
	for _, row := range rows {
		item, err := instanceFromRecycleRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// UpsertCheckpoint 新增或更新检查点结果。
func (tx *txStore) UpsertCheckpoint(ctx context.Context, item CheckpointResult) (CheckpointResult, error) {
	if item.BindingOutput == nil {
		item.BindingOutput = map[string]any{}
	}
	bindingOutput, err := encodeJSON(item.BindingOutput, apperr.ErrExperimentCheckpointInvalid)
	if err != nil {
		return CheckpointResult{}, err
	}
	row, err := tx.q.UpsertCheckpointResult(ctx, sqlcgen.UpsertCheckpointResultParams{ID: item.ID, TenantID: item.TenantID, InstanceID: item.InstanceID, CheckpointID: item.CheckpointID, JudgeTaskRef: pgtypex.Text(item.JudgeTaskRef), Passed: item.Passed, Column7: fmt.Sprintf("%.2f", item.Score), DetailRef: pgtypex.Text(item.DetailRef), BindingOutput: bindingOutput})
	if err != nil {
		return CheckpointResult{}, apperr.ErrExperimentCheckpointInvalid.WithCause(err)
	}
	return checkpointFromUpsertRow(row)
}

// GetCheckpointByJudgeTask 按 M3 判题任务引用查找检查点结果。
func (tx *txStore) GetCheckpointByJudgeTask(ctx context.Context, tenantID int64, judgeTaskRef string) (CheckpointResult, error) {
	row, err := tx.q.GetCheckpointResultByJudgeTask(ctx, sqlcgen.GetCheckpointResultByJudgeTaskParams{TenantID: tenantID, JudgeTaskRef: pgtypex.Text(judgeTaskRef)})
	if err != nil {
		return CheckpointResult{}, apperr.ErrExperimentCheckpointInvalid.WithCause(err)
	}
	return checkpointFromJudgeTaskRow(row)
}

// ListCheckpoints 查询实例检查点结果。
func (tx *txStore) ListCheckpoints(ctx context.Context, tenantID, instanceID int64) ([]CheckpointResult, error) {
	rows, err := tx.q.ListCheckpointResults(ctx, sqlcgen.ListCheckpointResultsParams{TenantID: tenantID, InstanceID: instanceID})
	if err != nil {
		return nil, apperr.ErrExperimentCheckpointInvalid.WithCause(err)
	}
	out := make([]CheckpointResult, 0, len(rows))
	for _, row := range rows {
		item, err := checkpointFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// UpsertReport 保存学生实验报告引用。
func (tx *txStore) UpsertReport(ctx context.Context, item ExperimentReport) (ExperimentReport, error) {
	row, err := tx.q.UpsertExperimentReport(ctx, sqlcgen.UpsertExperimentReportParams{ID: item.ID, TenantID: item.TenantID, InstanceID: item.InstanceID, StudentID: item.StudentID, ContentRef: item.ContentRef})
	if err != nil {
		return ExperimentReport{}, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	return reportFromUpsertRow(row), nil
}

// GradeReport 保存教师报告批改分和评语。
func (tx *txStore) GradeReport(ctx context.Context, tenantID, id int64, score float64, comment string) (ExperimentReport, error) {
	row, err := tx.q.GradeExperimentReport(ctx, sqlcgen.GradeExperimentReportParams{TenantID: tenantID, ID: id, Column3: fmt.Sprintf("%.2f", score), Comment: pgtypex.Text(comment)})
	if err != nil {
		return ExperimentReport{}, apperr.ErrExperimentReportNotFound.WithCause(err)
	}
	return reportFromGradeRow(row), nil
}

// GetReport 读取报告。
func (tx *txStore) GetReport(ctx context.Context, tenantID, id int64) (ExperimentReport, error) {
	row, err := tx.q.GetExperimentReport(ctx, sqlcgen.GetExperimentReportParams{TenantID: tenantID, ID: id})
	if err != nil {
		return ExperimentReport{}, apperr.ErrExperimentReportNotFound.WithCause(err)
	}
	return ExperimentReport{ID: row.ID, TenantID: row.TenantID, InstanceID: row.InstanceID, StudentID: row.StudentID, ContentRef: row.ContentRef, ManualScore: row.ManualScore, Comment: pgtypex.TextValue(row.Comment), Status: row.Status, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt)}, nil
}

// GetReportByInstanceStudent 读取当前学生在指定实例下提交的报告。
func (tx *txStore) GetReportByInstanceStudent(ctx context.Context, tenantID, instanceID, studentID int64) (ExperimentReport, error) {
	row, err := tx.q.GetExperimentReportByInstanceStudent(ctx, sqlcgen.GetExperimentReportByInstanceStudentParams{TenantID: tenantID, InstanceID: instanceID, StudentID: studentID})
	if err != nil {
		return ExperimentReport{}, apperr.ErrExperimentReportNotFound.WithCause(err)
	}
	return ExperimentReport{ID: row.ID, TenantID: row.TenantID, InstanceID: row.InstanceID, StudentID: row.StudentID, ContentRef: row.ContentRef, ManualScore: row.ManualScore, Comment: pgtypex.TextValue(row.Comment), Status: row.Status, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt)}, nil
}

// ListReports 查询实验报告分页。
func (tx *txStore) ListReports(ctx context.Context, tenantID, experimentID int64, page, size int) ([]ExperimentReport, int64, error) {
	rows, err := tx.q.ListExperimentReports(ctx, sqlcgen.ListExperimentReportsParams{TenantID: tenantID, ExperimentID: experimentID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	total, err := tx.q.CountExperimentReports(ctx, sqlcgen.CountExperimentReportsParams{TenantID: tenantID, ExperimentID: experimentID})
	if err != nil {
		return nil, 0, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	out := make([]ExperimentReport, 0, len(rows))
	for _, row := range rows {
		out = append(out, reportFromListRow(row))
	}
	return out, total, nil
}

// SumScores 汇总检查点和已批改报告得分。
func (tx *txStore) SumScores(ctx context.Context, tenantID, instanceID int64) (float64, error) {
	checkpoint, err := tx.q.SumCheckpointScores(ctx, sqlcgen.SumCheckpointScoresParams{TenantID: tenantID, InstanceID: instanceID})
	if err != nil {
		return 0, apperr.ErrExperimentScoreInvalid.WithCause(err)
	}
	report, err := tx.q.SumReportScores(ctx, sqlcgen.SumReportScoresParams{TenantID: tenantID, InstanceID: instanceID})
	if err != nil {
		return 0, apperr.ErrExperimentScoreInvalid.WithCause(err)
	}
	return checkpoint + report, nil
}

// Stats 返回租户或课程维度实验统计。
func (tx *txStore) Stats(ctx context.Context, tenantID, courseID int64) (ExperimentStatsSnapshot, error) {
	row, err := tx.q.ExperimentStats(ctx, sqlcgen.ExperimentStatsParams{TenantID: tenantID, Column2: courseID})
	if err != nil {
		return ExperimentStatsSnapshot{}, apperr.ErrExperimentInvalid.WithCause(err)
	}
	return ExperimentStatsSnapshot{ExperimentCount: row.ExperimentCount, ActiveInstanceCount: row.ActiveInstanceCount}, nil
}

// CreateExperimentScoreOutbox 在实例得分变更事务内保存得分事件。
func (tx *txStore) CreateExperimentScoreOutbox(ctx context.Context, id int64, inst ExperimentInstance, traceID string, scoredAt time.Time) (ExperimentScoreOutbox, error) {
	row, err := tx.q.CreateExperimentScoreOutbox(ctx, sqlcgen.CreateExperimentScoreOutboxParams{ID: id, TenantID: inst.TenantID, ExperimentID: inst.ExperimentID, InstanceID: inst.ID, StudentID: inst.OwnerAccountID, Column6: fmt.Sprintf("%.2f", inst.Score), TraceID: traceID, ScoredAt: timex.RequiredTimestamptz(scoredAt)})
	if err != nil {
		return ExperimentScoreOutbox{}, apperr.ErrExperimentEventFailed.WithCause(err)
	}
	return experimentScoreOutbox(row), nil
}

// ClaimPendingExperimentScoreOutbox 跨租户领取待发布、失败待重试或卡住超时的得分事件。
func (tx *txStore) ClaimPendingExperimentScoreOutbox(ctx context.Context, limit int32, staleBefore time.Time) ([]ExperimentScoreOutbox, error) {
	rows, err := tx.q.ClaimPendingExperimentScoreOutbox(ctx, sqlcgen.ClaimPendingExperimentScoreOutboxParams{StaleBefore: timex.RequiredTimestamptz(staleBefore), PageLimit: limit})
	if err != nil {
		return nil, apperr.ErrExperimentEventFailed.WithCause(err)
	}
	out := make([]ExperimentScoreOutbox, 0, len(rows))
	for _, row := range rows {
		out = append(out, experimentScoreOutbox(row))
	}
	return out, nil
}

// MarkExperimentScoreOutboxPublished 标记得分事件投递成功。
func (tx *txStore) MarkExperimentScoreOutboxPublished(ctx context.Context, tenantID, id int64) (ExperimentScoreOutbox, error) {
	row, err := tx.q.MarkExperimentScoreOutboxPublished(ctx, sqlcgen.MarkExperimentScoreOutboxPublishedParams{TenantID: tenantID, ID: id})
	if err != nil {
		return ExperimentScoreOutbox{}, apperr.ErrExperimentEventFailed.WithCause(err)
	}
	return experimentScoreOutbox(row), nil
}

// MarkExperimentScoreOutboxFailed 标记得分事件投递失败并保留脱敏原因。
func (tx *txStore) MarkExperimentScoreOutboxFailed(ctx context.Context, tenantID, id int64, reason string) (ExperimentScoreOutbox, error) {
	row, err := tx.q.MarkExperimentScoreOutboxFailed(ctx, sqlcgen.MarkExperimentScoreOutboxFailedParams{TenantID: tenantID, ID: id, LastError: pgtypex.Text(reason)})
	if err != nil {
		return ExperimentScoreOutbox{}, apperr.ErrExperimentEventFailed.WithCause(err)
	}
	return experimentScoreOutbox(row), nil
}
