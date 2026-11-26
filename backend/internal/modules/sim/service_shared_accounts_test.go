// sim service_shared_accounts_test 文件验证共享仿真会话账号的跨模块身份边界。
package sim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// simIdentityStub 为共享账号校验提供最小身份契约桩。
type simIdentityStub struct {
	accounts []contracts.AccountInfo
	err      error
}

// RequireTenantModules 不参与本组共享账号测试。
func (s simIdentityStub) RequireTenantModules(context.Context, int64, ...string) error { return nil }

// EnabledTenantModules 不参与本组共享账号测试。
func (s simIdentityStub) EnabledTenantModules(context.Context, int64) ([]string, error) {
	return contracts.TenantModules(), nil
}

// GetAccount 不参与本组测试。
func (s simIdentityStub) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{}, nil
}

// BatchGetAccounts 返回测试指定的账号摘要或错误。
func (s simIdentityStub) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return s.accounts, s.err
}

// HasRole 不参与本组测试。
func (s simIdentityStub) HasRole(context.Context, int64, string) (bool, error) {
	return false, nil
}

// ListClassStudents 不参与本组测试。
func (s simIdentityStub) ListClassStudents(context.Context, int64, int64, int, int) ([]contracts.AccountInfo, error) {
	return nil, nil
}

// TestValidateSessionSharedAccounts 固化共享账号必须真实存在且属于目标租户的规则。
func TestValidateSessionSharedAccounts(t *testing.T) {
	tests := []struct {
		name     string
		accounts []contracts.AccountInfo
		err      error
		wantErr  bool
	}{
		{name: "same tenant", accounts: []contracts.AccountInfo{{AccountID: 11, TenantID: 7}, {AccountID: 12, TenantID: 7}}},
		{name: "missing account", accounts: []contracts.AccountInfo{{AccountID: 11, TenantID: 7}}, wantErr: true},
		{name: "cross tenant", accounts: []contracts.AccountInfo{{AccountID: 11, TenantID: 7}, {AccountID: 12, TenantID: 8}}, wantErr: true},
		{name: "identity failure", err: errors.New("identity unavailable"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{identity: simIdentityStub{accounts: tt.accounts, err: tt.err}}
			err := svc.validateSessionSharedAccounts(context.Background(), 7, []int64{11, 12})
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateSessionSharedAccounts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConcurrentIsolatedSessionCreationHonorsTenantQuota(t *testing.T) {
	store := &simQuotaStore{unlockedReadsReady: make(chan struct{}), pkg: Package{ID: 9, Code: "teacher_7__sample", Version: "1.0.0", Compute: ComputeIsolated, BackendAdapter: "runner", BackendConfig: map[string]any{}, Status: PackageStatusPublished}}
	adapter := &simTestBackendAdapter{}
	ids, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, ids: ids, identity: simIdentityStub{}, audit: simTestAudit{}, backends: BackendRegistry{"runner": adapter}, maxIsolatedSessionsPerTenant: 1}
	ctx, err := auth.WithServiceIdentity(context.Background(), 7, "experiment:2026:instance:55")
	if err != nil {
		t.Fatal(err)
	}
	ctx = response.WithTrace(ctx, "sim-quota-test")
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			<-start
			_, err := service.CreateSession(ctx, contracts.SimCreateSessionRequest{TenantID: 7, PackageCode: store.pkg.Code, Version: store.pkg.Version, OwnerAccountID: 11, SourceRef: "experiment:2026:instance:55", ScopeRef: fmt.Sprintf("experiment:2026:instance:55:run:%d", i+1)})
			results <- err
		}(i)
	}
	close(start)
	var succeeded, rejected int
	for i := 0; i < 2; i++ {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, apperr.ErrSimBackendComputeUnavailable):
			rejected++
		default:
			t.Fatalf("unexpected session creation error: %v", err)
		}
	}
	if succeeded != 1 || rejected != 1 || store.sessions != 1 || store.lockCalls != 2 || !store.countedUnderLock {
		t.Fatalf("quota result: success=%d rejected=%d sessions=%d locks=%d count_locked=%v", succeeded, rejected, store.sessions, store.lockCalls, store.countedUnderLock)
	}
}

