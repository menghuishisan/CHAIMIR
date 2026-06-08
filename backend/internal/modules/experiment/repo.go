// M7 数据访问层:封装 experiment 自有租户表的 sqlc 查询与 RLS 注入。
package experiment

import (
	"context"

	"chaimir/internal/modules/experiment/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// experimentStore 是服务层依赖的数据访问接口,便于服务逻辑测试。
type experimentStore interface {
	ListExperiments(context.Context, int64, int16, int, int) ([]ExperimentDTO, int64, error)
	CreateExperiment(context.Context, tenant.Identity, int64, ExperimentRequest) (ExperimentDTO, error)
	GetExperiment(context.Context, int64) (ExperimentDTO, error)
	UpdateExperiment(context.Context, int64, ExperimentRequest) (ExperimentDTO, error)
	UpdateExperimentStatus(context.Context, int64, int16) (ExperimentDTO, error)
	CreateInstance(context.Context, tenant.Identity, int64, int64, int64, string) (ExperimentInstanceDTO, error)
	GetInstance(context.Context, int64) (ExperimentInstanceDTO, error)
	UpdateInstanceResources(context.Context, int64, []SandboxRef, []SimSessionRef, int16) (ExperimentInstanceDTO, error)
	UpdateInstanceStatus(context.Context, int64, int16) (ExperimentInstanceDTO, error)
	UpdateInstanceScore(context.Context, int64, float64) (ExperimentInstanceDTO, error)
	MarkInstancesReleasedBySandbox(context.Context, int64, int64) ([]ExperimentInstanceDTO, error)
	UpsertCheckpointResult(context.Context, CheckpointResultDTO) (CheckpointResultDTO, error)
	PendingCheckpointByJudgeTask(context.Context, int64, int64) (PendingCheckpoint, error)
	ListCheckpointScores(context.Context, int64) ([]ScorePart, error)
	LatestReportScore(context.Context, int64) (*float64, error)
	CreateReport(context.Context, tenant.Identity, int64, int64, string) (ReportDTO, error)
	ListReports(context.Context, int64, int, int) ([]ReportDTO, error)
	GradeReportAuthorized(context.Context, tenant.Identity, bool, int64, float64, string) (ReportDTO, error)
	CreateGroup(context.Context, tenant.Identity, int64, int64, string) (GroupDTO, error)
	AddGroupMemberAuthorized(context.Context, tenant.Identity, bool, int64, int64, int64, string) (GroupMemberDTO, error)
	GetGroup(context.Context, int64) (GroupDTO, error)
	GetGroupForExperiment(context.Context, int64, int64) (GroupDTO, error)
	Stats(context.Context, int64, int64) (StatsDTO, error)
}

// repo 是 M7 模块数据库访问封装。
type repo struct {
	db *db.DB
}

// newRepo 构造 M7 repo。
func newRepo(database *db.DB) *repo { return &repo{db: database} }

// queryFunc 是 M7 sqlc 查询闭包。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 从请求上下文读取租户并注入 RLS。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 注入 RLS,供事件与 contracts 内部入口使用。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// ListExperiments 查询实验列表并返回同条件总数。
func (r *repo) ListExperiments(ctx context.Context, courseID int64, status int16, page, size int) ([]ExperimentDTO, int64, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.Experiment
	var total int64
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListExperiments(ctx, sqlcgen.ListExperimentsParams{
			CourseID: pgInt8Filter(courseID), Status: pgInt2Filter(status),
			OffsetCount: int32((page - 1) * size), LimitCount: int32(size),
		})
		if err != nil {
			return err
		}
		total, err = q.CountExperiments(ctx, pgInt8Filter(courseID))
		return err
	}); err != nil {
		return nil, 0, apperr.ErrExperimentQueryFailed.WithCause(err)
	}
	out := make([]ExperimentDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, experimentDTOFromRow(row))
	}
	return out, total, nil
}

// CreateExperiment 创建实验草稿。
func (r *repo) CreateExperiment(ctx context.Context, id tenant.Identity, experimentID int64, req ExperimentRequest) (ExperimentDTO, error) {
	components, groupConfig, err := experimentRequestJSON(req)
	if err != nil {
		return ExperimentDTO{}, err
	}
	var row sqlcgen.Experiment
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateExperiment(ctx, sqlcgen.CreateExperimentParams{
			ID: experimentID, TenantID: id.TenantID, CourseID: pgInt8FromString(req.CourseID), AuthorID: id.AccountID,
			TemplateRef: pgText(req.TemplateRef), TemplateVersion: pgText(req.TemplateVersion), Name: req.Name,
			Description: req.Description, Components: components, CollabMode: normalizedCollabMode(req.CollabMode),
			GroupConfig: groupConfig, RequireReport: req.RequireReport, WizardStep: normalizedWizardStep(req.WizardStep),
			Status: ExperimentStatusDraft,
		})
		return createErr
	}); err != nil {
		return ExperimentDTO{}, apperr.ErrExperimentInvalid.WithCause(err)
	}
	return experimentDTOFromRow(row), nil
}

