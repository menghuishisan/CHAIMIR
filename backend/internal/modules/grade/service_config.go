// 管理成绩等级映射、预警规则与学校学期。
package grade

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// CreateLevelConfig 创建等级映射配置。
func (s *Service) CreateLevelConfig(ctx context.Context, req LevelConfigRequest) (LevelConfigDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	if err := validateLevelConfig(req); err != nil {
		return LevelConfigDTO{}, err
	}
	var out LevelConfigDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if req.IsDefault {
			if err := tx.LockGradeLevelDefaultScope(ctx, gradeConfigLockKey(id.TenantID, 1)); err != nil {
				return err
			}
			if err := tx.ClearDefaultLevelConfigs(ctx); err != nil {
				return err
			}
		}
		var err error
		out, err = tx.CreateLevelConfig(ctx, s.ids.Generate(), id.TenantID, req)
		return err
	})
	return out, mapGradeConfigErr(err)
}

// ListLevelConfigs 查询等级映射配置。
func (s *Service) ListLevelConfigs(ctx context.Context) ([]LevelConfigDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return nil, err
	}
	var out []LevelConfigDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListLevelConfigs(ctx)
		return err
	})
	return out, mapGradeConfigErr(err)
}

// UpdateLevelConfig 更新等级映射配置。
func (s *Service) UpdateLevelConfig(ctx context.Context, configID int64, req LevelConfigRequest) (LevelConfigDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	if err := validateLevelConfig(req); err != nil {
		return LevelConfigDTO{}, err
	}
	var out LevelConfigDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if req.IsDefault {
			if err := tx.LockGradeLevelDefaultScope(ctx, gradeConfigLockKey(id.TenantID, 1)); err != nil {
				return err
			}
			if err := tx.ClearDefaultLevelConfigs(ctx); err != nil {
				return err
			}
		}
		var err error
		out, err = tx.UpdateLevelConfig(ctx, configID, req)
		return err
	})
	return out, mapGradeConfigErr(err)
}

func gradeConfigLockKey(tenantID int64, scope int64) int64 {
	return tenantID*10 + scope
}

// CreateSemester 创建学期。
func (s *Service) CreateSemester(ctx context.Context, req SemesterRequest) (SemesterDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return SemesterDTO{}, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid
	}
	start, err := timex.ParseDate(req.StartDate)
	if err != nil {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	end, err := timex.ParseDate(req.EndDate)
	if err != nil {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	if start.After(end) {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid
	}
	var out SemesterDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		// 所有学期创建共用事务锁,避免两个非当前学期同时越过区间校验。
		if err := tx.LockSemesterCurrentScope(ctx, gradeConfigLockKey(id.TenantID, 2)); err != nil {
			return err
		}
		semesters, err := tx.ListSemesters(ctx)
		if err != nil {
			return err
		}
		for _, semester := range semesters {
			existingStart, err := timex.ParseDate(semester.StartDate)
			if err != nil {
				return fmt.Errorf("parse existing semester %d start: %w", semester.ID.Int64(), err)
			}
			existingEnd, err := timex.ParseDate(semester.EndDate)
			if err != nil {
				return fmt.Errorf("parse existing semester %d end: %w", semester.ID.Int64(), err)
			}
			if !start.After(existingEnd) && !end.Before(existingStart) {
				return apperr.ErrGradeConfigInvalid
			}
		}
		if req.IsCurrent {
			if err := tx.ClearCurrentSemesters(ctx); err != nil {
				return err
			}
		}
		out, err = tx.CreateSemester(ctx, s.ids.Generate(), id.TenantID, req)
		return err
	})
	return out, mapGradeConfigErr(err)
}

// ListSemesters 查询学期列表。
func (s *Service) ListSemesters(ctx context.Context) ([]SemesterDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return nil, err
	}
	var out []SemesterDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListSemesters(ctx)
		return err
	})
	return out, mapGradeConfigErr(err)
}

// GetWarningRules 读取默认等级配置中的学业预警规则。
func (s *Service) GetWarningRules(ctx context.Context) (WarningRules, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return WarningRules{}, err
	}
	cfg, err := s.defaultConfig(ctx, id.TenantID)
	if err != nil {
		return WarningRules{}, err
	}
	return cfg.WarningRules, nil
}

// UpdateWarningRules 更新默认等级配置中的学业预警规则。
func (s *Service) UpdateWarningRules(ctx context.Context, req WarningRulesRequest) (WarningRules, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return WarningRules{}, err
	}
	rules, err := warningRulesFromRequest(req)
	if err != nil {
		return WarningRules{}, err
	}
	if err := validateWarningRules(rules); err != nil {
		return WarningRules{}, err
	}
	var out WarningRules
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockGradeLevelDefaultScope(ctx, gradeConfigLockKey(id.TenantID, 1)); err != nil {
			return err
		}
		var err error
		out, err = tx.UpdateDefaultWarningRules(ctx, rules)
		return err
	})
	if err != nil {
		return WarningRules{}, mapGradeConfigErr(err)
	}
	return out, nil
}

// warningRulesFromRequest 把必填更新请求转换为持久化模型,保留显式零值语义。
func warningRulesFromRequest(req WarningRulesRequest) (WarningRules, error) {
	if req.FailCount == nil || req.MinGPA == nil {
		return WarningRules{}, apperr.ErrGradeConfigInvalid
	}
	return WarningRules{FailCount: *req.FailCount, MinGPA: *req.MinGPA}, nil
}

// defaultConfig 读取租户默认等级映射配置。
func (s *Service) defaultConfig(ctx context.Context, tenantID int64) (LevelConfigDTO, error) {
	var cfg LevelConfigDTO
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		cfg, err = tx.GetDefaultLevelConfig(ctx)
		return err
	})
	if err != nil {
		return LevelConfigDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	return cfg, nil
}

// validateLevelConfig 校验等级配置可用于 GPA 计算。
func validateLevelConfig(req LevelConfigRequest) error {
	if strings.TrimSpace(req.Name) == "" || len(req.Mapping) == 0 {
		return apperr.ErrGradeConfigInvalid
	}
	if err := validateWarningRules(req.WarningRules); err != nil {
		return err
	}
	_, _, err := ComputeGPA([]CourseGradeInput{{FinalTotal: 100, Credits: 1}}, req.Mapping)
	return err
}

// validateWarningRules 校验学业预警阈值不会产生无意义配置。
func validateWarningRules(rules WarningRules) error {
	if rules.FailCount < 0 || rules.MinGPA < 0 || rules.MinGPA > 4 {
		return apperr.ErrGradeConfigInvalid
	}
	return nil
}

// mapGradeConfigErr 统一转换配置类错误码。
func mapGradeConfigErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.ErrGradeConfigInvalid.WithCause(err)
}
