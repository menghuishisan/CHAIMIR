// 缓存已发布成绩摘要，并按完整发布集合的摘要失效全部学期。
package grade

import (
	"context"
	"log/slog"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// gradeAggregateCacheKey 隔离不同发布范围的摘要。
func (s *Service) gradeAggregateCacheKey(tenantID, studentID, semesterID int64, fingerprint string) string {
	return "tenant:" + ids.Format(tenantID) + ":grade:aggregate:" + ids.Format(studentID) + ":" + ids.Format(semesterID) + ":" + fingerprint
}

// readGradeAggregateCache 读取浏览器成绩页的标量缓存,不会反序列化课程明细。
func (s *Service) readGradeAggregateCache(ctx context.Context, key string) (StudentGradeAggregateDTO, bool) {
	data, ok, err := s.cache.GetBytes(ctx, key)
	if err != nil {
		logging.ErrorContext(ctx, "grade aggregate cache read failed", err.Error(), slog.String("cache_key", key))
		return StudentGradeAggregateDTO{}, false
	}
	if !ok {
		return StudentGradeAggregateDTO{}, false
	}
	var out StudentGradeAggregateDTO
	if err := jsonx.DecodeStrictKnownFields(data, &out); err != nil {
		logging.ErrorContext(ctx, "grade aggregate cache decode failed", err.Error(), slog.String("cache_key", key))
		if delErr := s.cache.Delete(ctx, key); delErr != nil {
			logging.ErrorContext(ctx, "grade aggregate cache delete corrupt value failed", delErr.Error(), slog.String("cache_key", key))
		}
		return StudentGradeAggregateDTO{}, false
	}
	return out, true
}

// writeGradeAggregateCache 写入浏览器成绩页标量摘要,禁止把课程明细写入 Redis。
func (s *Service) writeGradeAggregateCache(ctx context.Context, key string, aggregate StudentGradeAggregateDTO) {
	data, err := jsonx.AnyBytes(aggregate, apperr.ErrGradeAggregationFailed)
	if err != nil {
		logging.ErrorContext(ctx, "grade aggregate cache encode failed", err.Error(), slog.String("cache_key", key))
		return
	}
	if err := s.cache.SetBytes(ctx, key, data, time.Duration(s.cfg.GradeAggregateCacheTTLSeconds)*time.Second); err != nil {
		logging.ErrorContext(ctx, "grade aggregate cache write failed", err.Error(), slog.String("cache_key", key))
	}
}

// invalidateGradeAggregateCache 失效当前发布修订的全部学期与全量摘要。
func (s *Service) invalidateGradeAggregateCache(ctx context.Context, tenantID, studentID int64) {
	var semesters []SemesterDTO
	var fingerprint string
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		publications, err := tx.ListGradePublications(ctx)
		if err != nil {
			return err
		}
		_, fingerprint = publicationScope(publications)
		semesters, err = tx.ListSemesters(ctx)
		return err
	})
	if err != nil {
		logging.ErrorContext(ctx, "grade cache scope read failed", err.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("student_id", studentID))
		return
	}
	keys := []string{s.gradeAggregateCacheKey(tenantID, studentID, 0, fingerprint)}
	for _, semester := range semesters {
		keys = append(keys, s.gradeAggregateCacheKey(tenantID, studentID, semester.ID.Int64(), fingerprint))
	}
	for _, key := range keys {
		if err := s.cache.Delete(ctx, key); err != nil {
			logging.ErrorContext(ctx, "grade aggregate cache invalidate failed", err.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("student_id", studentID), slog.String("cache_key", key))
		}
	}
}

// invalidateCourseAggregateCache 通过 M6 课程成绩契约定位学生,再逐个失效 GPA 标量缓存。
func (s *Service) invalidateCourseAggregateCache(ctx context.Context, tenantID, courseID int64) {
	grades, err := s.teaching.ListCourseGrades(ctx, tenantID, courseID)
	if err != nil {
		logging.ErrorContext(ctx, "grade course aggregate cache source read failed", err.Error(), slog.Int64("tenant_id", tenantID), slog.Int64("course_id", courseID))
		return
	}
	seen := map[int64]struct{}{}
	for _, row := range grades {
		if _, ok := seen[row.StudentID]; ok {
			continue
		}
		seen[row.StudentID] = struct{}{}
		s.invalidateGradeAggregateCache(ctx, tenantID, row.StudentID)
	}
}