type simQuotaStore struct {
	Store
	TxStore
	mu                 sync.Mutex
	pkg                Package
	unlockedReadsReady chan struct{}
	unlockedReads      int
	sessions           int
	lockCalls          int
	countedUnderLock   bool
	sessionWrites      int
}

type simQuotaTx struct {
	TxStore
	store  *simQuotaStore
	locked bool
}

func (s *simQuotaStore) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, &simQuotaTx{store: s})
}

func (s *simQuotaStore) TenantTx(ctx context.Context, _ int64, fn func(context.Context, TxStore) error) error {
	tx := &simQuotaTx{store: s}
	defer func() {
		if tx.locked {
			s.mu.Unlock()
		}
	}()
	return fn(ctx, tx)
}

func (tx *simQuotaTx) GetPackageByCodeVersion(context.Context, string, string) (Package, error) {
	return tx.store.pkg, nil
}

func (tx *simQuotaTx) LockIsolatedSessionCreation(context.Context, int64) error {
	tx.store.mu.Lock()
	tx.locked = true
	tx.store.lockCalls++
	return nil
}

func (tx *simQuotaTx) CountActiveIsolatedSessions(context.Context, int64) (int64, error) {
	if !tx.locked {
		tx.store.mu.Lock()
		count := tx.store.sessions
		tx.store.unlockedReads++
		if tx.store.unlockedReads == 2 {
			close(tx.store.unlockedReadsReady)
		}
		tx.store.mu.Unlock()
		<-tx.store.unlockedReadsReady
		return int64(count), nil
	}
	tx.store.countedUnderLock = tx.locked
	return int64(tx.store.sessions), nil
}

func (tx *simQuotaTx) CreateSession(_ context.Context, session Session) (Session, error) {
	if !tx.locked {
		tx.store.mu.Lock()
		defer tx.store.mu.Unlock()
	}
	tx.store.sessions++
	tx.store.sessionWrites++
	session.Status = SessionAvailable
	return session, nil
}

type simTestBackendAdapter struct {
	BackendAdapter
	released []int64
	failures map[int64]int
}

func (*simTestBackendAdapter) Descriptor() BackendAdapterDescriptor {
	return BackendAdapterDescriptor{Code: "runner", Name: "运行器", Protocol: "stdio-json"}
}

func (*simTestBackendAdapter) ValidateConfig(map[string]any) error { return nil }

func (*simTestBackendAdapter) Serve(context.Context, SessionWithPackage, *ExecutionBundle, BackendConn) error {
	return nil
}

func (*simTestBackendAdapter) Preview(context.Context, Package, int64, *ExecutionBundle, int) (PreviewResult, error) {
	return PreviewResult{}, nil
}

func (a *simTestBackendAdapter) Release(_ context.Context, session SessionWithPackage) error {
	a.released = append(a.released, session.ID)
	if a.failures[session.ID] > 0 {
		a.failures[session.ID]--
		return fmt.Errorf("injected release failure for %d", session.ID)
	}
	return nil
}

type simTestAudit struct{ audit.Writer }

type simControlBus struct {
	handlers []eventbus.Handler
}

func (*simControlBus) PublishFanout(context.Context, string, any) error { return nil }
func (b *simControlBus) SubscribeFanout(_ string, handler eventbus.Handler) (eventbus.Subscription, error) {
	b.handlers = append(b.handlers, handler)
	return nil, nil
}

func (simTestAudit) Write(context.Context, audit.Entry) error { return nil }

var _ contracts.IdentityService = simIdentityStub{}