// GetExperiment 读取单个实验定义。
func (r *repo) GetExperiment(ctx context.Context, id int64) (ExperimentDTO, error) {
	var row sqlcgen.Experiment
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetExperimentByID(ctx, id)
		return err
	}); err != nil {
		return ExperimentDTO{}, notFoundOrInternal(err, apperr.ErrExperimentNotFound)
	}
	return experimentDTOFromRow(row), nil
}

// UpdateExperiment 更新草稿或下架实验的编排配置。
func (r *repo) UpdateExperiment(ctx context.Context, id int64, req ExperimentRequest) (ExperimentDTO, error) {
	components, groupConfig, err := experimentRequestJSON(req)
	if err != nil {
		return ExperimentDTO{}, err
	}
	var row sqlcgen.Experiment
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.UpdateExperiment(ctx, sqlcgen.UpdateExperimentParams{
			ID: id, CourseID: pgInt8FromString(req.CourseID), TemplateRef: pgText(req.TemplateRef),
			TemplateVersion: pgText(req.TemplateVersion), Name: req.Name, Description: req.Description,
			Components: components, CollabMode: normalizedCollabMode(req.CollabMode), GroupConfig: groupConfig,
			RequireReport: req.RequireReport, WizardStep: normalizedWizardStep(req.WizardStep),
		})
		return updateErr
	}); err != nil {
		return ExperimentDTO{}, notFoundOrInternal(err, apperr.ErrExperimentNotFound)
	}
	return experimentDTOFromRow(row), nil
}

// UpdateExperimentStatus 更新实验发布状态。
func (r *repo) UpdateExperimentStatus(ctx context.Context, id int64, status int16) (ExperimentDTO, error) {
	var row sqlcgen.Experiment
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateExperimentStatus(ctx, sqlcgen.UpdateExperimentStatusParams{ID: id, Status: status})
		return err
	}); err != nil {
		return ExperimentDTO{}, notFoundOrInternal(err, apperr.ErrExperimentNotFound)
	}
	return experimentDTOFromRow(row), nil
}

// CreateInstance 创建实验实例初始记录。
func (r *repo) CreateInstance(ctx context.Context, id tenant.Identity, instanceID, experimentID, groupID int64, sourceRef string) (ExperimentInstanceDTO, error) {
	sandboxRefs, simRefs, err := emptyResourceRefs()
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	var row sqlcgen.ExperimentInstance
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateExperimentInstance(ctx, sqlcgen.CreateExperimentInstanceParams{
			ID: instanceID, TenantID: id.TenantID, ExperimentID: experimentID, OwnerAccountID: id.AccountID,
			GroupID: pgInt8(groupID), SourceRef: sourceRef, SandboxRefs: sandboxRefs, SimSessionRefs: simRefs, Status: InstanceStatusCreating,
		})
		return createErr
	}); err != nil {
		return ExperimentInstanceDTO{}, apperr.ErrExperimentInstanceInvalid.WithCause(err)
	}
	return experimentInstanceDTOFromRow(row), nil
}

// GetInstance 读取实验实例。
func (r *repo) GetInstance(ctx context.Context, id int64) (ExperimentInstanceDTO, error) {
	var row sqlcgen.ExperimentInstance
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetExperimentInstanceByID(ctx, id)
		return err
	}); err != nil {
		return ExperimentInstanceDTO{}, notFoundOrInternal(err, apperr.ErrExperimentInstanceNotFound)
	}
	return experimentInstanceDTOFromRow(row), nil
}

// UpdateInstanceResources 写入实例引擎资源引用并推进状态。
func (r *repo) UpdateInstanceResources(ctx context.Context, id int64, sandboxes []SandboxRef, sims []SimSessionRef, status int16) (ExperimentInstanceDTO, error) {
	sandboxRefs, err := sandboxRefsBytes(sandboxes)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	simRefs, err := simRefsBytes(sims)
	if err != nil {
		return ExperimentInstanceDTO{}, err
	}
	var row sqlcgen.ExperimentInstance
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.UpdateExperimentInstanceResources(ctx, sqlcgen.UpdateExperimentInstanceResourcesParams{ID: id, SandboxRefs: sandboxRefs, SimSessionRefs: simRefs, Status: status})
		return updateErr
	}); err != nil {
		return ExperimentInstanceDTO{}, notFoundOrInternal(err, apperr.ErrExperimentInstanceNotFound)
	}
	return experimentInstanceDTOFromRow(row), nil
}

