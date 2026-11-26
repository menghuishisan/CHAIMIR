// 保存和读取成绩单元数据。
package grade

import (
	"context"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/pgtypex"
)

// CreateTranscriptRecord 创建成绩单记录。
func (t *txStore) CreateTranscriptRecord(ctx context.Context, id, tenantID int64, req TranscriptRequest, pdfRef string) (TranscriptDTO, error) {
	row, err := t.q.CreateTranscriptRecord(ctx, sqlcgen.CreateTranscriptRecordParams{ID: id, TenantID: tenantID, StudentID: req.StudentID.Int64(), Scope: req.Scope, SemesterID: pgtypex.Int8(req.SemesterID.Int64()), PdfRef: pdfRef})
	if err != nil {
		return TranscriptDTO{}, err
	}
	return transcriptDTO(row), nil
}

// GetTranscriptRecord 查询成绩单记录。
func (t *txStore) GetTranscriptRecord(ctx context.Context, id int64) (TranscriptDTO, error) {
	row, err := t.q.GetTranscriptRecord(ctx, id)
	if err != nil {
		return TranscriptDTO{}, err
	}
	return transcriptDTO(row), nil
}

// ListTranscriptRecords 查询成绩单记录。
func (t *txStore) ListTranscriptRecords(ctx context.Context, studentID int64, page, size int) ([]TranscriptDTO, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListTranscriptRecords(ctx, sqlcgen.ListTranscriptRecordsParams{StudentID: studentID, PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]TranscriptDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, transcriptDTO(row))
	}
	return out, nil
}
