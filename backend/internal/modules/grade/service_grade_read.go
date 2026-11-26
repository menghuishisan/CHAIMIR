// 按 M11 发布权威流式读取 M6 成绩，统一分页、GPA 和成绩单口径。
package grade

import (
	"context"
	"encoding/binary"
	"sort"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
)

// publicationScope 对完整发布集合取摘要，避免较早修订晚提交时命中旧范围。
func publicationScope(rows []gradePublication) (map[int64]int64, string) {
	published := make(map[int64]int64, len(rows))
	ordered := append([]gradePublication(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CourseID < ordered[j].CourseID })
	var scope []byte
	for _, row := range ordered {
		scope = binary.AppendVarint(scope, row.CourseID)
		scope = binary.AppendVarint(scope, row.SemesterID)
		scope = binary.AppendVarint(scope, row.Revision)
		if row.Published {
			scope = append(scope, 1)
			published[row.CourseID] = row.SemesterID
		} else {
			scope = append(scope, 0)
		}
	}
	return published, pkgcrypto.SHA256Hex(scope)
}

// walkPublishedGrades 先筛发布范围再交给调用方，不按 M6 页码切公开页面。
func (s *Service) walkPublishedGrades(ctx context.Context, tenantID, studentID int64, published map[int64]int64, visit func(contracts.TeachingCourseGrade, int64) error) error {
	if len(published) == 0 {
		return nil
	}
	for page := 1; ; page++ {
		rows, _, err := s.teaching.ListStudentGrades(ctx, tenantID, studentID, "", page, pagex.MaximumSize())
		if err != nil {
			return apperr.ErrGradeAggregationFailed.WithCause(err)
		}
		for _, row := range rows {
			semesterID, ok := published[row.CourseID]
			if !ok {
				continue
			}
			if err := visit(row, semesterID); err != nil {
				return err
			}
		}
		if len(rows) < pagex.MaximumSize() {
			return nil
		}
	}
}

// readStudentGrades 的 size=0 仅供成绩单完整读取，并跳过摘要缓存。
func (s *Service) readStudentGrades(ctx context.Context, tenantID, studentID, semesterID int64, page, size int) (StudentGradePageDTO, error) {
	out := StudentGradePageDTO{StudentID: ids.ID(studentID), SemesterID: ids.ID(semesterID), Page: page, Size: size, List: make([]CourseGradeInput, 0)}
	aggregate := StudentGradeAggregateDTO{ComputedAt: timex.Now()}
	var cacheKey string
	var cacheHit bool
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		if semesterID > 0 {
			semesters, err := tx.ListSemesters(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, semester := range semesters {
				found = found || semester.ID.Int64() == semesterID
			}
			if !found {
				return apperr.ErrGradeConfigInvalid
			}
		}
		publications, err := tx.ListGradePublications(ctx)
		if err != nil {
			return apperr.ErrGradeAggregationFailed.WithCause(err)
		}
		published, fingerprint := publicationScope(publications)
		cacheKey = s.gradeAggregateCacheKey(tenantID, studentID, semesterID, fingerprint)
		if size > 0 {
			if cached, ok := s.readGradeAggregateCache(ctx, cacheKey); ok {
				aggregate, cacheHit = cached, true
			}
		}
		var mapping []LevelRule
		var current, cumulative gpaTotals
		err = s.walkPublishedGrades(ctx, tenantID, studentID, published, func(row contracts.TeachingCourseGrade, rowSemesterID int64) error {
			inScope := semesterID == 0 || rowSemesterID == semesterID
			if !cacheHit {
				if mapping == nil {
					cfg, err := tx.GetDefaultLevelConfig(ctx)
					if err != nil {
						return apperr.ErrGradeConfigInvalid.WithCause(err)
					}
					if err := validateGPAMapping(cfg.Mapping); err != nil {
						return err
					}
					mapping = orderedMapping(cfg.Mapping)
				}
				input := CourseGradeInput{FinalTotal: row.FinalTotal, Credits: row.Credits}
				if err := cumulative.add(input, mapping); err != nil {
					return err
				}
				if inScope {
					if err := current.add(input, mapping); err != nil {
						return err
					}
				}
			}
			if inScope {
				index := out.Total
				out.Total++
				if size == 0 || (index >= int64(page-1)*int64(size) && len(out.List) < size) {
					out.List = append(out.List, CourseGradeInput{CourseID: ids.ID(row.CourseID), StudentID: ids.ID(row.StudentID), FinalTotal: row.FinalTotal, Credits: row.Credits})
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !cacheHit {
			aggregate = StudentGradeAggregateDTO{StudentID: out.StudentID, SemesterID: out.SemesterID, GPA: current.gpa(), TotalCredits: current.credits, CumulativeGPA: cumulative.gpa(), Total: out.Total, ComputedAt: timex.Now()}
		}
		return nil
	})
	if err != nil {
		return StudentGradePageDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	if size > 0 && !cacheHit {
		s.writeGradeAggregateCache(ctx, cacheKey, aggregate)
	}
	out.GPA, out.TotalCredits, out.CumulativeGPA, out.ComputedAt = aggregate.GPA, aggregate.TotalCredits, aggregate.CumulativeGPA, aggregate.ComputedAt
	return out, nil
}