// UpdateInstanceStatus 更新实例状态。
func (r *repo) UpdateInstanceStatus(ctx context.Context, id int64, status int16) (ExperimentInstanceDTO, error) {
	var row sqlcgen.ExperimentInstance
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateExperimentInstanceStatus(ctx, sqlcgen.UpdateExperimentInstanceStatusParams{ID: id, Status: status})
		return err
	}); err != nil {
		return ExperimentInstanceDTO{}, notFoundOrInternal(err, apperr.ErrExperimentInstanceNotFound)
	}
	return experimentInstanceDTOFromRow(row), nil
}

// UpdateInstanceScore 写入实例最终得分并置为完成态。
func (r *repo) UpdateInstanceScore(ctx context.Context, id int64, score float64) (ExperimentInstanceDTO, error) {
	numeric, err := pgNumeric(score)
	if err != nil {
		return ExperimentInstanceDTO{}, apperr.ErrExperimentScoreInvalid.WithCause(err)
	}
	var row sqlcgen.ExperimentInstance
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.UpdateExperimentInstanceScore(ctx, sqlcgen.UpdateExperimentInstanceScoreParams{ID: id, Score: numeric, Status: InstanceStatusCompleted})
		return updateErr
	}); err != nil {
		return ExperimentInstanceDTO{}, notFoundOrInternal(err, apperr.ErrExperimentInstanceNotFound)
	}
	return experimentInstanceDTOFromRow(row), nil
}

// MarkInstancesReleasedBySandbox 根据沙箱回收事件标记相关实例为环境已释放。
func (r *repo) MarkInstancesReleasedBySandbox(ctx context.Context, tenantID, sandboxID int64) ([]ExperimentInstanceDTO, error) {
	data, err := jsonx.ObjectBytes(map[string]any{"id": sandboxID}, apperr.ErrExperimentInstanceInvalid)
	if err != nil {
		return nil, err
	}
	var rows []sqlcgen.ExperimentInstance
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var updateErr error
		rows, updateErr = q.MarkInstancesReleasedBySandbox(ctx, sqlcgen.MarkInstancesReleasedBySandboxParams{
			TenantID: tenantID, SandboxRefJson: []byte("[" + string(data) + "]"), Status: InstanceStatusReleased,
		})
		return updateErr
	}); err != nil {
		return nil, apperr.ErrExperimentInstanceInvalid.WithCause(err)
	}
	out := make([]ExperimentInstanceDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, experimentInstanceDTOFromRow(row))
	}
	return out, nil
}

// UpsertCheckpointResult 新增或更新检查点结果。
func (r *repo) UpsertCheckpointResult(ctx context.Context, result CheckpointResultDTO) (CheckpointResultDTO, error) {
	score, err := pgNumeric(result.Score)
	if err != nil {
		return CheckpointResultDTO{}, apperr.ErrCheckpointResultInvalid.WithCause(err)
	}
	var row sqlcgen.CheckpointResult
	if err := r.inTenantID(ctx, result.TenantID, func(q *sqlcgen.Queries) error {
		var upsertErr error
		row, upsertErr = q.UpsertCheckpointResult(ctx, sqlcgen.UpsertCheckpointResultParams{
			ID: ids.ParseOrZero(result.ID), TenantID: result.TenantID, InstanceID: result.InstanceID, CheckpointID: result.CheckpointID,
			JudgeTaskRef: pgText(result.JudgeTaskRef), Passed: result.Passed, Score: score, DetailRef: pgText(result.DetailRef),
		})
		return upsertErr
	}); err != nil {
		return CheckpointResultDTO{}, apperr.ErrCheckpointResultInvalid.WithCause(err)
	}
	return checkpointResultDTOFromRow(row), nil
}

// PendingCheckpointByJudgeTask 根据判题任务 ID 定位等待回写的检查点。
func (r *repo) PendingCheckpointByJudgeTask(ctx context.Context, tenantID, taskID int64) (PendingCheckpoint, error) {
	var row sqlcgen.GetCheckpointResultByJudgeTaskRow
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetCheckpointResultByJudgeTask(ctx, pgText(ids.Format(taskID)))
		return err
	}); err != nil {
		return PendingCheckpoint{}, notFoundOrInternal(err, apperr.ErrCheckpointResultNotFound)
	}
	return PendingCheckpoint{TenantID: row.TenantID, InstanceID: row.InstanceID, CheckpointID: row.CheckpointID, SourceRef: row.SourceRef}, nil
}

