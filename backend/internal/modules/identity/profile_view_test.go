// 个人组织档案投影测试。
package identity

import (
	"testing"

	"chaimir/internal/modules/identity/internal/sqlcgen"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestAccountMutationSnapshotAllowsMissingInitialAdminProfile 确认首个管理员可暂缺组织档案。
func TestAccountMutationSnapshotAllowsMissingInitialAdminProfile(t *testing.T) {
	snapshot := accountMutationSnapshot(sqlcgen.Account{ID: 1, Name: "管理员"}, nil, sqlcgen.AccountProfile{}, false)

	if snapshot.No != "" || snapshot.OrgID != 0 || snapshot.Title != "" {
		t.Fatalf("expected empty profile fields, got %#v", snapshot)
	}
}

// TestAccountMutationSnapshotFillsExistingProfile 确认已有档案正常进入个人中心投影。
func TestAccountMutationSnapshotFillsExistingProfile(t *testing.T) {
	snapshot := accountMutationSnapshot(sqlcgen.Account{ID: 1, Name: "教师"}, nil, sqlcgen.AccountProfile{
		No:    "T001",
		OrgID: 99,
		Title: pgtype.Text{String: "教授", Valid: true},
	}, true)

	if snapshot.No != "T001" || snapshot.OrgID != 99 || snapshot.Title != "教授" {
		t.Fatalf("unexpected profile fields: %#v", snapshot)
	}
}
