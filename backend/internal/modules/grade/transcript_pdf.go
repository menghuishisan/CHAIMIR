// M11 成绩单 PDF 渲染器:负责把已聚合的成绩数据生成正式 PDF,并写入可核验的防伪标识。
package grade

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

type transcriptDocument struct {
	TenantID   int64
	StudentID  int64
	Scope      int16
	SemesterID string
	Courses    []CourseGradeDTO
	GPA        []SemesterGradeDTO
	SigningKey string
}

// renderTranscriptDocument 使用维护型 PDF 库生成成绩单,避免业务代码手写 PDF 对象结构带来格式与安全风险。
func renderTranscriptDocument(doc transcriptDocument) ([]byte, error) {
	verification := transcriptVerificationCode(doc)
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetTitle("Chaimir Transcript", false)
	pdf.SetAuthor("Chaimir", false)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "Chaimir Transcript", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 7, fmt.Sprintf("Student: %d", doc.StudentID), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("Scope: %d", doc.Scope), "", 1, "L", false, 0, "")
	if strings.TrimSpace(doc.SemesterID) != "" {
		pdf.CellFormat(0, 7, "Semester: "+doc.SemesterID, "", 1, "L", false, 0, "")
	}
	pdf.CellFormat(0, 7, "Verification: "+verification, "", 1, "L", false, 0, "")
	pdf.Ln(3)

	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 8, "Courses", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	for _, course := range doc.Courses {
		pdf.CellFormat(0, 6, fmt.Sprintf("Course %s  Score %.1f  Credits %.1f  GPA %.3f", course.CourseID, course.FinalTotal, course.Credits, course.GPA), "", 1, "L", false, 0, "")
	}

	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 8, "GPA", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	for _, item := range doc.GPA {
		pdf.CellFormat(0, 6, fmt.Sprintf("Semester %s  GPA %.3f  Cumulative %.3f  Credits %.1f", item.SemesterID, item.GPA, item.CumulativeGPA, item.TotalCredits), "", 1, "L", false, 0, "")
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// transcriptVerificationCode 以平台 HMAC 密钥签署成绩单核心字段,用于下载后人工或系统核验来源。
func transcriptVerificationCode(doc transcriptDocument) string {
	mac := hmac.New(sha256.New, []byte(doc.SigningKey))
	fmt.Fprintf(mac, "%d:%d:%d:%s", doc.TenantID, doc.StudentID, doc.Scope, doc.SemesterID)
	for _, course := range doc.Courses {
		fmt.Fprintf(mac, ":%s:%.1f:%.1f:%.3f", course.CourseID, course.FinalTotal, course.Credits, course.GPA)
	}
	for _, item := range doc.GPA {
		fmt.Fprintf(mac, ":%s:%.3f:%.3f:%.1f", item.SemesterID, item.GPA, item.CumulativeGPA, item.TotalCredits)
	}
	sum := hex.EncodeToString(mac.Sum(nil))
	return strings.ToUpper(sum[:16])
}
