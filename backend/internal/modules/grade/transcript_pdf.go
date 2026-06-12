// grade transcript_pdf 文件负责把成绩摘要渲染为可下载 PDF 字节。
package grade

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

// renderTranscriptPDF 生成成绩单 PDF 文件内容。
func renderTranscriptPDF(summary GradeSummaryDTO, signingKey string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Chaimir Transcript", false)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(190, 10, "Chaimir Transcript")
	pdf.Ln(14)
	pdf.SetFont("Helvetica", "", 11)
	pdf.Cell(190, 8, fmt.Sprintf("Student ID: %d", summary.StudentID))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("GPA: %.3f    Credits: %.1f", summary.GPA, summary.TotalCredits))
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(35, 8, "Course")
	pdf.Cell(35, 8, "Score")
	pdf.Cell(35, 8, "Credits")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 10)
	for _, row := range summary.CourseGrades {
		pdf.Cell(35, 8, fmt.Sprintf("%d", row.CourseID))
		pdf.Cell(35, 8, fmt.Sprintf("%.2f", row.FinalTotal))
		pdf.Cell(35, 8, fmt.Sprintf("%.1f", row.Credits))
		pdf.Ln(8)
	}
	pdf.Ln(6)
	pdf.SetFont("Helvetica", "", 8)
	pdf.MultiCell(190, 5, "Verification: "+verificationText(summary, signingKey), "", "", false)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// verificationText 生成不暴露密钥的成绩单校验摘要。
func verificationText(summary GradeSummaryDTO, signingKey string) string {
	seed := fmt.Sprintf("%d:%.3f:%.1f:%s", summary.StudentID, summary.GPA, summary.TotalCredits, signingKey)
	if len(seed) <= 24 {
		return strings.ToUpper(seed)
	}
	return strings.ToUpper(seed[:24])
}