func TestArchivedIsolatedSessionCleanupCanResumeAfterReleaseFailure(t *testing.T) {
	store := &simCleanupStore{sessions: map[int64]SessionWithPackage{
		91: {Session: Session{ID: 91, TenantID: 7, SourceRef: "experiment:2026:instance:55", ScopeRef: "experiment:2026:instance:55:run:1", Compute: ComputeIsolated, Status: SessionArchived}, BackendAdapter: "runner"},
		92: {Session: Session{ID: 92, TenantID: 7, SourceRef: "experiment:2026:instance:55", ScopeRef: "experiment:2026:instance:55:run:1", Compute: ComputeIsolated, Status: SessionArchived}, BackendAdapter: "runner"},
	}}
	adapter := &simTestBackendAdapter{failures: map[int64]int{91: 1}}
	service := &Service{store: store, audit: simTestAudit{}, bus: &simControlBus{}, backends: BackendRegistry{"runner": adapter}}
	ctx, err := auth.WithServiceIdentity(context.Background(), 7, "experiment:2026:instance:55")
	if err != nil {
		t.Fatal(err)
	}
	ctx = response.WithTrace(ctx, "sim-recycle-test")
	req := contracts.SimRecycleRequest{TenantID: 7, SourceRef: "experiment:2026:instance:55", ScopeRef: "experiment:2026:instance:55:run:1"}
	if err := service.RecycleByScopeRef(ctx, req); err == nil {
		t.Fatal("first release should preserve its failure")
	}
	if !slices.Equal(adapter.released, []int64{91, 92}) {
		t.Fatalf("release stopped after first failure: %v", adapter.released)
	}
	if err := service.RecycleByScopeRef(ctx, req); err != nil {
		t.Fatalf("retry did not finish archived resources: %v", err)
	}
	if !slices.Equal(adapter.released, []int64{91, 92, 91, 92}) {
		t.Fatalf("retry did not revisit every archived resource: %v", adapter.released)
	}
}

func TestArchivedSingleSessionCleanupCanResumeAfterReleaseFailure(t *testing.T) {
	store := &simCleanupStore{sessions: map[int64]SessionWithPackage{91: {Session: Session{ID: 91, TenantID: 7, SourceRef: "experiment:2026:instance:55", ScopeRef: "experiment:2026:instance:55:run:1", Compute: ComputeIsolated, Status: SessionArchived}, BackendAdapter: "runner"}}}
	adapter := &simTestBackendAdapter{failures: map[int64]int{91: 1}}
	service := &Service{store: store, audit: simTestAudit{}, bus: &simControlBus{}, backends: BackendRegistry{"runner": adapter}}
	ctx, err := auth.WithServiceIdentity(context.Background(), 7, "experiment:2026:instance:55")
	if err != nil {
		t.Fatal(err)
	}
	ctx = response.WithTrace(ctx, "sim-destroy-test")
	req := contracts.SimDestroySessionRequest{TenantID: 7, SessionID: 91, SourceRef: "experiment:2026:instance:55"}
	if err := service.DestroySession(ctx, req); err == nil {
		t.Fatal("first release should fail")
	}
	if err := service.DestroySession(ctx, req); err != nil {
		t.Fatalf("archived session cleanup did not retry: %v", err)
	}
	if store.statusUpdates != 0 || len(adapter.released) != 2 {
		t.Fatalf("retry duplicated archive transition or skipped release: updates=%d releases=%v", store.statusUpdates, adapter.released)
	}
}

func TestConcurrentDuplicateActionsAreIdempotent(t *testing.T) {
	store := &simActionStore{lookupBarrier: make(chan struct{})}
	ids, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, identity: simIdentityStub{}, ids: ids}
	requests := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := service.ReportAction(context.Background(), 7, 11, 91, ReportActionRequest{Seq: 1, AtTick: 0, EventType: "select", Payload: map[string]any{}})
			requests <- err
		}()
	}
	var failures []error
	for range 2 {
		if err := <-requests; err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) != 0 || len(store.actions) != 1 || store.lockCalls != 2 {
		t.Fatalf("concurrent duplicate result: errors=%v actions=%v locks=%d", failures, store.actions, store.lockCalls)
	}
}

type simActionStore struct {
	Store
	mu             sync.Mutex
	rowLock        sync.Mutex
	lookupBarrier  chan struct{}
	lookupCount    int
	lockCalls      int
	actions        map[int32]Action
	compute        int16
	status         int16
	shares         int
	shareErrors    []error
	scaleLimit     map[string]any
	backendAdapter string
}

type simActionTx struct {
	TxStore
	store  *simActionStore
	locked bool
}

type shareArchiveStore struct {
	Store
	TxStore
	locked bool
	shares int
}