// ListCheckpointScores 读取实例所有检查点得分。
func (r *repo) ListCheckpointScores(ctx context.Context, instanceID int64) ([]ScorePart, error) {
	var rows []sqlcgen.CheckpointResult
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListCheckpointResultsByInstance(ctx, instanceID)
		return err
	}); err != nil {
		return nil, apperr.ErrCheckpointResultInvalid.WithCause(err)
	}
	out := make([]ScorePart, 0, len(rows))
	for _, row := range rows {
		out = append(out, ScorePart{Score: numericValue(row.Score)})
	}
	return out, nil
}

// LatestReportScore 返回实例最新一份已批改报告分。
func (r *repo) LatestReportScore(ctx context.Context, instanceID int64) (*float64, error) {
	var rows []sqlcgen.ExperimentReport
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListReportsByInstance(ctx, instanceID)
		return err
	}); err != nil {
		return nil, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	for _, row := range rows {
		if row.Status == ReportStatusGraded {
			return numericPtr(row.ManualScore), nil
		}
	}
	return nil, nil
}

// CreateReport 提交或覆盖当前学生的实验报告。
func (r *repo) CreateReport(ctx context.Context, id tenant.Identity, reportID, instanceID int64, contentRef string) (ReportDTO, error) {
	var row sqlcgen.ExperimentReport
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateExperimentReport(ctx, sqlcgen.CreateExperimentReportParams{ID: reportID, TenantID: id.TenantID, InstanceID: instanceID, StudentID: id.AccountID, ContentRef: contentRef, Status: ReportStatusSubmitted})
		return err
	}); err != nil {
		return ReportDTO{}, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	return reportDTOFromRow(row), nil
}

// ListReports 查询某实验下的报告列表。
func (r *repo) ListReports(ctx context.Context, experimentID int64, page, size int) ([]ReportDTO, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.ExperimentReport
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListReportsByExperiment(ctx, sqlcgen.ListReportsByExperimentParams{ExperimentID: experimentID, OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
		return err
	}); err != nil {
		return nil, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	out := make([]ReportDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, reportDTOFromRow(row))
	}
	return out, nil
}

// GradeReportAuthorized 按实验作者/学校管理员/平台上下文原子批改实验报告。
func (r *repo) GradeReportAuthorized(ctx context.Context, id tenant.Identity, isSchoolAdmin bool, reportID int64, score float64, comment string) (ReportDTO, error) {
	numeric, err := pgNumeric(score)
	if err != nil {
		return ReportDTO{}, apperr.ErrExperimentReportInvalid.WithCause(err)
	}
	var row sqlcgen.GradeExperimentReportAuthorizedRow
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.GradeExperimentReportAuthorized(ctx, sqlcgen.GradeExperimentReportAuthorizedParams{
			ID: reportID, IsPlatform: id.IsPlatform, IsSchoolAdmin: isSchoolAdmin, ActorID: id.AccountID,
			ManualScore: numeric, Comment: pgText(comment), Status: ReportStatusGraded,
		})
		return updateErr
	}); err != nil {
		return ReportDTO{}, notFoundOrInternal(err, apperr.ErrExperimentReportNotFound)
	}
	if !row.Authorized {
		return ReportDTO{}, apperr.ErrExperimentForbidden
	}
	return reportDTOFromAuthorizedRow(row), nil
}

// CreateGroup 创建实验协作小组。
func (r *repo) CreateGroup(ctx context.Context, id tenant.Identity, groupID, experimentID int64, name string) (GroupDTO, error) {
	var row sqlcgen.ExperimentGroup
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateExperimentGroup(ctx, sqlcgen.CreateExperimentGroupParams{ID: groupID, TenantID: id.TenantID, ExperimentID: experimentID, Name: name})
		return err
	}); err != nil {
		return GroupDTO{}, apperr.ErrExperimentGroupInvalid.WithCause(err)
	}
	return groupDTOFromRows(row, nil), nil
}

