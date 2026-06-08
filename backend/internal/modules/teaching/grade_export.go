// M6 成绩导出:把单课程成绩转换为教师可下载的 Excel 文件。
package teaching

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/pkg/apperr"

	"github.com/xuri/excelize/v2"
)

const (
	gradeExportContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	gradeExportSheetName   = "课程成绩"
	gradeExportBatchSize   = 100
)

// GradeExportFile 是导出文件边界对象,避免 HTTP 层拼接文件格式细节。
type GradeExportFile struct {
	Filename    string
	ContentType string
	Content     []byte
}

// ExportGrades 校验课程管理权限后导出课程完整成绩。
func (s *Service) ExportGrades(ctx context.Context, courseID int64) (GradeExportFile, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return GradeExportFile{}, err
	}
	grades, err := s.listAllGradesInTenant(ctx, courseID)
	if err != nil {
		return GradeExportFile{}, err
	}
	content, err := buildGradeExportWorkbook(grades)
	if err != nil {
		return GradeExportFile{}, err
	}
	return GradeExportFile{
		Filename:    fmt.Sprintf("course-%d-grades.xlsx", courseID),
		ContentType: gradeExportContentType,
		Content:     content,
	}, nil
}

// listAllGradesInTenant 分批读取课程成绩,导出场景不能沿用列表页大小造成数据截断。
func (s *Service) listAllGradesInTenant(ctx context.Context, courseID int64) ([]CourseGradeDTO, error) {
	var grades []CourseGradeDTO
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		for offset := int32(0); ; offset += gradeExportBatchSize {
			rows, err := q.ListCourseGrades(ctx, sqlcgen.ListCourseGradesParams{
				CourseID:    courseID,
				LimitCount:  gradeExportBatchSize,
				OffsetCount: offset,
			})
			if err != nil {
				return err
			}
			for _, row := range rows {
				grades = append(grades, gradeDTOFromRow(row))
			}
			if len(rows) < gradeExportBatchSize {
				return nil
			}
		}
	}); err != nil {
		return nil, apperr.ErrGradeExportFailed.WithCause(err)
	}
	return grades, nil
}

// buildGradeExportWorkbook 使用 excelize 生成标准 xlsx,避免自研二进制格式导致客户端无法识别。
func buildGradeExportWorkbook(grades []CourseGradeDTO) (content []byte, err error) {
	book := excelize.NewFile()
	defer func() {
		if closeErr := book.Close(); closeErr != nil && err == nil {
			err = apperr.ErrGradeExportFailed.WithCause(closeErr)
		}
	}()
	defaultSheet := book.GetSheetName(0)
	if err := book.SetSheetName(defaultSheet, gradeExportSheetName); err != nil {
		return nil, apperr.ErrGradeExportFailed.WithCause(err)
	}

	headers := []string{"课程ID", "学生ID", "自动成绩", "覆盖成绩", "最终成绩", "是否手动调整"}
	for col, header := range headers {
		cell, err := excelize.CoordinatesToCellName(col+1, 1)
		if err != nil {
			return nil, apperr.ErrGradeExportFailed.WithCause(err)
		}
		if err := book.SetCellValue(gradeExportSheetName, cell, header); err != nil {
			return nil, apperr.ErrGradeExportFailed.WithCause(err)
		}
	}
	for rowIndex, grade := range grades {
		row := rowIndex + 2
		override := ""
		if grade.OverrideTotal != nil {
			override = strconv.FormatFloat(*grade.OverrideTotal, 'f', -1, 64)
		}
		manual := "否"
		if grade.IsOverridden {
			manual = "是"
		}
		values := []any{grade.CourseID, grade.StudentID, grade.AutoTotal, override, grade.FinalTotal, manual}
		for col, value := range values {
			cell, err := excelize.CoordinatesToCellName(col+1, row)
			if err != nil {
				return nil, apperr.ErrGradeExportFailed.WithCause(err)
			}
			if err := book.SetCellValue(gradeExportSheetName, cell, value); err != nil {
				return nil, apperr.ErrGradeExportFailed.WithCause(err)
			}
		}
	}
	if err := book.SetColWidth(gradeExportSheetName, "A", "F", 18); err != nil {
		return nil, apperr.ErrGradeExportFailed.WithCause(err)
	}
	var buf bytes.Buffer
	if err := book.Write(&buf); err != nil {
		return nil, apperr.ErrGradeExportFailed.WithCause(err)
	}
	return buf.Bytes(), nil
}