func (s *shareArchiveStore) TenantTx(ctx context.Context, _ int64, run func(context.Context, TxStore) error) error {
	return run(ctx, s)
}
func (s *shareArchiveStore) LockSessionActions(context.Context, int64, int64) error {
	s.locked = true
	return nil
}
func (s *shareArchiveStore) GetSession(context.Context, int64, int64) (Session, error) {
	status := SessionAvailable
	if s.locked {
		status = SessionArchived
	}
	return Session{ID: 91, TenantID: 7, OwnerAccountID: 11, Status: status}, nil
}
func (s *shareArchiveStore) CreateShare(_ context.Context, share Share) (Share, error) {
	s.shares++
	return share, nil
}

func TestShareCreationReadsStateAfterConcurrentArchive(t *testing.T) {
	store := &shareArchiveStore{}
	ids, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, ids: ids, identity: simIdentityStub{}}
	_, err = service.ShareSession(context.Background(), 7, 11, 91, time.Time{})
	if !errors.Is(err, apperr.ErrSimSessionStateInvalid) || store.shares != 0 {
		t.Fatalf("share creation escaped archive serialization: error=%v shares=%d locked=%v", err, store.shares, store.locked)
	}
}

func (s *simActionStore) TenantTx(ctx context.Context, _ int64, fn func(context.Context, TxStore) error) error {
	tx := &simActionTx{store: s}
	defer func() {
		if tx.locked {
			s.rowLock.Unlock()
		}
	}()
	return fn(ctx, tx)
}

func (tx *simActionTx) LockSessionActions(context.Context, int64, int64) error {
	tx.store.rowLock.Lock()
	tx.locked = true
	tx.store.lockCalls++
	return nil
}

func (tx *simActionTx) GetSessionWithPackage(context.Context, int64, int64) (SessionWithPackage, error) {
	status := tx.store.status
	if status == 0 {
		status = SessionAvailable
	}
	scale := tx.store.scaleLimit
	if scale == nil {
		scale = map[string]any{"nodes": 50, "max_tick": 5000, "max_events": 10000}
	}
	return SessionWithPackage{Session: Session{ID: 91, TenantID: 7, OwnerAccountID: 11, Status: status, Compute: tx.store.compute}, BackendAdapter: tx.store.backendAdapter, ScaleLimit: scale, InteractionSchema: InteractionSchema{Events: map[string]InteractionEventSchema{"select": {Kind: "event", Target: "none"}}}}, nil
}

func TestBrowserActionsStayWithinReplayLimits(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		tick, maxTick, maxEvents int32
		allowed                  bool
	}{
		{name: "tick limit", tick: 3, maxTick: 2, maxEvents: 10},
		{name: "flattened event limit", tick: 3, maxTick: 10, maxEvents: 3},
		{name: "event boundary", tick: 2, maxTick: 10, maxEvents: 3, allowed: true},
		{name: "user action at maximum tick", tick: 2, maxTick: 2, maxEvents: 10, allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &simActionStore{scaleLimit: map[string]any{"nodes": 50, "max_tick": tc.maxTick, "max_events": tc.maxEvents}}
			ids, err := snowflake.NewNode(1)
			if err != nil {
				t.Fatal(err)
			}
			service := &Service{store: store, ids: ids, identity: simIdentityStub{}}
			_, err = service.ReportAction(context.Background(), 7, 11, 91, ReportActionRequest{Seq: 1, AtTick: tc.tick, EventType: "select", Payload: map[string]any{}})
			if tc.allowed {
				if err != nil || len(store.actions) != 1 {
					t.Fatalf("valid boundary rejected: error=%v actions=%v", err, store.actions)
				}
			} else if !errors.Is(err, apperr.ErrSimActionSeqInvalid) || len(store.actions) != 0 {
				t.Fatalf("unreplayable action accepted: error=%v actions=%v", err, store.actions)
			}
		})
	}
}

func (tx *simActionTx) CreateShare(_ context.Context, share Share) (Share, error) {
	tx.store.shares++
	if tx.store.shares <= len(tx.store.shareErrors) && tx.store.shareErrors[tx.store.shares-1] != nil {
		return Share{}, tx.store.shareErrors[tx.store.shares-1]
	}
	return share, nil
}

