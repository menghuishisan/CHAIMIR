package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/workload"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"
	"chaimir/pkg/terminal"

	"github.com/gin-gonic/gin"
)

type terminalRecheckTx struct {
	TxStore
	sandbox  Sandbox
	runtime  Runtime
	image    RuntimeImage
	active   int
	events   int
	eventErr error
}

func (tx *terminalRecheckTx) GetSandbox(context.Context, int64, int64) (Sandbox, error) {
	return tx.sandbox, nil
}
func (tx *terminalRecheckTx) GetRuntimeByID(context.Context, int64) (Runtime, error) {
	return tx.runtime, nil
}
func (tx *terminalRecheckTx) GetRuntimeImageByID(context.Context, int64, int64) (RuntimeImage, error) {
	return tx.image, nil
}
func (tx *terminalRecheckTx) LockSandboxChain(context.Context, int64, int64) error { return nil }
func (tx *terminalRecheckTx) MarkSandboxActive(context.Context, int64, int64) (Sandbox, error) {
	tx.active++
	tx.sandbox.Status = SandboxStatusRunning
	return tx.sandbox, nil
}
func (tx *terminalRecheckTx) ClaimSandboxWorkspaceRevision(_ context.Context, _, _, expected int64) (int64, error) {
	if tx.sandbox.WorkspaceRevision != expected {
		return 0, apperr.ErrConflict
	}
	tx.sandbox.WorkspaceRevision++
	return tx.sandbox.WorkspaceRevision, nil
}
func (tx *terminalRecheckTx) CreateSandboxEvent(context.Context, int64, int64, int64, string, []byte) error {
	if tx.eventErr != nil {
		return tx.eventErr
	}
	tx.events++
	return nil
}

type terminalRecheckStore struct {
	Store
	tx *terminalRecheckTx
}

func (s terminalRecheckStore) TenantTx(ctx context.Context, _ int64, fn func(context.Context, TxStore) error) error {
	active, events := s.tx.active, s.tx.events
	status := s.tx.sandbox.Status
	revision := s.tx.sandbox.WorkspaceRevision
	err := fn(ctx, s.tx)
	if err != nil {
		s.tx.active, s.tx.events = active, events
		s.tx.sandbox.Status = status
		s.tx.sandbox.WorkspaceRevision = revision
	}
	return err
}
func (s terminalRecheckStore) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s.tx)
}

type terminalRecheckIdentity struct{ contracts.IdentityService }

func (terminalRecheckIdentity) RequireTenantModules(context.Context, int64, ...string) error {
	return nil
}

type terminalRecheckOrchestrator struct {
	Orchestrator
	streams int
	targets []string
}

func (o *terminalRecheckOrchestrator) ToolReady(context.Context, Sandbox, Tool) error { return nil }
func (o *terminalRecheckOrchestrator) ExecStream(_ context.Context, _, target string, _ []string, _ io.Reader, _ io.Writer, _ io.Writer, _ bool, _ terminal.SizeQueue) error {
	o.streams++
	o.targets = append(o.targets, target)
	return nil
}

