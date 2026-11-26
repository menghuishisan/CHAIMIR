package contest

import (
	"context"
	"io"
	"net/url"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/terminal"
)

type terminalGrantTx struct {
	TxStore
	grant ContestAccessGrant
}

func (tx terminalGrantTx) FindContestAccessGrantForSubject(context.Context, int64, int64, int64) (ContestAccessGrant, error) {
	return tx.grant, nil
}

type terminalGrantStore struct {
	Store
	tx terminalGrantTx
}

func (s terminalGrantStore) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s.tx)
}

type terminalGrantSandbox struct {
	contracts.SandboxService
	calls               int
	access              contracts.SandboxPrincipalRequest
	toolCode, proxyPath string
}

func (s *terminalGrantSandbox) AttachSandboxTerminal(_ context.Context, access contracts.SandboxPrincipalRequest, _ contracts.SandboxTerminalTarget, _ io.Reader, _ io.Writer, _ terminal.SizeQueue) error {
	s.calls++
	s.access = access
	return nil
}

func (s *terminalGrantSandbox) ToolProxyTargetForPrincipal(_ context.Context, access contracts.SandboxPrincipalRequest, toolCode, proxyPath string) (*url.URL, error) {
	s.calls++
	s.access, s.toolCode, s.proxyPath = access, toolCode, proxyPath
	return &url.URL{Scheme: "http", Host: "service-api.sandbox-4.svc.cluster.local:8000"}, nil
}

func TestContestToolProxyPassesPathAfterGrantChecks(t *testing.T) {
	grant := ContestAccessGrant{ID: 3, GrantVersion: 7, TenantID: 9, SandboxID: 4, SourceRef: "contest:2026:problem:2:team:5:0123456789abcdef", MemberTenantID: 5, MemberAccountID: 6, Status: contestAccessGrantActive, ExpiresAt: time.Now().Add(time.Hour), Capabilities: []string{contestSandboxCapabilityWebTools}}
	for _, mode := range []string{"active", "revoked", "expired", "missing-capability"} {
		t.Run(mode, func(t *testing.T) {
			current := grant
			switch mode {
			case "revoked":
				current.Status = contestAccessGrantRevoked
			case "expired":
				current.ExpiresAt = time.Now().Add(-time.Hour)
			case "missing-capability":
				current.Capabilities = nil
			}
			sandbox := &terminalGrantSandbox{}
			svc := &Service{store: terminalGrantStore{tx: terminalGrantTx{grant: current}}, sandbox: sandbox}
			ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: grant.MemberTenantID, AccountID: grant.MemberAccountID})
			access, target, err := svc.ContestSandboxToolProxyTarget(ctx, 4, "browser", "/api/v2/blocks")
			if mode != "active" {
				if err == nil || sandbox.calls != 0 {
					t.Fatalf("invalid grant reached M2: err=%v calls=%d", err, sandbox.calls)
				}
				return
			}
			if err != nil || target == nil || sandbox.calls != 1 || sandbox.toolCode != "browser" || sandbox.proxyPath != "/api/v2/blocks" || sandbox.access != access {
				t.Fatalf("proxy request lost its path or access: target=%v err=%v sandbox=%+v", target, err, sandbox)
			}
		})
	}
}

func TestContestTerminalRechecksGrantAfterSessionBinding(t *testing.T) {
	grant := ContestAccessGrant{ID: 3, GrantVersion: 7, TenantID: 9, SandboxID: 4, SourceRef: "contest:2026:problem:2:team:5:0123456789abcdef", MemberTenantID: 5, MemberAccountID: 6, Status: contestAccessGrantActive, ExpiresAt: time.Now().Add(time.Hour), Capabilities: []string{contestSandboxCapabilityTerminal}}
	access := contracts.SandboxPrincipalRequest{TenantID: grant.TenantID, SandboxID: grant.SandboxID, SourceRef: grant.SourceRef, Principal: contracts.SandboxAccessPrincipal{AuthorizationID: grant.ID, AuthorizationRevision: grant.GrantVersion, SubjectTenantID: grant.MemberTenantID, SubjectAccountID: grant.MemberAccountID, SourceRef: grant.SourceRef}}
	for _, scenario := range []struct {
		name       string
		current    ContestAccessGrant
		wantAttach bool
	}{
		{name: "same grant", current: grant, wantAttach: true},
		{name: "revoked", current: func() ContestAccessGrant { value := grant; value.Status = contestAccessGrantRevoked; return value }()},
		{name: "new grant version", current: func() ContestAccessGrant { value := grant; value.GrantVersion++; return value }()},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			sandbox := &terminalGrantSandbox{}
			svc := &Service{store: terminalGrantStore{tx: terminalGrantTx{grant: scenario.current}}, sandbox: sandbox}
			ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: grant.MemberTenantID, AccountID: grant.MemberAccountID})
			err := svc.AttachContestSandboxTerminal(ctx, access, contracts.SandboxTerminalTarget{}, nil, io.Discard, nil)
			if scenario.wantAttach {
				if err != nil || sandbox.calls != 1 || sandbox.access != access {
					t.Fatalf("valid grant did not attach: err=%v calls=%d", err, sandbox.calls)
				}
			} else if err == nil || sandbox.calls != 0 {
				t.Fatalf("stale grant reached M2: err=%v calls=%d", err, sandbox.calls)
			}
		})
	}
}