func TestShareCreationRetriesOnlySkippedCodeCollisions(t *testing.T) {
	primaryKey := &pgconn.PgError{Code: "23505", ConstraintName: "sim_share_pkey"}
	for _, tc := range []struct {
		name     string
		failures []error
		attempts int
		want     error
	}{
		{name: "first code succeeds", attempts: 1},
		{name: "skipped code collision", failures: []error{pgx.ErrNoRows}, attempts: 2},
		{name: "collision budget exhausted", failures: []error{pgx.ErrNoRows, pgx.ErrNoRows, pgx.ErrNoRows, pgx.ErrNoRows, pgx.ErrNoRows}, attempts: 5, want: pgx.ErrNoRows},
		{name: "primary key is not a code collision", failures: []error{primaryKey}, attempts: 1, want: primaryKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &simActionStore{shareErrors: tc.failures}
			ids, err := snowflake.NewNode(1)
			if err != nil {
				t.Fatal(err)
			}
			service := &Service{store: store, ids: ids, identity: simIdentityStub{}}
			result, err := service.ShareSession(context.Background(), 7, 11, 91, time.Time{})
			if store.shares != tc.attempts || !errors.Is(err, tc.want) {
				t.Fatalf("wrong collision handling: error=%v attempts=%d want=%v/%d", err, store.shares, tc.want, tc.attempts)
			}
			if tc.want == nil && (result.Code == "" || result.Status != "active") || tc.want != nil && result.Code != "" {
				t.Fatalf("share result does not match persistence: %+v", result)
			}
		})
	}
}

func TestSessionCreationWritesAvailableOnce(t *testing.T) {
	store := &simQuotaStore{pkg: Package{ID: 9, Code: "builtin__sample", Version: "1.0.0", Compute: ComputeBrowser, Status: PackageStatusPublished}}
	ids, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, ids: ids, identity: simIdentityStub{}, audit: simTestAudit{}}
	ctx, err := auth.WithServiceIdentity(context.Background(), 7, "experiment:2026:instance:55")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateSession(ctx, contracts.SimCreateSessionRequest{TenantID: 7, PackageCode: store.pkg.Code, Version: store.pkg.Version, OwnerAccountID: 11, SourceRef: "experiment:2026:instance:55", ScopeRef: "experiment:2026:instance:55:run:1"})
	if err != nil || store.sessions != 1 || store.sessionWrites != 1 {
		t.Fatalf("creation must insert the available session once: error=%v inserts=%d writes=%d", err, store.sessions, store.sessionWrites)
	}
}

func TestSessionLifecycleGuards(t *testing.T) {
	// 按持久化值检查入口；归档和非法值都不能执行或创建分享。
	for _, status := range []int16{1, 2, 3, 4, 5, 6, -1} {
		for _, operation := range []string{"action", "stream", "share"} {
			t.Run(fmt.Sprintf("status=%d/%s", status, operation), func(t *testing.T) {
				store := &simActionStore{status: status, actions: map[int32]Action{1: {Seq: 1}}}
				ids, err := snowflake.NewNode(1)
				if err != nil {
					t.Fatal(err)
				}
				service := &Service{store: store, ids: ids, identity: simIdentityStub{}}
				switch operation {
				case "action":
					_, err = service.ReportAction(context.Background(), 7, 11, 91, ReportActionRequest{Seq: 2, EventType: "select", Payload: map[string]any{}})
				case "stream":
					conn := &backendValidatedConn{svc: service, session: SessionWithPackage{Session: Session{ID: 91, Status: SessionAvailable}}, tenantID: 7, accountID: 11}
					_, err = conn.LoadReplay(context.Background())
				case "share":
					_, err = service.ShareSession(context.Background(), 7, 11, 91, time.Time{})
				}
				if status == 1 {
					if err != nil {
						t.Fatalf("available session rejected: %v", err)
					}
				} else if !errors.Is(err, apperr.ErrSimSessionStateInvalid) || len(store.actions) != 1 || store.shares != 0 {
					t.Fatalf("non-available session accepted: error=%v actions=%v shares=%d", err, store.actions, store.shares)
				}
			})
		}
	}
}