func terminalRecheckFixture(t *testing.T) (*Service, *terminalRecheckTx, contracts.SandboxPrincipalRequest) {
	t.Helper()
	imageURL := "registry.example/runtime@sha256:" + strings.Repeat("a", 64)
	mountWorkspace := true
	adapter, err := json.Marshal(AdapterSpec{
		WorkspaceDir:     "/workspace",
		RuntimeContainer: workload.ComponentSpec{Name: "runtime"},
		InfraSidecars: []workload.ComponentSpec{{Name: "init-script", ImageURL: imageURL, MountWorkspace: &mountWorkspace,
			EphemeralMounts: []workload.EphemeralMountSpec{{Name: "tmp", MountPath: "/tmp"}}}},
		Pods: []workload.PodSpec{{Name: "sandbox", Containers: []workload.ComponentSpec{
			{Name: "runtime"}, {Name: "student", Labels: map[string]string{studentAccessLabel: "true"}}, {Name: "init-script"},
		}}},
		WorkspaceOps: WorkspaceOps{ExecTarget: "sandbox/student", ScriptExecTarget: "sandbox/init-script",
			RunScript: []string{"run"}, CleanWorkspace: []string{"clean"},
			Terminal: []string{"terminal", "{workspace}"}, WriteFile: []string{"write-file", "{path}"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := contracts.SandboxCompositionSnapshot{
		Spec:       contracts.SandboxCompositionSpec{ID: "terminal-test", AccessProfile: contracts.SandboxAccessExperiment, WorkspaceRuntimeInstance: "chain", Runtimes: []contracts.CompositionRuntimeRef{{InstanceCode: "chain", Code: "evm", ImageVersion: "1"}}},
		Runtimes:   []contracts.CompiledRuntimeSnapshot{{InstanceCode: "chain", RuntimeID: 1, ImageID: 2, Code: "evm", ImageVersion: "1", ImageURL: imageURL, AdapterSpec: adapter}},
		Components: []contracts.CompiledComponentSnapshot{{ComponentID: 3, Category: "tool", Code: "checker", Kind: SandboxToolKindCommand, ResourceSpec: json.RawMessage(`{"components":[{"name":"checker"}]}`)}},
	}
	snapshot.Digest, err = contracts.CanonicalSnapshotDigest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	sb := Sandbox{ID: 4, TenantID: 5, Namespace: "sandbox-4", SourceRef: "experiment:2026:instance:4", OwnerAccountID: 6, SharedAccountIDs: []int64{7}, Phase: SandboxPhaseFullyReady, Status: SandboxStatusIdle, CompositionDigest: snapshot.Digest, CompositionSnapshot: raw, WorkspaceRevision: 1}
	runtime, image, err := workspaceRuntimeSnapshot(sb)
	if err != nil {
		t.Fatal(err)
	}
	runtime.Status, runtime.SelftestStatus = RuntimeStatusAvailable, RuntimeSelftestPassed
	image.Status = RuntimeImageStatusAvailable
	tx := &terminalRecheckTx{sandbox: sb, runtime: runtime, image: image}
	node, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{store: terminalRecheckStore{tx: tx}, identity: terminalRecheckIdentity{}, orchestrator: &terminalRecheckOrchestrator{}, ids: node}
	access := contracts.SandboxPrincipalRequest{
		TenantID: sb.TenantID, SandboxID: sb.ID, SourceRef: sb.SourceRef,
		Principal: contracts.SandboxAccessPrincipal{AuthorizationID: 8, AuthorizationRevision: 1, SubjectTenantID: 9, SubjectAccountID: 10, SourceRef: sb.SourceRef},
	}
	return svc, tx, access
}

func TestTerminalTargetsDoNotUpdateActivity(t *testing.T) {
	for _, principal := range []bool{false, true} {
		for _, container := range []string{"", "chain-sandbox/student", "chain-sandbox/runtime", "unknown/container"} {
			svc, tx, access := terminalRecheckFixture(t)
			var err error
			if principal {
				_, err = svc.TerminalTargetForPrincipal(context.Background(), access, container)
			} else {
				_, err = svc.TerminalTargetForOwner(context.Background(), 5, 6, 4, container)
			}
			allowed := container == "" || container == "chain-sandbox/student"
			if (err == nil) != allowed {
				t.Fatalf("principal=%v container=%q error=%v", principal, container, err)
			}
			if tx.active != 0 || tx.events != 0 {
				t.Fatalf("target lookup wrote state: principal=%v container=%q active=%d events=%d", principal, container, tx.active, tx.events)
			}
		}
	}
}

func TestWorkspaceRejectsDisabledRuntimeImage(t *testing.T) {
	for _, principal := range []bool{false, true} {
		t.Run(fmt.Sprint(principal), func(t *testing.T) {
			svc, tx, access := terminalRecheckFixture(t)
			tx.image.Status = RuntimeImageStatusDisabled
			var err error
			if principal {
				_, err = svc.TerminalTargetForPrincipal(context.Background(), access, "")
			} else {
				_, err = svc.TerminalTargetForOwner(context.Background(), tx.sandbox.TenantID, tx.sandbox.OwnerAccountID, tx.sandbox.ID, "")
			}
			if err == nil || tx.active != 0 {
				t.Fatalf("disabled image reached workspace access: err=%v activity=%d", err, tx.active)
			}
		})
	}
}

func TestTerminalAttachRechecksOwnerAndPrincipalBeforeSideEffects(t *testing.T) {
	svc, tx, access := terminalRecheckFixture(t)
	target, err := svc.TerminalTargetForPrincipal(context.Background(), access, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*contracts.SandboxTerminalTarget){
		func(v *contracts.SandboxTerminalTarget) { v.TenantID++ },
		func(v *contracts.SandboxTerminalTarget) { v.SandboxID++ },
		func(v *contracts.SandboxTerminalTarget) { v.Namespace = "forged" },
		func(v *contracts.SandboxTerminalTarget) { v.Container = "chain-sandbox/runtime" },
		func(v *contracts.SandboxTerminalTarget) { v.Command = []string{"forged"} },
	} {
		forged := target
		mutate(&forged)
		if err := svc.AttachSandboxTerminal(context.Background(), access, forged, nil, io.Discard, nil); !errors.Is(err, apperr.ErrSandboxOwnershipInvalid) {
			t.Fatalf("forged principal target accepted: %v", err)
		}
	}
	ownerTarget, err := svc.TerminalTargetForOwner(context.Background(), 5, 6, 4, "")
	if err != nil {
		t.Fatal(err)
	}
	tx.sandbox.SharedAccountIDs = nil
	if err := svc.AttachTerminalForOwner(context.Background(), 5, 7, ownerTarget, nil, io.Discard, nil); !errors.Is(err, apperr.ErrSandboxOwnershipInvalid) {
		t.Fatalf("revoked member attached: %v", err)
	}
	if tx.active != 0 || tx.events != 0 || svc.orchestrator.(*terminalRecheckOrchestrator).streams != 0 {
		t.Fatalf("rejected target changed state: active=%d events=%d", tx.active, tx.events)
	}
	if err := svc.AttachTerminalForOwner(context.Background(), 5, 6, ownerTarget, nil, io.Discard, nil); err != nil {
		t.Fatalf("owner attach failed: %v", err)
	}
	if tx.active != 1 || tx.events != 1 || svc.orchestrator.(*terminalRecheckOrchestrator).streams != 1 {
		t.Fatalf("valid attach side effects mismatch: active=%d events=%d streams=%d", tx.active, tx.events, svc.orchestrator.(*terminalRecheckOrchestrator).streams)
	}
}

func TestToolProxyValidatesTargetBeforeMarkingActive(t *testing.T) {
	for _, principal := range []bool{false, true} {
		for _, validRoute := range []bool{false, true} {
			svc, tx, access := terminalRecheckFixture(t)
			var snapshot contracts.SandboxCompositionSnapshot
			if err := json.Unmarshal(tx.sandbox.CompositionSnapshot, &snapshot); err != nil {
				t.Fatal(err)
			}
			spec := ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "browser"}}}
			if validRoute {
				spec.Components[0].Ports = []workload.PortSpec{{Name: "http", ContainerPort: 9000, Protocol: "TCP"}}
				spec.Services = []workload.ServiceSpec{{Name: "browser", Component: "browser", Ports: []workload.ServicePortSpec{{Name: "http", Port: 8080, TargetPort: "http", Protocol: "TCP"}}}}
				spec.Routes = []workload.RouteSpec{{PathPrefix: "/", Service: "browser", Port: "http"}}
			}
			snapshot.Components[0].Kind = SandboxToolKindWebEmbed
			var err error
			snapshot.Components[0].ResourceSpec, err = json.Marshal(spec)
			if err != nil {
				t.Fatal(err)
			}
			snapshot.Digest, err = contracts.CanonicalSnapshotDigest(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			tx.sandbox.CompositionDigest = snapshot.Digest
			tx.sandbox.CompositionSnapshot, err = json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if principal {
				_, err = svc.ToolProxyTargetForPrincipal(context.Background(), access, "checker", "/")
			} else {
				_, _, err = svc.ToolProxyTargetForOwner(context.Background(), 5, 6, 4, "checker", "/")
			}
			if (err == nil) != validRoute {
				t.Fatalf("principal=%v validRoute=%v error=%v", principal, validRoute, err)
			}
			wantActive := 0
			if validRoute {
				wantActive = 1
			}
			if tx.active != wantActive {
				t.Fatalf("principal=%v validRoute=%v activity=%d want=%d", principal, validRoute, tx.active, wantActive)
			}
		}
	}
}

func proxyRouteFixture(t *testing.T, routes []workload.RouteSpec) (*Service, *terminalRecheckTx, contracts.SandboxPrincipalRequest) {
	t.Helper()
	svc, tx, access := terminalRecheckFixture(t)
	var snapshot contracts.SandboxCompositionSnapshot
	if err := json.Unmarshal(tx.sandbox.CompositionSnapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	spec := ToolResourceSpec{Routes: routes}
	for _, name := range []string{"web", "api", "v2"} {
		spec.Components = append(spec.Components, workload.ComponentSpec{Name: name, Ports: []workload.PortSpec{{Name: "http", ContainerPort: 9000, Protocol: "TCP"}}})
		spec.Services = append(spec.Services, workload.ServiceSpec{Name: "service-" + name, Component: name,
			Ports: []workload.ServicePortSpec{{Name: "http", Port: 8000, TargetPort: "http", Protocol: "TCP"}}})
	}
	var err error
	snapshot.Components[0].Kind = SandboxToolKindWebEmbed
	snapshot.Components[0].ResourceSpec, err = json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Digest, err = contracts.CanonicalSnapshotDigest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	tx.sandbox.CompositionDigest = snapshot.Digest
	tx.sandbox.CompositionSnapshot, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return svc, tx, access
}

func TestOwnerToolProxyUsesLongestDeclaredPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("platform credentials reached the tool")
		}
		if _, err := io.WriteString(w, r.Host+"|"+r.URL.RequestURI()); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(upstream.Close)
	// 保留生产代理选择的 Host，只将网络拨号导向本机 HTTP 服务。
	previous := http.DefaultTransport
	transport := previous.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", strings.TrimPrefix(upstream.URL, "http://"))
	}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections() })

	for _, reverse := range []bool{false, true} {
		for _, tc := range []struct {
			path, service string
			noRoot        bool
		}{
			{path: "/", service: "web"}, {path: "/assets/app.js", service: "web"}, {path: "/api", service: "api"}, {path: "/api/", service: "api"},
			{path: "/api/blocks", service: "api"}, {path: "/api/v2/blocks", service: "v2"}, {path: "/apix", service: "web"}, {path: "/api%20", service: "web"},
			{path: "/api/blocks", service: "api", noRoot: true}, {path: "/apix", noRoot: true}, {path: "/other", noRoot: true},
		} {
			t.Run(fmt.Sprintf("reverse=%t,root=%t%s", reverse, !tc.noRoot, tc.path), func(t *testing.T) {
				routes := []workload.RouteSpec{{PathPrefix: "/", Service: "service-web", Port: "http"},
					{PathPrefix: "/api", Service: "service-api", Port: "http"}, {PathPrefix: "/api/v2", Service: "service-v2", Port: "http"}}
				if tc.noRoot {
					routes = routes[1:]
				}
				if reverse {
					slices.Reverse(routes)
				}
				svc, tx, _ := proxyRouteFixture(t, routes)
				router := gin.New()
				router.Any("/api/v1/sandbox/sandboxes/:id/tools/:tool_code/*proxy_path", (sandboxAPI{svc: svc}).toolProxy)
				req := httptest.NewRequest(http.MethodGet, "/api/v1/sandbox/sandboxes/4/tools/checker"+tc.path+"?q=block%2Fone", nil)
				req.Header.Set("Authorization", "must-stay-on-platform")
				req.Header.Set("Cookie", "platform=must-stay-on-platform")
				ctx, cancel := context.WithCancel(req.Context())
				defer cancel()
				req = req.WithContext(tenant.WithContext(ctx, tenant.Identity{TenantID: 5, AccountID: 6}))
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				if tc.service == "" {
					var envelope struct{ Code string }
					if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Code != apperr.ErrSandboxToolProxyUnavailable.UserCode() || tx.active != 0 {
						t.Fatalf("unmatched path was accepted: body=%q activity=%d err=%v", response.Body.String(), tx.active, err)
					}
					return
				}
				want := "service-" + tc.service + ".sandbox-4.svc.cluster.local:8000|" + tc.path + "?q=block%2Fone"
				if response.Code != http.StatusOK || response.Body.String() != want || tx.active != 1 {
					t.Fatalf("wrong proxy route: status=%d body=%q activity=%d want=%q", response.Code, response.Body.String(), tx.active, want)
				}
			})
		}
	}
}

