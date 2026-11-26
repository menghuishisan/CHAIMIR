package sandbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"github.com/jackc/pgx/v5"
)

type recycleIdentityDB struct {
	sqlcgen.DBTX
	args []any
}

func (db *recycleIdentityDB) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	db.args = args
	return recycleIdentityRow{}
}

type recycleIdentityRow struct{}

func (recycleIdentityRow) Scan(...any) error { return pgx.ErrNoRows }

func TestRecycleOutboxInsertRetainsLifecycleScope(t *testing.T) {
	db := &recycleIdentityDB{}
	repo := &txStore{q: sqlcgen.New(db)}
	sb := Sandbox{ID: 2, TenantID: 1, SourceRef: "experiment:course:instance:3", ScopeRef: "experiment:course:instance:3:run:4"}
	if _, err := repo.CreateSandboxRecycleOutbox(context.Background(), 5, sb, "finished", "trace", time.Now()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
	if len(db.args) < 5 || db.args[4] != sb.ScopeRef {
		t.Fatalf("outbox SQL lost lifecycle scope: args=%v", db.args)
	}
}

func TestRecycleOutboxRowRetainsLifecycleScope(t *testing.T) {
	row := sqlcgen.SandboxRecycleOutbox{ID: 5, TenantID: 1, SandboxID: 2,
		SourceRef: "experiment:course:instance:3", ScopeRef: "experiment:course:instance:3:run:4"}
	item := sandboxRecycleOutbox(row)
	if item.ScopeRef != row.ScopeRef || item.SourceRef != row.SourceRef || item.SandboxID != row.SandboxID || item.TenantID != row.TenantID || item.ID != row.ID {
		t.Fatalf("outbox conversion lost event identity: %+v", item)
	}
}
