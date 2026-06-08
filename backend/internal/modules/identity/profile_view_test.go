// 个人组织档案视图测试。
package identity

import (
	"testing"

	"chaimir/internal/modules/identity/internal/sqlcgen"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestApplyProfileToMeViewAllowsMissingInitialAdminProfile 确认首个管理员可暂缺组织档案。
func TestApplyProfileToMeViewAllowsMissingInitialAdminProfile(t *testing.T) {
	view := &MeView{ID: "1", Name: "管理员"}

	applyProfileToMeView(view, sqlcgen.AccountProfile{}, false)

	if view.No != "" || view.OrgID != "" || view.Title != "" {
		t.Fatalf("expected empty profile fields, got %#v", view)
	}
}

// TestApplyProfileToMeViewFillsExistingProfile 确认已有档案正常返回只读学籍信息。
func TestApplyProfileToMeViewFillsExistingProfile(t *testing.T) {
	view := &MeView{ID: "1", Name: "教师"}

	applyProfileToMeView(view, sqlcgen.AccountProfile{
		No:    "T001",
		OrgID: 99,
		Title: pgtype.Text{String: "教授", Valid: true},
	}, true)

	if view.No != "T001" || view.OrgID != "99" || view.Title != "教授" {
		t.Fatalf("unexpected profile fields: %#v", view)
	}
}