func TestArchivedSessionRemainsReplayable(t *testing.T) {
	store := &simActionStore{status: 2, actions: map[int32]Action{1: {Seq: 1, EventType: "select", Payload: map[string]any{}}}}
	service := &Service{store: store, identity: simIdentityStub{}}
	replay, err := service.GetReplayForUser(context.Background(), 7, 11, 91)
	if err != nil || len(replay.Actions) != 1 {
		t.Fatalf("archiving destroyed the replay: error=%v replay=%+v", err, replay)
	}
}

func TestReplayProjectionDeclaresReadOnly(t *testing.T) {
	for _, status := range []int16{SessionAvailable, SessionArchived} {
		for _, public := range []bool{false, true} {
			session := SessionWithPackage{Session: Session{Status: status}}
			value := replayToResponse(session, nil)
			if public {
				value = replayToPublicResponse(session, nil)
			}
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}
			if payload["read_only"] != (public || status == SessionArchived) {
				t.Errorf("read-only capability missing or wrong: status=%d public=%v response=%s", status, public, data)
			}
		}
	}
}

func TestArchivedReplayCannotAppendActions(t *testing.T) {
	store := &simActionStore{status: SessionArchived, compute: ComputeIsolated, actions: map[int32]Action{1: {Seq: 1}}}
	service := &Service{store: store, identity: simIdentityStub{}}
	conn := &backendValidatedConn{svc: service, session: SessionWithPackage{Session: Session{ID: 91, Status: SessionArchived}}, tenantID: 7, accountID: 11}
	actions, err := conn.LoadReplay(context.Background())
	if err != nil || len(actions) != 1 {
		t.Fatalf("archived replay unavailable: error=%v actions=%v", err, actions)
	}
	err = conn.RecordExecuted(context.Background(), BackendCommand{Kind: BackendCommandEvent, Event: BackendEvent{EventType: "select"}})
	if !errors.Is(err, apperr.ErrSimSessionStateInvalid) || len(store.actions) != 1 {
		t.Fatalf("read-only stream appended an action: error=%v actions=%v", err, store.actions)
	}
}

func (tx *simActionTx) GetSession(ctx context.Context, tenantID, id int64) (Session, error) {
	session, err := tx.GetSessionWithPackage(ctx, tenantID, id)
	return session.Session, err
}

func (tx *simActionTx) ListActions(context.Context, int64, int64) ([]Action, error) {
	tx.store.mu.Lock()
	defer tx.store.mu.Unlock()
	var actions []Action
	for _, action := range tx.store.actions {
		actions = append(actions, action)
	}
	slices.SortFunc(actions, func(a, b Action) int { return int(a.Seq - b.Seq) })
	return actions, nil
}

func (tx *simActionTx) GetActionBySeq(_ context.Context, _, _ int64, seq int32) (Action, error) {
	tx.store.mu.Lock()
	action, ok := tx.store.actions[seq]
	tx.store.mu.Unlock()
	if !tx.locked {
		tx.store.mu.Lock()
		tx.store.lookupCount++
		if tx.store.lookupCount == 2 {
			close(tx.store.lookupBarrier)
		}
		tx.store.mu.Unlock()
		<-tx.store.lookupBarrier
	}
	if ok {
		return action, nil
	}
	return Action{}, pgx.ErrNoRows
}

func (tx *simActionTx) GetLastAction(ctx context.Context, tenantID, sessionID int64) (Action, error) {
	tx.store.mu.Lock()
	defer tx.store.mu.Unlock()
	var last Action
	for _, action := range tx.store.actions {
		if action.Seq > last.Seq {
			last = action
		}
	}
	if last.Seq == 0 {
		return Action{}, pgx.ErrNoRows
	}
	return last, nil
}

