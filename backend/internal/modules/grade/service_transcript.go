// 生成成绩单元数据并签发统一文件下载授权。
package grade

import (
	"bytes"
	"context"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

const (
	gradeModuleName             = "grade"
	gradeTranscriptResourceType = "transcript"
	transcriptPDFContentType    = "application/pdf"
)

// GenerateTranscript 生成成绩单 PDF 并保存元数据。
func (s *Service) GenerateTranscript(ctx context.Context, req TranscriptRequest) (TranscriptDTO, error) {
	id, err := s.normalizeStudentOrSchoolAdmin(ctx, req.StudentID.Int64())
	if err != nil {
		return TranscriptDTO{}, err
	}
	actorRole, err := s.gradeActorRole(ctx, id.AccountID)
	if err != nil {
		return TranscriptDTO{}, err
	}
	if req.Scope != TranscriptScopeSemester && req.Scope != TranscriptScopeFull {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed
	}
	if req.Scope == TranscriptScopeSemester && req.SemesterID <= 0 {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed
	}
	summary, err := s.transcriptSummary(ctx, req.StudentID.Int64(), req.Scope, req.SemesterID.Int64())
	if err != nil {
		return TranscriptDTO{}, err
	}
	pdf, err := renderTranscriptPDF(summary, s.cfg.TranscriptSigningKey)
	if err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	fileName := ids.Format(s.ids.Generate()) + ".pdf"
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:        id.TenantID,
		AccountID:       id.AccountID,
		Module:          gradeModuleName,
		ResourceType:    gradeTranscriptResourceType,
		ResourceID:      summary.StudentID.String(),
		FileName:        fileName,
		ContentType:     transcriptPDFContentType,
		Size:            int64(len(pdf)),
		MaxBytes:        s.cfg.TranscriptMaxBytes,
		ExpectedBucket:  s.storage.BucketReport(),
		AllowedFileName: true,
		Content:         pdf,
	})
	if err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(pdf), int64(len(pdf)), transcriptPDFContentType); err != nil {
		err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, false, func(ctx context.Context) error { return s.storage.Delete(ctx, plan.Bucket, plan.Key) })
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	req.StudentID = summary.StudentID
	var out TranscriptDTO
	commitAttempted := false
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateTranscriptRecord(ctx, s.ids.Generate(), id.TenantID, req, plan.ObjectRef)
		if err != nil {
			return err
		}
		if err := s.writeAudit(ctx, id.TenantID, id.AccountID, actorRole, "grade.transcript.generate", auditTargetTranscript, out.ID.Int64(), map[string]any{"student_id": summary.StudentID.String(), "scope": req.Scope, "semester_id": req.SemesterID.String()}); err != nil {
			return err
		}
		commitAttempted = true
		return nil
	})
	if err != nil {
		err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, commitAttempted, func(ctx context.Context) error { return s.storage.Delete(ctx, plan.Bucket, plan.Key) })
		return TranscriptDTO{}, mapGradeTranscriptErr(err)
	}
	return out, nil
}

// GenerateTranscriptBatch 为多个学生生成成绩单,每份成绩单复用单份记录与对象存储流程。
func (s *Service) GenerateTranscriptBatch(ctx context.Context, req TranscriptBatchRequest) ([]TranscriptDTO, error) {
	if _, err := s.requireSchoolAdmin(ctx); err != nil {
		return nil, err
	}
	if req.Scope != TranscriptScopeSemester && req.Scope != TranscriptScopeFull {
		return nil, apperr.ErrGradeTranscriptFailed
	}
	if len(req.StudentIDs) == 0 || len(req.StudentIDs) > 200 {
		return nil, apperr.ErrGradeTranscriptFailed
	}
	out := make([]TranscriptDTO, 0, len(req.StudentIDs))
	seen := map[int64]struct{}{}
	for _, publicStudentID := range req.StudentIDs {
		studentID := publicStudentID.Int64()
		if studentID <= 0 {
			return nil, apperr.ErrGradeTranscriptFailed
		}
		if _, ok := seen[studentID]; ok {
			continue
		}
		seen[studentID] = struct{}{}
		item, err := s.GenerateTranscript(ctx, TranscriptRequest{StudentID: ids.ID(studentID), Scope: req.Scope, SemesterID: req.SemesterID})
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// DownloadTranscript 为成绩单对象签发短时下载授权。
func (s *Service) DownloadTranscript(ctx context.Context, transcriptID int64) (TranscriptDownloadGrantDTO, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return TranscriptDownloadGrantDTO{}, err
	}
	var record TranscriptDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		record, err = tx.GetTranscriptRecord(ctx, transcriptID)
		return err
	})
	if err != nil {
		return TranscriptDownloadGrantDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	if _, err := s.normalizeStudentOrSchoolAdmin(ctx, record.StudentID.Int64()); err != nil {
		return TranscriptDownloadGrantDTO{}, err
	}
	actorRole, err := s.gradeActorRole(ctx, id.AccountID)
	if err != nil {
		return TranscriptDownloadGrantDTO{}, err
	}
	token, grant, err := s.files.IssueDownloadGrant(storage.IssueDownloadGrantRequest{
		TenantID:         id.TenantID,
		ResourceTenantID: id.TenantID,
		AccountID:        id.AccountID,
		ObjectRef:        record.PDFRef,
		Module:           gradeModuleName,
		ResourceType:     gradeTranscriptResourceType,
		ResourceID:       record.StudentID.String(),
		ExpiresAt:        time.Time{},
	})
	if err != nil {
		return TranscriptDownloadGrantDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, actorRole, "grade.transcript.download", auditTargetTranscript, record.ID.Int64(), map[string]any{"student_id": record.StudentID.String()}); err != nil {
		return TranscriptDownloadGrantDTO{}, err
	}
	return TranscriptDownloadGrantDTO{Token: token, Transcript: record, ExpiresAt: timex.RFC3339OrEmpty(grant.ExpiresAt)}, nil
}

func (s *Service) transcriptSummary(ctx context.Context, studentID int64, scope int16, semesterID int64) (transcriptData, error) {
	id, studentID, err := s.normalizeReadableStudent(ctx, studentID)
	if err != nil {
		return transcriptData{}, err
	}
	if scope != TranscriptScopeSemester {
		semesterID = 0
	}
	page, err := s.readStudentGrades(ctx, id.TenantID, studentID, semesterID, 0, 0)
	if err != nil {
		return transcriptData{}, err
	}
	out := transcriptData{GradeSummaryDTO: GradeSummaryDTO{StudentID: page.StudentID, SemesterID: page.SemesterID, TotalCredits: page.TotalCredits, GPA: page.GPA, CumulativeGPA: page.CumulativeGPA, ComputedAt: page.ComputedAt}, CourseGrades: page.List}
	return out, nil
}

// mapGradeTranscriptErr 统一转换成绩单类错误码。
func mapGradeTranscriptErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.ErrGradeTranscriptFailed.WithCause(err)
}
