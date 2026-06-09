// M6 成绩导出:把单课程成绩转换为教师可下载的 Excel 文件。
package teaching

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"chaimir/pkg/apperr"

	"github.com/xuri/excelize/v2"
)

const (
	gradeExportContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	gradeExportSheetName   = "课程成绩"
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
	batchSize := s.gradeExportBatchSize
	for offset := 0; ; offset += batchSize {
		rows, err := s.repo.listCourseGradesPage(ctx, courseID, batchSize, offset)
		if err != nil {
			return nil, apperr.ErrGradeExportFailed.WithCause(err)
		}
		grades = append(grades, rows...)
		if len(rows) < batchSize {
			return grades, nil
		}
	}
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

	// 先写固定表头,导出格式由后端统一生成,避免前端各端自行拼 Excel。
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
	// 再逐行写入成绩明细,覆盖成绩为空时保留空单元格表示未人工调整。
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
	// 最后设置基础列宽并写出真实 xlsx 二进制内容。
	if err := book.SetColWidth(gradeExportSheetName, "A", "F", 18); err != nil {
		return nil, apperr.ErrGradeExportFailed.WithCause(err)
	}
	var buf bytes.Buffer
	if err := book.Write(&buf); err != nil {
		return nil, apperr.ErrGradeExportFailed.WithCause(err)
	}
	return buf.Bytes(), nil
}