func TestToolProxyOwnerAndPrincipalUseSamePathRules(t *testing.T) {
	for _, principal := range []bool{false, true} {
		for _, tc := range []struct {
			path, namespace, service string
		}{
			{path: "/api", namespace: "sandbox-4", service: "api"},
			{path: "/api/blocks", namespace: "sandbox-4", service: "api"},
			{path: "/apix", namespace: "sandbox-4"},
			{path: "/other", namespace: "sandbox-4"},
			{path: "/api"},
		} {
			t.Run(fmt.Sprintf("principal=%t,namespace=%s%s", principal, tc.namespace, tc.path), func(t *testing.T) {
				svc, tx, access := proxyRouteFixture(t, []workload.RouteSpec{{PathPrefix: "/api", Service: "service-api", Port: "http"}})
				tx.sandbox.Namespace = tc.namespace
				var target *url.URL
				var err error
				if principal {
					target, err = svc.ToolProxyTargetForPrincipal(context.Background(), access, "checker", tc.path)
				} else {
					_, target, err = svc.ToolProxyTargetForOwner(context.Background(), 5, 6, 4, "checker", tc.path)
				}
				if tc.service == "" {
					if err == nil || target != nil || tx.active != 0 {
						t.Fatalf("invalid target wrote activity: target=%v err=%v activity=%d", target, err, tx.active)
					}
					return
				}
				want := "http://service-api.sandbox-4.svc.cluster.local:8000"
				if err != nil || target == nil || target.String() != want || tx.active != 1 {
					t.Fatalf("proxy target mismatch: target=%v err=%v activity=%d want=%s", target, err, tx.active, want)
				}
			})
		}
	}
}