func TestIsolatedExecutionRejectsStaleHistory(t *testing.T) {
	store := &simActionStore{compute: ComputeIsolated}
	ids, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, identity: simIdentityStub{}, ids: ids}
	newConn := func() *backendValidatedConn {
		conn := &backendValidatedConn{svc: service, session: SessionWithPackage{Session: Session{ID: 91, Status: SessionAvailable}}, tenantID: 7, accountID: 11}
		if _, err := conn.LoadReplay(context.Background()); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	old, current := newConn(), newConn()
	command := BackendCommand{Kind: BackendCommandEvent, Event: BackendEvent{EventType: "select", Payload: map[string]any{}}}
	if err := current.RecordExecuted(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if err := old.RecordExecuted(context.Background(), command); !errors.Is(err, apperr.ErrSimActionSeqInvalid) {
		t.Fatalf("stale runner appended an event from obsolete history: %v", err)
	}
	if len(store.actions) != 1 {
		t.Fatalf("stale event entered replay: %v", store.actions)
	}
	if err := current.RecordExecuted(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if len(store.actions) != 2 {
		t.Fatalf("current runner failed to advance: %v", store.actions)
	}
}

func TestIsolatedActionsCannotBypassExecutionOverHTTP(t *testing.T) {
	store := &simActionStore{compute: ComputeIsolated}
	ids, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, identity: simIdentityStub{}, ids: ids}
	_, err = service.ReportAction(context.Background(), 7, 11, 91, ReportActionRequest{Seq: 1, EventType: "select", Payload: map[string]any{}})
	if err == nil || len(store.actions) != 0 {
		t.Fatalf("unexecuted isolated action persisted: error=%v actions=%v", err, store.actions)
	}
}

func TestReplayActionsCannotMoveBackwardInTime(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		t.Run(fmt.Sprint("isolated=", isolated), func(t *testing.T) {
			store := &simActionStore{actions: map[int32]Action{1: {Seq: 1, AtTick: 10}}}
			ids, err := snowflake.NewNode(1)
			if err != nil {
				t.Fatal(err)
			}
			service := &Service{store: store, identity: simIdentityStub{}, ids: ids}
			if isolated {
				conn := &backendValidatedConn{svc: service, session: SessionWithPackage{Session: Session{ID: 91, Status: SessionAvailable}}, tenantID: 7, accountID: 11, tick: 9, lastSeq: 1}
				err = conn.RecordExecuted(context.Background(), BackendCommand{Kind: BackendCommandEvent, Event: BackendEvent{EventType: "select", Payload: map[string]any{}}})
			} else {
				_, err = service.ReportAction(context.Background(), 7, 11, 91, ReportActionRequest{Seq: 2, AtTick: 9, EventType: "select", Payload: map[string]any{}})
			}
			if !errors.Is(err, apperr.ErrSimActionSeqInvalid) || len(store.actions) != 1 {
				t.Fatalf("backward timestamp entered deterministic replay: error=%v actions=%v", err, store.actions)
			}
		})
	}
}

func (tx *simActionTx) CreateAction(_ context.Context, action Action) (Action, error) {
	tx.store.mu.Lock()
	defer tx.store.mu.Unlock()
	if tx.store.actions == nil {
		tx.store.actions = make(map[int32]Action)
	}
	if _, exists := tx.store.actions[action.Seq]; exists {
		return Action{}, errors.New("duplicate action sequence")
	}
	tx.store.actions[action.Seq] = action
	return action, nil
}

type simCleanupStore struct {
	Store
	TxStore
	sessions      map[int64]SessionWithPackage
	statusUpdates int
}

func (s *simCleanupStore) LockSessionActions(context.Context, int64, int64) error {
	return nil
}

func (s *simCleanupStore) TenantTx(ctx context.Context, _ int64, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s)
}

func (s *simCleanupStore) GetSessionWithPackage(_ context.Context, _, id int64) (SessionWithPackage, error) {
	return s.sessions[id], nil
}

func (s *simCleanupStore) GetSession(_ context.Context, _, id int64) (Session, error) {
	return s.sessions[id].Session, nil
}

func (s *simCleanupStore) ArchiveSession(_ context.Context, _, id int64) (Session, error) {
	s.statusUpdates++
	session := s.sessions[id]
	session.Status = SessionArchived
	s.sessions[id] = session
	return session.Session, nil
}

func (s *simCleanupStore) ArchiveSessionsByScopeRef(_ context.Context, tenantID int64, scopeRef, sourceRef string) ([]Session, error) {
	ids := make([]int64, 0)
	for id, session := range s.sessions {
		if session.TenantID == tenantID && session.ScopeRef == scopeRef && session.SourceRef == sourceRef {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	items := make([]Session, 0, len(ids))
	for _, id := range ids {
		session := s.sessions[id]
		session.Status = SessionArchived
		s.sessions[id] = session
		items = append(items, session.Session)
	}
	return items, nil
}
