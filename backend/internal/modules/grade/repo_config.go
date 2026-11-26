// 访问成绩等级、预警规则与学期配置。
package grade

import (
	"context"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// CreateLevelConfig 创建等级映射配置。
func (t *txStore) CreateLevelConfig(ctx context.Context, id, tenantID int64, req LevelConfigRequest) (LevelConfigDTO, error) {
	mapping, err := jsonx.AnyBytes(req.Mapping, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	warnings, err := jsonx.AnyBytes(req.WarningRules, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	row, err := t.q.CreateLevelConfig(ctx, sqlcgen.CreateLevelConfigParams{ID: id, TenantID: tenantID, Name: req.Name, Mapping: mapping, WarningRules: warnings, IsDefault: req.IsDefault})
	if err != nil {
		return LevelConfigDTO{}, err
	}
	return levelConfigDTO(row)
}

// ListLevelConfigs 查询等级映射配置。
func (t *txStore) ListLevelConfigs(ctx context.Context) ([]LevelConfigDTO, error) {
	rows, err := t.q.ListLevelConfigs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LevelConfigDTO, 0, len(rows))
	for _, row := range rows {
		item, err := levelConfigDTO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// GetDefaultLevelConfig 查询默认等级映射配置。
func (t *txStore) GetDefaultLevelConfig(ctx context.Context) (LevelConfigDTO, error) {
	row, err := t.q.GetDefaultLevelConfig(ctx)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	return levelConfigDTO(row)
}

// LockGradeLevelDefaultScope 串行化当前租户默认等级切换。
func (t *txStore) LockGradeLevelDefaultScope(ctx context.Context, lockKey int64) error {
	return t.q.LockGradeLevelDefaultScope(ctx, lockKey)
}

// ClearDefaultLevelConfigs 清除当前租户既有默认等级标记。
func (t *txStore) ClearDefaultLevelConfigs(ctx context.Context) error {
	return t.q.ClearDefaultLevelConfigs(ctx)
}

// UpdateLevelConfig 更新等级映射配置。
func (t *txStore) UpdateLevelConfig(ctx context.Context, id int64, req LevelConfigRequest) (LevelConfigDTO, error) {
	mapping, err := jsonx.AnyBytes(req.Mapping, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	warnings, err := jsonx.AnyBytes(req.WarningRules, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	row, err := t.q.UpdateLevelConfig(ctx, sqlcgen.UpdateLevelConfigParams{ID: id, Name: req.Name, Mapping: mapping, WarningRules: warnings, IsDefault: req.IsDefault})
	if err != nil {
		return LevelConfigDTO{}, err
	}
	return levelConfigDTO(row)
}

// UpdateDefaultWarningRules 只更新默认配置的预警字段，保留其余配置。
func (t *txStore) UpdateDefaultWarningRules(ctx context.Context, rules WarningRules) (WarningRules, error) {
	warnings, err := jsonx.AnyBytes(rules, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return WarningRules{}, err
	}
	row, err := t.q.UpdateDefaultWarningRules(ctx, warnings)
	if err != nil {
		return WarningRules{}, err
	}
	var out WarningRules
	if err := jsonx.DecodeStrictKnownFields(row, &out); err != nil {
		return WarningRules{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	return out, nil
}

// CreateSemester 创建学期。
func (t *txStore) CreateSemester(ctx context.Context, id, tenantID int64, req SemesterRequest) (SemesterDTO, error) {
	start, err := timex.ParseDate(req.StartDate)
	if err != nil {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	end, err := timex.ParseDate(req.EndDate)
	if err != nil {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	row, err := t.q.CreateSemester(ctx, sqlcgen.CreateSemesterParams{ID: id, TenantID: tenantID, Name: req.Name, StartDate: pgtypex.Date(start), EndDate: pgtypex.Date(end), IsCurrent: req.IsCurrent})
	if err != nil {
		return SemesterDTO{}, err
	}
	return semesterDTO(row), nil
}

// ListSemesters 查询学期列表。
func (t *txStore) ListSemesters(ctx context.Context) ([]SemesterDTO, error) {
	rows, err := t.q.ListSemesters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SemesterDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, semesterDTO(row))
	}
	return out, nil
}

// LockSemesterCurrentScope 串行化租户内学期区间校验、创建与当前标记切换。
func (t *txStore) LockSemesterCurrentScope(ctx context.Context, lockKey int64) error {
	return t.q.LockSemesterCurrentScope(ctx, lockKey)
}

// ClearCurrentSemesters 清除当前租户既有当前学期标记。
func (t *txStore) ClearCurrentSemesters(ctx context.Context) error {
	return t.q.ClearCurrentSemesters(ctx)
}

// GetCurrentSemester 查询当前学期。
func (t *txStore) GetCurrentSemester(ctx context.Context) (SemesterDTO, error) {
	row, err := t.q.GetCurrentSemester(ctx)
	if err != nil {
		return SemesterDTO{}, err
	}
	return semesterDTO(row), nil
}