func TestTerminalEventFailureRollsBackActivityBeforeExec(t *testing.T) {
	svc, tx, access := terminalRecheckFixture(t)
	target, err := svc.TerminalTargetForPrincipal(context.Background(), access, "")
	if err != nil {
		t.Fatal(err)
	}
	tx.eventErr = errors.New("event write failed")
	err = svc.AttachSandboxTerminal(context.Background(), access, target, nil, io.Discard, nil)
	if err == nil || tx.active != 0 || tx.events != 0 || tx.sandbox.Status != SandboxStatusIdle || svc.orchestrator.(*terminalRecheckOrchestrator).streams != 0 {
		t.Fatalf("event failure did not roll back before exec: err=%v active=%d events=%d status=%d streams=%d", err, tx.active, tx.events, tx.sandbox.Status, svc.orchestrator.(*terminalRecheckOrchestrator).streams)
	}
}

type terminalSessionCloser struct {
	sessions []ws.SessionKey
	err      error
}

func (c *terminalSessionCloser) CloseSession(session ws.SessionKey) error {
	c.sessions = append(c.sessions, session)
	return c.err
}

func TestSandboxRevocationFanoutClosesExactSessions(t *testing.T) {
	hub := &terminalSessionCloser{}
	event := contracts.SandboxSessionsRevokedEvent{TenantID: 5, Sessions: []contracts.SandboxSessionRef{{SandboxID: 4, AccountID: 6}, {SandboxID: 7, AccountID: 8}}}
	data, err := json.Marshal(map[string]any{"tenant_id": event.TenantID, "trace_id": "sandbox-control-test", "sessions": event.Sessions})
	if err != nil {
		t.Fatal(err)
	}
	if err := handleSandboxSessionsRevoked(hub)(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	want := []ws.SessionKey{{TenantID: 5, AccountID: 6, Scope: sandboxTerminalSessionScope(4)}, {TenantID: 5, AccountID: 6, Scope: sandboxProgressSessionScope(4)}, {TenantID: 5, AccountID: 8, Scope: sandboxTerminalSessionScope(7)}, {TenantID: 5, AccountID: 8, Scope: sandboxProgressSessionScope(7)}}
	if !slices.Equal(hub.sessions, want) {
		t.Fatalf("closed sessions=%+v want=%+v", hub.sessions, want)
	}
}

type terminalFanoutBus struct {
	event   any
	subject string
	ctxErr  error
	err     error
}

func (b *terminalFanoutBus) Publish(context.Context, string, any) error { return nil }
func (b *terminalFanoutBus) Subscribe(string, string, eventbus.Handler) (eventbus.Subscription, error) {
	return nil, nil
}
func (b *terminalFanoutBus) Close() error { return nil }
func (b *terminalFanoutBus) PublishFanout(ctx context.Context, subject string, event any) error {
	b.event, b.subject, b.ctxErr = event, subject, ctx.Err()
	return b.err
}
func (b *terminalFanoutBus) SubscribeFanout(string, eventbus.Handler) (eventbus.Subscription, error) {
	return nil, nil
}

func TestSandboxRevocationBroadcastSurvivesRequestCancellation(t *testing.T) {
	bus := &terminalFanoutBus{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := publishSandboxSessionsRevoked(ctx, bus, 5, 4, []int64{6, 8}); err != nil {
		t.Fatal(err)
	}
	event, ok := bus.event.(contracts.SandboxSessionsRevokedEvent)
	if !ok || bus.subject != contracts.SubjectSandboxSessionsRevoked || bus.ctxErr != nil || event.TenantID != 5 || !slices.Equal(event.Sessions, []contracts.SandboxSessionRef{{SandboxID: 4, AccountID: 6}, {SandboxID: 4, AccountID: 8}}) {
		t.Fatalf("revocation fanout mismatch: subject=%q event=%#v ctxErr=%v", bus.subject, bus.event, bus.ctxErr)
	}
	if err := publishSandboxSessionsRevoked(context.Background(), bus, 5, 4, nil); err != nil {
		t.Fatalf("empty revoke should not publish: %v", err)
	}
}

type workspaceWriteOrchestrator struct {
	Orchestrator
	execs  int
	err    error
	stdout []byte
}

func (o *workspaceWriteOrchestrator) Exec(context.Context, string, string, []string, []byte, bool) ([]byte, []byte, error) {
	o.execs++
	return o.stdout, nil, o.err
}

func TestWorkspaceWritesValidateAndRecordBeforeExec(t *testing.T) {
	failure := errors.New("injected failure")
	for _, tt := range []struct {
		name       string
		expected   int64
		eventErr   error
		execErr    error
		wantExecs  int
		wantEvents int
		wantActive int
		wantRev    int64
		wantError  error
	}{
		{name: "conflict", expected: 99, wantRev: 1, wantError: apperr.ErrConflict},
		{name: "event-failure", expected: 1, eventErr: failure, wantRev: 1, wantError: failure},
		{name: "exec-failure", expected: 1, execErr: failure, wantExecs: 1, wantRev: 1, wantError: failure},
		{name: "success", expected: 1, wantExecs: 1, wantEvents: 1, wantActive: 1, wantRev: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, tx, access := terminalRecheckFixture(t)
			tx.eventErr = tt.eventErr
			o := &workspaceWriteOrchestrator{err: tt.execErr}
			svc.orchestrator = o
			_, err := svc.PutSandboxFile(context.Background(), contracts.SandboxFileWriteRequest{
				TenantID: access.TenantID, SandboxID: access.SandboxID, SourceRef: access.SourceRef,
				RelativePath: "main.txt", ContentBase64: base64.StdEncoding.EncodeToString([]byte("new content")), ExpectedRevision: tt.expected,
			})
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("unexpected result: got=%v want=%v", err, tt.wantError)
			}
			if o.execs != tt.wantExecs || tx.active != tt.wantActive || tx.events != tt.wantEvents || tx.sandbox.WorkspaceRevision != tt.wantRev {
				t.Fatalf("execution/activity/event/revision mismatch: %d/%d/%d/%d", o.execs, tx.active, tx.events, tx.sandbox.WorkspaceRevision)
			}
		})
	}
}

