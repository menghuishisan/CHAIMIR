// 查询、确认与扫描学生学业预警。
package grade

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
)

// ListWarnings 查询学业预警分页列表;status 传 0 表示不按确认状态过滤。
func (s *Service) ListWarnings(ctx context.Context, studentID int64, status int16, semesterID int64, page, size int) ([]WarningDTO, int64, int, int, pagex.Facets, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	page, size = pagex.Normalize(page, size)
	isAdmin, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return nil, 0, page, size, nil, apperr.ErrGradeForbidden.WithCause(err)
	}
	if !isAdmin {
		isStudent, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleStudent)
		if err != nil {
			return nil, 0, page, size, nil, apperr.ErrGradeForbidden.WithCause(err)
		}
		// 学生缺省为本人;校管保留 0 作为本租户全校查询范围。
		if studentID == 0 {
			studentID = id.AccountID
		}
		if !isStudent || studentID != id.AccountID {
			return nil, 0, page, size, nil, apperr.ErrGradeForbidden
		}
	}
	if studentID < 0 {
		return nil, 0, page, size, nil, apperr.ErrGradeForbidden
	}
	var out []WarningDTO
	var total int64
	var byStatus pagex.FacetGroup
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, total, err = tx.ListAcademicWarnings(ctx, studentID, status, semesterID, page, size)
		if err != nil {
			return err
		}
		byStatus, err = tx.CountAcademicWarningsByStatus(ctx, studentID, semesterID)
		return err
	})
	if err == nil {
		out, err = s.withWarningNames(ctx, id.TenantID, out)
	}
	return out, total, page, size, pagex.Facets{"status": byStatus}, mapGradeWarningErr(err)
}

// AckWarning 确认本人学业预警。
func (s *Service) AckWarning(ctx context.Context, warningID int64) (WarningDTO, error) {
	id, err := s.requireStudent(ctx)
	if err != nil {
		return WarningDTO{}, err
	}
	var out WarningDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.AckAcademicWarning(ctx, warningID, id.AccountID)
		return err
	})
	return out, mapGradeWarningErr(err)
}

// ScanWarnings 基于已有学生学期 GPA 聚合记录重算并复核预警。
func (s *Service) ScanWarnings(ctx context.Context, req WarningScanRequest) (WarningScanResultDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return WarningScanResultDTO{}, err
	}
	if req.StudentID < 0 || req.SemesterID < 0 {
		return WarningScanResultDTO{}, apperr.ErrGradeWarningInvalid
	}
	var students []int64
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if req.SemesterID == 0 {
			semester, err := tx.GetCurrentSemester(ctx)
			if err != nil {
				if isNoRows(err) {
					return apperr.New(apperr.CodeGradeConfigInvalid, "学校尚未设置当前学期,请先在成绩配置中设置").WithCause(err)
				}
				return apperr.ErrGradeConfigInvalid.WithCause(err)
			}
			req.SemesterID = semester.ID
		} else {
			semesters, err := tx.ListSemesters(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, semester := range semesters {
				found = found || semester.ID == req.SemesterID
			}
			if !found {
				return apperr.ErrGradeConfigInvalid
			}
		}
		var err error
		students, err = tx.ListGradeWarningStudents(ctx, req.StudentID.Int64(), req.SemesterID.Int64())
		return err
	})
	if err != nil {
		return WarningScanResultDTO{}, mapGradeWarningErr(err)
	}
	out := WarningScanResultDTO{}
	for _, studentID := range students {
		created, err := s.recomputeStudentWarnings(ctx, id.TenantID, studentID, req.SemesterID.Int64())
		if err != nil {
			return WarningScanResultDTO{}, err
		}
		out.Scanned++
		out.Created += created
	}
	return out, nil
}

// createWarnings 根据规则保存预警及其待投递状态,后台任务在事务提交后可靠发布通知。
// 是否挂科以租户等级映射的不及格档(绩点为 0)为唯一判定,不使用硬编码及格线。
func (s *Service) createWarnings(ctx context.Context, tx TxStore, tenantID, studentID, semesterID int64, grades []CourseGradeInput, gpa float64, rules WarningRules, mapping []LevelRule) (int, error) {
	failCount := 0
	ordered := orderedMapping(mapping)
	for _, row := range grades {
		if point, ok := gpaPointForScore(row.FinalTotal, ordered); ok && point == 0 {
			failCount++
		}
	}
	created := 0
	if rules.FailCount > 0 && failCount >= rules.FailCount {
		id := s.ids.Generate()
		row, err := tx.CreateAcademicWarning(ctx, id, tenantID, studentID, semesterID, WarningTypeFailedCourse, map[string]any{"failed_count": failCount, "gpa": gpa})
		if err != nil {
			return 0, apperr.ErrGradeWarningInvalid.WithCause(err)
		}
		if row.ID.Int64() == id {
			created++
		}
	}
	if rules.MinGPA > 0 && gpa < rules.MinGPA {
		id := s.ids.Generate()
		row, err := tx.CreateAcademicWarning(ctx, id, tenantID, studentID, semesterID, WarningTypeLowGPA, map[string]any{"gpa": gpa})
		if err != nil {
			return 0, apperr.ErrGradeWarningInvalid.WithCause(err)
		}
		if row.ID.Int64() == id {
			created++
		}
	}
	return created, nil
}

// mapGradeWarningErr 统一转换预警类错误码。
func mapGradeWarningErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.ErrGradeWarningInvalid.WithCause(err)
}