// AddGroupMemberAuthorized 按实验作者/学校管理员/平台上下文原子更新协作小组成员角色。
func (r *repo) AddGroupMemberAuthorized(ctx context.Context, id tenant.Identity, isSchoolAdmin bool, memberID, groupID, studentID int64, role string) (GroupMemberDTO, error) {
	var row sqlcgen.AddGroupMemberAuthorizedRow
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.AddGroupMemberAuthorized(ctx, sqlcgen.AddGroupMemberAuthorizedParams{
			GroupID: groupID, IsPlatform: id.IsPlatform, IsSchoolAdmin: isSchoolAdmin, ActorID: id.AccountID,
			ID: memberID, TenantID: id.TenantID, StudentID: studentID, Role: role,
		})
		return err
	}); err != nil {
		return GroupMemberDTO{}, notFoundOrInternal(err, apperr.ErrExperimentGroupNotFound)
	}
	if !row.Authorized {
		return GroupMemberDTO{}, apperr.ErrExperimentForbidden
	}
	return groupMemberDTOFromAuthorizedRow(row), nil
}

// GetGroup 读取小组及成员。
func (r *repo) GetGroup(ctx context.Context, id int64) (GroupDTO, error) {
	var group sqlcgen.ExperimentGroup
	var members []sqlcgen.GroupMember
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		group, err = q.GetExperimentGroupByID(ctx, id)
		if err != nil {
			return err
		}
		members, err = q.ListGroupMembers(ctx, id)
		return err
	}); err != nil {
		return GroupDTO{}, notFoundOrInternal(err, apperr.ErrExperimentGroupNotFound)
	}
	return groupDTOFromRows(group, members), nil
}

// GetGroupForExperiment 读取指定实验下的小组及成员,避免跨实验 group_id 混用。
func (r *repo) GetGroupForExperiment(ctx context.Context, groupID, experimentID int64) (GroupDTO, error) {
	var group sqlcgen.ExperimentGroup
	var members []sqlcgen.GroupMember
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		group, err = q.GetExperimentGroupByIDAndExperiment(ctx, sqlcgen.GetExperimentGroupByIDAndExperimentParams{ID: groupID, ExperimentID: experimentID})
		if err != nil {
			return err
		}
		members, err = q.ListGroupMembers(ctx, groupID)
		return err
	}); err != nil {
		return GroupDTO{}, notFoundOrInternal(err, apperr.ErrExperimentGroupNotFound)
	}
	return groupDTOFromRows(group, members), nil
}

// Stats 返回实验模块内部统计。
func (r *repo) Stats(ctx context.Context, tenantID, courseID int64) (StatsDTO, error) {
	var out StatsDTO
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		out.ExperimentCount, err = q.CountExperiments(ctx, pgInt8Filter(courseID))
		if err != nil {
			return err
		}
		out.ActiveInstanceCount, err = q.CountActiveInstances(ctx, pgInt8Filter(courseID))
		return err
	}); err != nil {
		return StatsDTO{}, apperr.ErrExperimentQueryFailed.WithCause(err)
	}
	out.TenantID = ids.Format(tenantID)
	if courseID > 0 {
		out.CourseID = ids.Format(courseID)
	}
	return out, nil
}

// experimentRequestJSON 序列化实验请求中的 JSONB 字段。
func experimentRequestJSON(req ExperimentRequest) ([]byte, []byte, error) {
	components, err := componentsBytes(req.Components)
	if err != nil {
		return nil, nil, err
	}
	groupConfig, err := jsonx.ObjectBytes(req.GroupConfig, apperr.ErrExperimentInvalid)
	if err != nil {
		return nil, nil, err
	}
	return components, groupConfig, nil
}

// emptyResourceRefs 返回空资源引用 JSON。
func emptyResourceRefs() ([]byte, []byte, error) {
	sandboxRefs, err := sandboxRefsBytes(nil)
	if err != nil {
		return nil, nil, err
	}
	simRefs, err := simRefsBytes(nil)
	if err != nil {
		return nil, nil, err
	}
	return sandboxRefs, simRefs, nil
}

// normalizedCollabMode 为未传协作模式的请求提供单人默认值。
func normalizedCollabMode(v int16) int16 {
	if v == 0 {
		return CollabModeSingle
	}
	return v
}

// normalizedWizardStep 为未传向导步骤的请求提供第一步默认值。
func normalizedWizardStep(v int16) int16 {
	if v == 0 {
		return 1
	}
	return v
}

// pgInt8FromString 把字符串 ID 转为可空 int8。
func pgInt8FromString(v string) pgtype.Int8 {
	id, _ := ids.Parse(v)
	return pgInt8(id)
}

// notFoundOrInternal 把 pgx 未命中转换为模块错误码。
func notFoundOrInternal(err error, notFound *apperr.Error) error {
	if db.IsNoRows(err) {
		return notFound.WithCause(err)
	}
	return apperr.ErrExperimentQueryFailed.WithCause(err)
}