func TestInvalidExecTargetDoesNotUpdateActivity(t *testing.T) {
	svc, tx, access := terminalRecheckFixture(t)
	svc.cfg.ExecTimeoutSeconds = 1
	o := &workspaceWriteOrchestrator{}
	svc.orchestrator = o
	_, err := svc.ExecSandboxCommand(context.Background(), contracts.SandboxExecRequest{
		TenantID: access.TenantID, SandboxID: access.SandboxID, SourceRef: access.SourceRef,
		Command: []string{"echo"}, Container: "bad//container",
	})
	if !errors.Is(err, apperr.ErrSandboxContractRequestInvalid) || tx.active != 0 || tx.events != 0 || o.execs != 0 {
		t.Fatalf("invalid exec target changed activity: err=%v activity=%d events=%d execs=%d", err, tx.active, tx.events, o.execs)
	}
}

type resetCapability struct {
	ChainCapability
	calls     []string
	deadlines []time.Time
	resetErr  error
	queryErr  error
}

func (c *resetCapability) Reset(ctx context.Context, _ Sandbox, _ Runtime) error {
	c.calls = append(c.calls, "reset")
	deadline, _ := ctx.Deadline()
	c.deadlines = append(c.deadlines, deadline)
	return c.resetErr
}

func (c *resetCapability) Query(ctx context.Context, _ Sandbox, _ Runtime, target string) (map[string]any, error) {
	c.calls = append(c.calls, "query:"+target)
	deadline, _ := ctx.Deadline()
	c.deadlines = append(c.deadlines, deadline)
	return nil, c.queryErr
}

type resetOrchestrator struct {
	Orchestrator
	targets  []string
	archives []RuntimeInitArchive
	err      error
}

func (o *resetOrchestrator) ResetSandboxRuntime(_ context.Context, _ CreateSandboxPlan, instance string, archives []RuntimeInitArchive) error {
	o.targets = append(o.targets, instance)
	o.archives = archives
	return o.err
}

type initArchiveStorage struct {
	objectStorage
	data  []byte
	err   error
	reads int
}

func (s *initArchiveStorage) BucketCode() string   { return "code" }
func (s *initArchiveStorage) BucketAttach() string { return "attach" }
func (s *initArchiveStorage) Get(context.Context, string, string) (io.ReadCloser, error) {
	s.reads++
	if s.err != nil {
		return nil, s.err
	}
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

func initArchiveTar(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	if err := writer.WriteHeader(&tar.Header{Name: "genesis.json", Mode: 0600, Size: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestInitAssetOwnershipAndDigest(t *testing.T) {
	data := initArchiveTar(t)
	for _, tt := range []struct {
		name      string
		tenant    int64
		ref       string
		badHash   bool
		emptyHash bool
		unsafe    bool
		wantRead  bool
		wantOK    bool
	}{
		{name: "platform-selftest", ref: "s3://attach/0/sandbox/runtime-assets/genesis.tar", wantRead: true, wantOK: true},
		{name: "shared-runtime-in-tenant", tenant: 456, ref: "s3://attach/0/sandbox/runtime-assets/genesis.tar", wantRead: true, wantOK: true},
		{name: "teacher-owned", tenant: 456, ref: "s3://attach/456/genesis.tar", wantRead: true, wantOK: true},
		{name: "teacher-code-bucket", tenant: 456, ref: "s3://code/456/genesis.tar", wantRead: true, wantOK: true},
		{name: "other-tenant", tenant: 456, ref: "s3://attach/789/genesis.tar"},
		{name: "platform-cannot-read-teacher", ref: "s3://attach/456/genesis.tar"},
		{name: "negative-tenant", tenant: -1, ref: "s3://attach/0/genesis.tar"},
		{name: "wrong-bucket", tenant: 456, ref: "s3://reports/0/genesis.tar"},
		{name: "prefix-lookalike", tenant: 456, ref: "s3://attach/00/genesis.tar"},
		{name: "invalid-ref", tenant: 456, ref: "s3://attach/0/../genesis.tar"},
		{name: "shared-digest-mismatch", tenant: 456, ref: "s3://attach/0/genesis.tar", badHash: true, wantRead: true},
		{name: "shared-digest-required", tenant: 456, ref: "s3://attach/0/genesis.tar", emptyHash: true},
		{name: "shared-unsafe-archive", tenant: 456, ref: "s3://attach/0/genesis.tar", unsafe: true, wantRead: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plan, _ := runtimeAssetPlan()
			objects := &initArchiveStorage{data: data}
			if tt.unsafe {
				objects.data = []byte("not an archive")
			}
			asset := &plan.Runtimes[0].Runtime.AdapterSpec.InitAssets[0]
			asset.SourceRef, asset.ExpectedSHA256 = tt.ref, pkgcrypto.SHA256Hex(objects.data)
			if tt.badHash {
				asset.ExpectedSHA256 = strings.Repeat("b", 64)
			}
			if tt.emptyHash {
				asset.ExpectedSHA256 = ""
			}
			svc := &Service{objectStorage: objects, cfg: config.SandboxConfig{InitArchiveMaxBytes: 1 << 20, InitArchiveMaxFiles: 10, InitArchiveMaxUnpackedBytes: 1 << 20}}
			archives, err := svc.prepareRuntimeInitArchives(context.Background(), Sandbox{TenantID: tt.tenant}, plan.Runtimes)
			if (err == nil) != tt.wantOK || (objects.reads != 0) != tt.wantRead {
				t.Fatalf("ownership or digest boundary violated: err=%v reads=%d", err, objects.reads)
			}
			if tt.wantOK {
				if len(archives) != 1 || archives[0].RuntimeInstance != "chain" || archives[0].TargetDomain != VolumeDomainRuntimeState {
					t.Fatalf("lost runtime asset destination: %v", archives)
				}
				reader := tar.NewReader(bytes.NewReader(archives[0].Tarball))
				header, err := reader.Next()
				if err != nil || header.Name != "genesis.json" {
					t.Fatalf("archive was not safely repacked: header=%v err=%v", header, err)
				}
			}
		})
	}
}

type initAssetCompileStore struct {
	Store
	entered bool
	stop    error
}

func (s *initAssetCompileStore) PlatformTx(context.Context, func(context.Context, TxStore) error) error {
	s.entered = true
	return s.stop
}

func TestTeacherCompositionRejectsUnownedInitAssetsBeforeFreezing(t *testing.T) {
	for _, tt := range []struct {
		name string
		ref  string
		own  bool
	}{
		{name: "own-attachment", ref: "s3://attach/456/genesis.tar", own: true},
		{name: "own-code", ref: "s3://code/456/genesis.tar", own: true},
		{name: "arbitrary-platform-object", ref: "s3://attach/0/genesis.tar"},
		{name: "other-tenant", ref: "s3://attach/789/genesis.tar"},
		{name: "wrong-bucket", ref: "s3://reports/456/genesis.tar"},
		{name: "invalid-ref", ref: "s3://attach/456/../genesis.tar"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stop := errors.New("catalog transaction boundary")
			store := &initAssetCompileStore{stop: stop}
			svc := &Service{store: store, objectStorage: &initArchiveStorage{}}
			spec := contracts.SandboxCompositionSpec{ID: "assets", AccessProfile: contracts.SandboxAccessExperiment,
				WorkspaceRuntimeInstance: "chain", Runtimes: []contracts.CompositionRuntimeRef{{InstanceCode: "chain", Code: "evm", ImageVersion: "v1"}},
				InitAssets: []contracts.CompositionInitAsset{{Name: "genesis", SourceRef: tt.ref, TargetRuntimeInstance: "chain", TargetDomain: VolumeDomainRuntimeState, ExpectedSHA256: strings.Repeat("a", 64)}}}
			if _, err := contracts.CanonicalDigest(spec); err != nil {
				t.Fatal(err)
			}
			_, err := svc.CompileSandboxComposition(context.Background(), 456, spec)
			if store.entered != tt.own || (tt.own && !errors.Is(err, stop)) || (!tt.own && !errors.Is(err, apperr.ErrSandboxInitObjectRefInvalid)) {
				t.Fatalf("teacher material reached the shared snapshot registry: err=%v entered=%v", err, store.entered)
			}
		})
	}
}

func initScriptRuntime() Runtime {
	mount := true
	return Runtime{AdapterSpec: AdapterSpec{WorkspaceDir: "/workspace", WorkspaceOps: WorkspaceOps{
		ExecTarget: "node/student", ScriptExecTarget: "node/worker", RunScript: []string{"run"}, CleanWorkspace: []string{"clean"},
	}, Pods: []workload.PodSpec{{Name: "node", Containers: []workload.ComponentSpec{{
		Name: "worker", MountWorkspace: &mount, EphemeralMounts: []workload.EphemeralMountSpec{{Name: "tmp", MountPath: "/tmp"}},
	}}}}}}
}

func TestSharedInitAssetsDoNotAuthorizeWorkspaceOrScripts(t *testing.T) {
	runtime := initScriptRuntime()
	if _, err := initScriptExecTarget(runtime.AdapterSpec); err != nil {
		t.Fatal(err)
	}
	for _, tenant := range []int64{0, 456} {
		objects := &initArchiveStorage{}
		svc := &Service{objectStorage: objects}
		sb := Sandbox{TenantID: tenant}
		ref := "s3://attach/0/sandbox/runtime-assets/genesis.tar"
		if err := svc.restoreArchiveToWorkspace(context.Background(), sb, runtime, ref, "", "."); !errors.Is(err, apperr.ErrSandboxInitObjectRefInvalid) {
			t.Fatalf("workspace restore accepted platform material: tenant=%d err=%v", tenant, err)
		}
		if err := svc.runInitScriptIfNeeded(context.Background(), sb, runtime, ref); !errors.Is(err, apperr.ErrSandboxInitObjectRefInvalid) {
			t.Fatalf("script execution accepted platform material: tenant=%d err=%v", tenant, err)
		}
		if objects.reads != 0 {
			t.Fatal("unowned material reached object storage")
		}
	}
}

type initScriptOrchestrator struct {
	Orchestrator
	input  []byte
	target string
	err    error
}

func (o *initScriptOrchestrator) Exec(_ context.Context, _, target string, _ []string, input []byte, _ bool) ([]byte, []byte, error) {
	o.input, o.target = input, target
	return []byte("private stdout"), []byte("private stderr"), o.err
}

func TestInitScriptKeepsBoundedTenantReadAndPrivateExecution(t *testing.T) {
	readFailure := errors.New("object unavailable")
	execFailure := errors.New("execution failed")
	for _, tt := range []struct {
		name string
		data []byte
		read error
		exec error
		want error
	}{
		{name: "success", data: []byte("echo initialized\n")},
		{name: "empty", want: apperr.ErrSandboxInitArchiveTooLarge},
		{name: "too-large", data: bytes.Repeat([]byte("a"), 65), want: apperr.ErrSandboxInitArchiveTooLarge},
		{name: "read-failure", read: readFailure, want: readFailure},
		{name: "execution-failure", data: []byte("echo private\n"), exec: execFailure, want: execFailure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, tx, _ := terminalRecheckFixture(t)
			orchestrator := &initScriptOrchestrator{err: tt.exec}
			svc.orchestrator, svc.objectStorage = orchestrator, &initArchiveStorage{data: tt.data, err: tt.read}
			svc.cfg.InitArchiveMaxBytes = 64
			err := svc.runInitScriptIfNeeded(context.Background(), tx.sandbox, initScriptRuntime(), "s3://attach/5/init.sh")
			if !errors.Is(err, tt.want) {
				t.Fatalf("script result=%v want=%v", err, tt.want)
			}
			if tt.want == nil || tt.exec != nil {
				if orchestrator.target != "node/worker" || !bytes.Equal(orchestrator.input, tt.data) {
					t.Fatal("script did not reach the private worker through stdin")
				}
			} else if orchestrator.target != "" {
				t.Fatal("invalid object reached execution")
			}
			if (tx.events != 0) != (tt.want == nil) {
				t.Fatal("script success event does not match execution outcome")
			}
			if err != nil && (strings.Contains(err.Error(), "private stdout") || strings.Contains(err.Error(), "private stderr")) {
				t.Fatal("execution error exposed private script output")
			}
		})
	}
}

func TestRuntimeResetPreparesAllAssetsBeforeDestruction(t *testing.T) {
	data := initArchiveTar(t)
	for _, name := range []string{"success", "read-failure", "digest-mismatch", "unsafe-archive", "invalid-declaration"} {
		t.Run(name, func(t *testing.T) {
			plan, _ := runtimeAssetPlan()
			store := &initArchiveStorage{data: data}
			if name == "unsafe-archive" {
				store.data = []byte("not an archive")
			}
			asset := &plan.Runtimes[0].Runtime.AdapterSpec.InitAssets[0]
			asset.ExpectedSHA256 = pkgcrypto.SHA256Hex(store.data)
			if name == "read-failure" {
				store.err = errors.New("object unavailable")
			}
			if name == "digest-mismatch" {
				asset.ExpectedSHA256 = strings.Repeat("b", 64)
			}
			if name == "invalid-declaration" {
				asset.ExpectedSHA256 = ""
			}
			plan.Runtimes[0].Runtime.AdapterSpec.Selftest = map[string]any{"query_target": "chain-id"}
			plan.Runtimes[0].Runtime.AdapterSpec.CapabilityCommands.ResetStrategy = "recreate_runtime"
			o := &resetOrchestrator{}
			cap := &resetCapability{}
			s := &Service{orchestrator: o, objectStorage: store, cfg: config.SandboxConfig{
				ReadyTimeoutSeconds: 2, ReadyPollIntervalSeconds: 1, ChainRPCTimeoutSeconds: 1,
				InitArchiveMaxBytes: 1 << 20, InitArchiveMaxFiles: 10, InitArchiveMaxUnpackedBytes: 1 << 20,
			}}
			err := s.resetRuntimeForPlan(context.Background(), plan, "chain", cap)
			if name == "success" {
				if err != nil || len(o.archives) != 1 || o.archives[0].RuntimeInstance != "chain" || o.archives[0].TargetDomain != VolumeDomainRuntimeState || len(o.archives[0].Tarball) == 0 || !slices.Equal(cap.calls, []string{"query:chain-id"}) {
					t.Fatalf("validated assets were not passed to the reset: err=%v archives=%v query=%v", err, o.archives, cap.calls)
				}
			} else if err == nil || len(o.targets) != 0 || len(cap.calls) != 0 {
				t.Fatalf("invalid assets reached destructive reset: err=%v resets=%v query=%v", err, o.targets, cap.calls)
			}
			if name == "invalid-declaration" && store.reads != 0 {
				t.Fatal("invalid declaration reached object storage")
			}
		})
	}
}

func TestRuntimeResetVerifiesNativeReadiness(t *testing.T) {
	failure := errors.New("native query rejected")
	for _, tt := range []struct {
		name     string
		strategy string
		resetErr error
		queryErr error
		want     []string
		wantErr  error
	}{
		{name: "native", want: []string{"reset", "query:chain-id"}},
		{name: "recreate", strategy: "recreate_runtime", want: []string{"query:chain-id"}},
		{name: "native-reset-failure", resetErr: failure, want: []string{"reset"}, wantErr: failure},
		{name: "recreate-failure", strategy: "recreate_runtime", resetErr: failure, wantErr: failure},
		{name: "native-not-ready", queryErr: failure, want: []string{"reset", "query:chain-id"}, wantErr: failure},
		{name: "recreated-not-ready", strategy: "recreate_runtime", queryErr: failure, want: []string{"query:chain-id"}, wantErr: failure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cap := &resetCapability{resetErr: tt.resetErr, queryErr: tt.queryErr}
			o := &resetOrchestrator{err: tt.resetErr}
			s := &Service{orchestrator: o, cfg: config.SandboxConfig{ReadyTimeoutSeconds: 2, ReadyPollIntervalSeconds: 1, ChainRPCTimeoutSeconds: 1}}
			plan := CreateSandboxPlan{WorkspaceRuntimeInstance: "workspace", Runtimes: []RuntimePlan{{InstanceCode: "workspace"}, {InstanceCode: "selected", Runtime: Runtime{AdapterSpec: AdapterSpec{
				Selftest: map[string]any{"query_target": "chain-id"}, CapabilityCommands: CapabilityCommandSet{ResetStrategy: tt.strategy},
			}}}}}
			err := s.resetRuntimeForPlan(context.Background(), plan, "selected", cap)
			if !errors.Is(err, tt.wantErr) || !slices.Equal(cap.calls, tt.want) {
				t.Fatalf("reset returned before native verification: err=%v calls=%v want=%v", err, cap.calls, tt.want)
			}
			if tt.strategy == "recreate_runtime" && !slices.Equal(o.targets, []string{"selected"}) {
				t.Fatalf("wrong runtime reset: %v", o.targets)
			}
			for _, deadline := range cap.deadlines {
				if deadline.IsZero() || !deadline.Equal(cap.deadlines[0]) {
					t.Fatalf("reset and verification must share one deadline: %v", cap.deadlines)
				}
			}
		})
	}
}

func TestRuntimeResetRejectsInvalidInputsBeforeExecution(t *testing.T) {
	for _, name := range []string{"missing-instance", "empty-target", "non-string-target", "invalid-timeout", "invalid-poll", "invalid-rpc-timeout", "canceled"} {
		t.Run(name, func(t *testing.T) {
			o := &resetOrchestrator{}
			cap := &resetCapability{}
			s := &Service{orchestrator: o, cfg: config.SandboxConfig{ReadyTimeoutSeconds: 2, ReadyPollIntervalSeconds: 1, ChainRPCTimeoutSeconds: 1}}
			plan := CreateSandboxPlan{Runtimes: []RuntimePlan{{InstanceCode: "chain", Runtime: Runtime{AdapterSpec: AdapterSpec{
				Selftest: map[string]any{"query_target": "chain-id"}, CapabilityCommands: CapabilityCommandSet{ResetStrategy: "recreate_runtime"},
			}}}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "missing-instance":
				plan.Runtimes = nil
			case "empty-target":
				plan.Runtimes[0].Runtime.AdapterSpec.Selftest["query_target"] = "  "
			case "non-string-target":
				plan.Runtimes[0].Runtime.AdapterSpec.Selftest["query_target"] = 12
			case "invalid-timeout":
				s.cfg.ReadyTimeoutSeconds = 0
			case "invalid-poll":
				s.cfg.ReadyPollIntervalSeconds = 0
			case "invalid-rpc-timeout":
				s.cfg.ChainRPCTimeoutSeconds = 0
			case "canceled":
				cancel()
			}
			if err := s.resetRuntimeForPlan(ctx, plan, "chain", cap); err == nil || len(o.targets) != 0 || len(cap.calls) != 0 {
				t.Fatalf("invalid reset performed work: err=%v targets=%v calls=%v", err, o.targets, cap.calls)
			}
		})
	}
}

func TestPluginResetUsesRegisteredCapability(t *testing.T) {
	s, tx, access := terminalRecheckFixture(t)
	var snapshot contracts.SandboxCompositionSnapshot
	if err := json.Unmarshal(tx.sandbox.CompositionSnapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	adapter := tx.runtime.AdapterSpec
	adapter.Selftest = map[string]any{"query_target": "chain-id"}
	raw, err := json.Marshal(adapter)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Runtimes[0].AdapterSpec = raw
	snapshot.Runtimes[0].AdapterLevel = RuntimeAdapterLevelPlugin
	snapshot.Runtimes[0].PluginRef = "registered-plugin"
	tx.runtime.PluginRef = "registered-plugin"
	snapshot.Digest, err = contracts.CanonicalSnapshotDigest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	tx.sandbox.CompositionDigest = snapshot.Digest
	tx.sandbox.CompositionSnapshot, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	cap := &resetCapability{}
	s.capabilities = map[string]ChainCapability{"registered-plugin": cap}
	s.cfg = config.SandboxConfig{ReadyTimeoutSeconds: 2, ReadyPollIntervalSeconds: 1, ChainRPCTimeoutSeconds: 1}
	if err := s.ChainReset(context.Background(), contracts.SandboxChainResetRequest{
		TenantID: access.TenantID, SandboxID: access.SandboxID, SourceRef: access.SourceRef, RuntimeInstance: "chain",
	}); err != nil {
		t.Fatalf("registered plugin reset rejected: %v", err)
	}
	if !slices.Equal(cap.calls, []string{"reset", "query:chain-id"}) {
		t.Fatalf("plugin reset skipped native verification: %v", cap.calls)
	}
}

func TestChainCommandRequiresJSONObject(t *testing.T) {
	for _, tt := range []struct {
		name   string
		output string
		valid  bool
	}{
		{name: "no-output"},
		{name: "whitespace", output: " \n\t"},
		{name: "null", output: "null"},
		{name: "array", output: "[]"},
		{name: "string", output: `"ok"`},
		{name: "truncated", output: `{"height":`},
		{name: "multiple-values", output: "{}{}"},
		{name: "empty-object", output: "{}", valid: true},
		{name: "query-result", output: "{\"height\":12}\n", valid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			o := &workspaceWriteOrchestrator{stdout: []byte(tt.output)}
			cap := execChainCapability{orchestrator: o}
			out, err := cap.Query(context.Background(), Sandbox{}, Runtime{AdapterSpec: AdapterSpec{
				CapabilityCommands: CapabilityCommandSet{Query: CapabilityCommandSpec{Command: []string{"native-query"}, ExecTarget: "node/runtime", TimeoutSeconds: 1}},
			}}, "chain-id")
			if (err == nil) != tt.valid || (tt.valid && out == nil) || o.execs != 1 {
				t.Fatalf("invalid native response accepted: out=%v err=%v execs=%d", out, err, o.execs)
			}
		})
	}
}

func TestChainCommandRejectsUnresolvedTimeout(t *testing.T) {
	for _, seconds := range []int32{0, -1} {
		o := &workspaceWriteOrchestrator{stdout: []byte("{}")}
		cap := execChainCapability{orchestrator: o}
		_, err := cap.Query(context.Background(), Sandbox{}, Runtime{AdapterSpec: AdapterSpec{
			CapabilityCommands: CapabilityCommandSet{Query: CapabilityCommandSpec{Command: []string{"native-query"}, ExecTarget: "node/runtime", TimeoutSeconds: seconds}},
		}}, "chain-id")
		if !errors.Is(err, apperr.ErrSandboxCapabilityUnavailable) || o.execs != 0 {
			t.Fatalf("unresolved timeout %d silently executed: err=%v execs=%d", seconds, err, o.execs)
		}
	}
}
