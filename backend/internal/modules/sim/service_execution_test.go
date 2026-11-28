package sim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	platformk8s "chaimir/internal/platform/k8s"
	platformredis "chaimir/internal/platform/redis"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	natsserver "github.com/nats-io/nats-server/v2/server"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

type executionLockFunc func(context.Context, string, time.Duration, func(context.Context) error) error

type previewRenewalBoundaryStore struct {
	Store
	TxStore
	entered chan struct{}
	release chan struct{}
	runDone chan struct{}
	calls   atomic.Int32
}

func (s *previewRenewalBoundaryStore) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s)
}

func (s *previewRenewalBoundaryStore) RenewPreviewLease(ctx context.Context, _, _ int64, _ string, _ time.Time) (bool, error) {
	if s.calls.Add(1) == 1 {
		return true, nil
	}
	close(s.entered)
	<-s.release
	return false, ctx.Err()
}

func (s *previewRenewalBoundaryStore) ReleasePreviewLease(context.Context, int64, int64, string) (int64, error) {
	<-s.runDone
	return 1, nil
}

func TestPreviewExecutionJoinsRenewal(t *testing.T) {
	store := &previewRenewalBoundaryStore{entered: make(chan struct{}), release: make(chan struct{}), runDone: make(chan struct{})}
	svc := &Service{store: store, previewMaxAttempts: 2}
	var runOnce, renewOnce sync.Once
	unblockRun := func() { runOnce.Do(func() { close(store.runDone) }) }
	unblockRenew := func() { renewOnce.Do(func() { close(store.release) }) }
	returned := make(chan struct{})
	done := make(chan error, 1)
	t.Cleanup(func() { unblockRun(); unblockRenew(); <-returned })
	go func() {
		defer close(returned)
		done <- svc.previewPackageWithLease(context.Background(), Package{ID: 2}, 3*time.Second)
	}()
	select {
	case <-store.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("renewal did not start")
	}
	unblockRun()
	select {
	case err := <-done:
		t.Fatalf("preview returned with renewal still active: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	unblockRenew()
	select {
	case err := <-done:
		if !errors.Is(err, apperr.ErrSimBackendComputeUnavailable) {
			t.Fatalf("preview failure lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("preview did not return after renewal stopped")
	}
}

func validateTeachingFrame(view map[string]any, scaleLimit map[string]any) error {
	if err := validateRuntimeProtocol("TeachingFrame", view); err != nil {
		return err
	}
	return validateTeachingFrameSemantics(view, scaleLimit)
}

func validTeachingFrame(t *testing.T) map[string]any {
	t.Helper()
	var frame map[string]any
	if err := json.Unmarshal([]byte(`{"summary":"初态","phase":{"id":"initial","title":"初态","intent":"observe","explanation":{"what":"观察","why":"验证"}},"focus":{"primary":["flow"]},"layout":{"primary":"flow"},"patterns":[{"id":"flow","mode":"pipeline","title":"流程","data":{"steps":[],"currentStepId":""}}]}`), &frame); err != nil {
		t.Fatal(err)
	}
	return frame
}

func TestTeachingFrameRejectsUnrenderableOutput(t *testing.T) {
	for _, name := range []string{"missing explanation", "missing pattern data", "invalid intent", "invalid focus", "oversized graph"} {
		t.Run(name, func(t *testing.T) {
			frame := validTeachingFrame(t)
			phase := frame["phase"].(map[string]any)
			pattern := frame["patterns"].([]any)[0].(map[string]any)
			switch name {
			case "missing explanation":
				delete(phase, "explanation")
			case "missing pattern data":
				delete(pattern, "data")
			case "invalid intent":
				phase["intent"] = "unknown"
			case "invalid focus":
				frame["focus"] = map[string]any{"primary": []any{7}}
			case "oversized graph":
				pattern["mode"] = "graph"
				pattern["data"] = map[string]any{"layout": "ring", "nodes": []any{map[string]any{"id": "a", "label": "a", "role": "node", "status": "idle"}, map[string]any{"id": "b", "label": "b", "role": "node", "status": "idle"}}, "edges": []any{}}
			}
			if err := validateTeachingFrame(frame, map[string]any{"nodes": 1, "max_tick": 8, "max_events": 8}); !errors.Is(err, apperr.ErrSimFrameInvalid) {
				t.Fatalf("unrenderable or oversized frame accepted: %v", err)
			}
		})
	}
}

func TestDescriptorRejectsUnrenderableOutput(t *testing.T) {
	descriptor := BackendDescriptor{Meta: map[string]any{"code": "teacher_11__fixture", "version": "1.0.0"}, Interactions: []map[string]any{{"id": "advance", "emits": "advance"}}}
	session := SessionWithPackage{PackageCode: "teacher_11__fixture", PackageVersion: "1.0.0"}
	session.InteractionSchema.Events = map[string]InteractionEventSchema{"advance": {}}
	if err := validateBackendDescriptor(descriptor, session.PackageCode, session.PackageVersion, session.InteractionSchema); !errors.Is(err, apperr.ErrSimFrameInvalid) {
		t.Fatalf("descriptor without narrative and interaction controls accepted: %v", err)
	}
}

func TestBackendSnapshotKeepsRequiredEmptyMaps(t *testing.T) {
	raw, err := json.Marshal(BackendSnapshot{InteractionAvailability: map[string]any{}, CheckpointResults: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"interaction_availability", "checkpoint_results"} {
		if _, ok := wire[field].(map[string]any); !ok {
			t.Fatalf("required object omitted from wire: %s", field)
		}
	}
}

func TestDescriptorControlsMatchReviewedManifest(t *testing.T) {
	raw, err := os.ReadFile("testdata/runtime_protocol.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"matching", "matching element and options", "kind", "target", "id", "params", "missing", "repeated"} {
		t.Run(name, func(t *testing.T) {
			var fixture struct {
				Descriptor BackendDescriptor `json:"descriptor"`
			}
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatal(err)
			}
			controls := fixture.Descriptor.Interactions
			pkg := Package{
				Code: "teacher_11__fixture", Version: "1.0.0",
				ScaleLimit: map[string]any{"nodes": 8, "max_tick": 8, "max_events": 8},
				InteractionSchema: InteractionSchema{Events: map[string]InteractionEventSchema{
					"advance": {InteractionID: "advance", Kind: "button", Target: "global", Params: []InteractionParam{}},
				}},
			}
			switch name {
			case "matching element and options":
				controls[0]["kind"] = "select-element"
				controls[0]["target"] = "element"
				controls[0]["elementFilter"] = "node"
				controls[0]["params"] = []any{map[string]any{
					"name": "size", "label": "规模", "type": "select", "default": 1, "required": true,
					"options": []any{map[string]any{"label": "一个", "value": 1}},
				}}
				pkg.InteractionSchema.Events["advance"] = InteractionEventSchema{
					InteractionID: "advance", Kind: "select-element", Target: "element",
					Params: []InteractionParam{{Name: "size", Type: "select", Required: true, Options: []string{"1"}}},
				}
			case "kind":
				controls[0]["kind"] = "hold"
			case "target":
				controls[0]["target"] = "element"
				controls[0]["elementFilter"] = "node"
			case "id":
				controls[0]["id"] = "other"
			case "params":
				controls[0]["params"] = []any{map[string]any{"name": "amount", "label": "数量", "type": "number", "default": 1}}
			case "missing":
				pkg.InteractionSchema.Events["inspect"] = InteractionEventSchema{InteractionID: "inspect", Kind: "button", Target: "global"}
			case "repeated":
				fixture.Descriptor.Interactions = append(controls, controls[0])
				pkg.InteractionSchema.Events["inspect"] = InteractionEventSchema{InteractionID: "inspect", Kind: "button", Target: "global"}
			}
			response, err := json.Marshal(runnerVerifyResponse{
				OK: true, Descriptor: fixture.Descriptor, Determinism: validationPassed,
				Frames: []BackendSnapshot{{
					Tick: 0, View: validTeachingFrame(t),
					State: map[string]any{
						"tick": 0, "phase": "initial", "metrics": map[string]any{}, "checkpointValues": map[string]any{},
						"explanation": map[string]any{"title": "初态", "effect": "观察", "reason": "验证", "defaultDurationMs": 1000},
					},
					InteractionAvailability: map[string]any{}, CheckpointResults: map[string]any{},
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			runtimeErr := validateBackendDescriptor(fixture.Descriptor, pkg.Code, pkg.Version, pkg.InteractionSchema)
			_, previewErr := decodePreviewResponse(response, 1, pkg)
			for _, err := range []error{runtimeErr, previewErr} {
				if strings.HasPrefix(name, "matching") && err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(name, "matching") && !errors.Is(err, apperr.ErrSimFrameInvalid) {
					t.Fatalf("descriptor differs from the reviewed controls: %v", err)
				}
			}
		})
	}
}

func TestRuntimeProtocolRenderingAndCapacity(t *testing.T) {
	raw, err := os.ReadFile("testdata/runtime_protocol.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Descriptor BackendDescriptor `json:"descriptor"`
		Snapshot   map[string]any    `json:"snapshot"`
		Patterns   []any             `json:"patterns"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	session := SessionWithPackage{PackageCode: "teacher_11__fixture", PackageVersion: "1.0.0", InteractionSchema: InteractionSchema{Events: map[string]InteractionEventSchema{"advance": {InteractionID: "advance", Kind: "button", Target: "global"}}}}
	if err := validateBackendDescriptor(fixture.Descriptor, session.PackageCode, session.PackageVersion, session.InteractionSchema); err != nil {
		t.Fatal(err)
	}
	for _, pattern := range fixture.Patterns {
		value := pattern.(map[string]any)
		t.Run(value["mode"].(string), func(t *testing.T) {
			frame := validTeachingFrame(t)
			frame["patterns"] = []any{pattern}
			if err := validateTeachingFrame(frame, map[string]any{"nodes": 8, "max_events": 8}); err != nil {
				t.Fatal(err)
			}
			if err := validateTeachingFrame(frame, map[string]any{"nodes": 0, "max_events": 0}); !errors.Is(err, apperr.ErrSimFrameInvalid) {
				t.Fatalf("capacity bypassed: %v", err)
			}
			data := value["data"]
			delete(value, "data")
			if err := validateTeachingFrame(frame, map[string]any{"nodes": 8, "max_events": 8}); !errors.Is(err, apperr.ErrSimFrameInvalid) {
				t.Fatalf("missing drawing data accepted: %v", err)
			}
			value["data"] = data
		})
	}
	snapshot := BackendSnapshot{Tick: 0, State: fixture.Snapshot["state"].(map[string]any), View: fixture.Snapshot["view"].(map[string]any), InteractionAvailability: fixture.Snapshot["interactionAvailability"].(map[string]any), CheckpointResults: fixture.Snapshot["checkpointResults"].(map[string]any)}
	limits := map[string]any{"nodes": 8, "max_tick": 8, "max_events": 8}
	if err := validateBackendSnapshot(snapshot, limits); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"_trace", "explanation", "checkpointResults", "interactionAvailability", "tick"} {
		t.Run(field, func(t *testing.T) {
			broken := snapshot
			state := make(map[string]any)
			for key, value := range snapshot.State {
				state[key] = value
			}
			broken.State = state
			switch field {
			case "_trace":
				state["_trace"] = map[string]any{"triggeredLines": "wrong", "variables": map[string]any{}}
			case "explanation":
				delete(state, "explanation")
			case "checkpointResults":
				broken.CheckpointResults = map[string]any{"done": map[string]any{"achieved": "wrong"}}
			case "interactionAvailability":
				broken.InteractionAvailability = map[string]any{"advance": "wrong"}
			case "tick":
				broken.Tick = 9
			}
			if err := validateBackendSnapshot(broken, limits); !errors.Is(err, apperr.ErrSimFrameInvalid) {
				t.Fatalf("unrenderable snapshot accepted: %v", err)
			}
		})
	}
}

func simK8sTestClient(t *testing.T, handler http.HandlerFunc) *platformk8s.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	kubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
	data := fmt.Sprintf(`{"apiVersion":"v1","kind":"Config","clusters":[{"name":"test","cluster":{"server":%q}}],"contexts":[{"name":"test","context":{"cluster":"test"}}],"current-context":"test"}`, server.URL)
	if err := os.WriteFile(kubeconfig, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	client, err := platformk8s.New(config.SandboxConfig{KubeconfigPath: kubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestIsolatedEnvironmentCapacityIncludesArchivedAndTerminating(t *testing.T) {
	for _, tenantID := range []int64{0, 7} {
		for _, status := range []int16{SessionAvailable, SessionArchived} {
			t.Run(fmt.Sprintf("tenant=%d/status=%d", tenantID, status), func(t *testing.T) {
				posts := 0
				client := simK8sTestClient(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Method == http.MethodGet && r.URL.Path == "/api/v1/namespaces" {
						selector, err := labels.Parse(r.URL.Query().Get("labelSelector"))
						if err != nil {
							t.Error(err)
						}
						existing := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "archived-replay", DeletionTimestamp: &metav1.Time{Time: time.Now()}, Labels: map[string]string{"chaimir.io/managed-by": "chaimir-backend", "chaimir.io/sim": "true", "chaimir.io/tenant-id": fmt.Sprint(tenantID), "chaimir.io/execution-scope": "90-replay"}}}
						list := corev1.NamespaceList{Items: []corev1.Namespace{}}
						if selector.Matches(labels.Set(existing.Labels)) {
							list.Items = append(list.Items, existing)
						}
						if err := json.NewEncoder(w).Encode(list); err != nil {
							t.Error(err)
						}
						return
					}
					if r.Method == http.MethodPost {
						posts++
					}
					w.WriteHeader(http.StatusServiceUnavailable)
				})
				adapter := &StdioJSONAdapter{k8s: client, cfg: config.SimBackendConfig{MaxConcurrentSessionsPerTenant: 1, SessionLeaseDurationSeconds: 60}, locker: executionLockFunc(func(ctx context.Context, _ string, _ time.Duration, run func(context.Context) error) error {
					return run(ctx)
				})}
				err := adapter.prepareSession(context.Background(), SessionWithPackage{Session: Session{ID: 91, TenantID: tenantID, Status: status}}, &stdioJSONExecution{namespace: metav1.ObjectMeta{Name: "sim-new-run"}})
				if !errors.Is(err, apperr.ErrSimBackendComputeUnavailable) || posts != 0 {
					t.Fatalf("existing replay bypassed environment capacity: error=%v creates=%d", err, posts)
				}
			})
		}
	}
}

func TestArchivedExecutionScopeDoesNotBlockArchiveRetry(t *testing.T) {
	var expectedScope string
	client := simK8sTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/namespaces" {
			t.Errorf("archive retry touched replay resources: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !strings.Contains(r.URL.Query().Get("labelSelector"), "chaimir.io/execution-scope="+expectedScope) {
			t.Errorf("wrong cleanup selector: %s", r.URL.RawQuery)
		}
		if err := json.NewEncoder(w).Encode(corev1.NamespaceList{Items: []corev1.Namespace{}}); err != nil {
			t.Error(err)
		}
	})
	held := ""
	stop := errors.New("stop before allocating replay environment")
	adapter := &StdioJSONAdapter{k8s: client, cfg: config.SimBackendConfig{SessionLeaseDurationSeconds: 60, PodReadyTimeoutSeconds: 1}, sandbox: config.SandboxConfig{ReadyPollIntervalSeconds: 1}}
	adapter.locker = executionLockFunc(func(ctx context.Context, key string, _ time.Duration, run func(context.Context) error) error {
		if held == "" {
			held = key
			return stop
		}
		if key == held {
			return platformredis.ErrLeaseHeld
		}
		return run(ctx)
	})
	session := SessionWithPackage{Session: Session{ID: 91, TenantID: 7, Status: SessionArchived}}
	expectedScope = "91-replay"
	if err := adapter.Serve(context.Background(), session, nil, nil); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	if held != "sim.execution:91-replay" {
		t.Errorf("archived replay acquired the live execution lease: %s", held)
	}
	expectedScope = "91"
	if err := adapter.Release(context.Background(), session); err != nil {
		t.Fatalf("archive retry waited for an unrelated replay lease: %v", err)
	}
}

func TestIsolatedEnvironmentAllocationIsSerializedAcrossReplicas(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	redisServer := miniredis.RunT(t)
	port, err := strconv.Atoi(redisServer.Port())
	if err != nil {
		t.Fatal(err)
	}
	countStarted, allowCount := make(chan struct{}), make(chan struct{})
	var firstCount sync.Once
	var resourcesMu sync.Mutex
	var resources []corev1.Namespace
	client := simK8sTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/namespaces" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Method == http.MethodGet {
			firstCount.Do(func() {
				close(countStarted)
				select {
				case <-allowCount:
				case <-r.Context().Done():
				}
			})
		}
		resourcesMu.Lock()
		defer resourcesMu.Unlock()
		var result any
		if r.Method == http.MethodGet {
			selector, err := labels.Parse(r.URL.Query().Get("labelSelector"))
			if err != nil {
				t.Error(err)
			}
			list := corev1.NamespaceList{Items: []corev1.Namespace{}}
			for _, namespace := range resources {
				if selector.Matches(labels.Set(namespace.Labels)) {
					list.Items = append(list.Items, namespace)
				}
			}
			result = list
		} else if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			var namespace corev1.Namespace
			if _, _, err := scheme.Codecs.UniversalDeserializer().Decode(body, nil, &namespace); err != nil {
				t.Error(err)
			}
			namespace.UID = types.UID(namespace.Name)
			resources = append(resources, namespace)
			result = namespace
		} else {
			t.Errorf("unexpected capacity request: %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			t.Error(err)
		}
	})
	adapters := make([]*StdioJSONAdapter, 2)
	for i := range adapters {
		locker, err := platformredis.New(ctx, config.RedisConfig{Host: redisServer.Host(), Port: port, PingTimeoutSeconds: 1})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := locker.Close(); err != nil {
				t.Error(err)
			}
		})
		adapters[i] = &StdioJSONAdapter{k8s: client, locker: locker, cfg: config.SimBackendConfig{MaxConcurrentSessionsPerTenant: 1, SessionLeaseDurationSeconds: 60, PodReadyTimeoutSeconds: 1}, sandbox: config.SandboxConfig{ReadyPollIntervalSeconds: 1}}
	}
	session := SessionWithPackage{Session: Session{ID: 91, TenantID: 7, Status: SessionArchived}}
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- adapters[0].prepareSession(ctx, session, &stdioJSONExecution{namespace: metav1.ObjectMeta{Name: "first-replay"}})
	}()
	select {
	case <-countStarted:
	case <-ctx.Done():
		t.Fatal("first replica did not check actual capacity")
	}
	if err := adapters[1].prepareSession(ctx, session, &stdioJSONExecution{namespace: metav1.ObjectMeta{Name: "racing-replay"}}); !errors.Is(err, platformredis.ErrLeaseHeld) {
		t.Errorf("second replica entered the same tenant allocation: %v", err)
	}
	close(allowCount)
	if err := <-firstResult; err == nil || errors.Is(err, apperr.ErrSimBackendComputeUnavailable) {
		t.Fatalf("first allocation did not reach the deliberately denied RBAC setup: %v", err)
	}
	if err := adapters[1].prepareSession(ctx, session, &stdioJSONExecution{namespace: metav1.ObjectMeta{Name: "later-replay"}}); !errors.Is(err, apperr.ErrSimBackendComputeUnavailable) {
		t.Fatalf("remaining environment no longer consumes capacity: %v", err)
	}
	session.TenantID = 8
	if err := adapters[1].prepareSession(ctx, session, &stdioJSONExecution{namespace: metav1.ObjectMeta{Name: "other-tenant"}}); err == nil || errors.Is(err, apperr.ErrSimBackendComputeUnavailable) {
		t.Fatalf("another tenant shared the first tenant's capacity: %v", err)
	}
	resourcesMu.Lock()
	defer resourcesMu.Unlock()
	if len(resources) != 2 || resources[0].Labels["chaimir.io/execution-scope"] != "91-replay" || resources[1].Labels["chaimir.io/tenant-id"] != "8" {
		t.Fatalf("wrong resource allocation or labels: %+v", resources)
	}
}

func TestExecutionCleanupRetainsLeaseAndPreservesBothErrors(t *testing.T) {
	var held atomic.Bool
	var deletes atomic.Int32
	client := simK8sTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !held.Load() && r.URL.Path != "/api/v1/namespaces" {
			t.Error("cleanup ran after releasing its lease")
		}
		if r.Method == http.MethodDelete {
			deletes.Add(1)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var result any = corev1.NamespaceList{Items: []corev1.Namespace{}}
		if r.URL.Path != "/api/v1/namespaces" {
			result = corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/"), UID: "current-uid"}}
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			t.Error(err)
		}
	})
	adapter := &StdioJSONAdapter{k8s: client, cfg: config.SimBackendConfig{NamespacePrefix: "sim-session-", SessionLeaseDurationSeconds: 60, PodReadyTimeoutSeconds: 1}}
	adapter.locker = executionLockFunc(func(ctx context.Context, _ string, _ time.Duration, run func(context.Context) error) error {
		held.Store(true)
		defer held.Store(false)
		return run(ctx)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runFailure := errors.New("execution failed")
	err := adapter.withExecution(ctx, "91", func(_ context.Context, execution *stdioJSONExecution) error {
		execution.namespace.UID = "current-uid"
		cancel()
		return runFailure
	})
	if !errors.Is(err, runFailure) || !strings.Contains(err.Error(), "删除后端仿真命名空间失败") || deletes.Load() != 1 || held.Load() {
		t.Fatalf("cancellation lost cleanup or an error: error=%v deletes=%d lease=%v", err, deletes.Load(), held.Load())
	}
}

type streamAdapter struct {
	simTestBackendAdapter
	serve func(context.Context, SessionWithPackage, BackendConn) error
}

func (a streamAdapter) Serve(ctx context.Context, session SessionWithPackage, _ *ExecutionBundle, conn BackendConn) error {
	return a.serve(ctx, session, conn)
}

func TestStreamModeAndFrameAreDerivedFromCurrentSession(t *testing.T) {
	for _, status := range []int16{SessionAvailable, SessionArchived} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			store := &simActionStore{status: status, compute: ComputeIsolated, backendAdapter: "fixture"}
			service := &Service{store: store, identity: simIdentityStub{}, backends: BackendRegistry{}}
			service.backends["fixture"] = &streamAdapter{serve: func(ctx context.Context, session SessionWithPackage, conn BackendConn) error {
				if session.Status != status {
					return fmt.Errorf("wrong stream mode: %d", session.Status)
				}
				if _, err := conn.LoadReplay(ctx); err != nil {
					return err
				}
				if err := conn.SendFrame(ctx, BackendStreamMessage{
					Type: backendStreamSnapshot, ReadOnly: status != SessionArchived,
					Snapshot: BackendSnapshot{
						State: map[string]any{"tick": 0, "phase": "initial", "explanation": map[string]any{"title": "初态", "effect": "观察", "reason": "验证", "defaultDurationMs": 1000}, "metrics": map[string]any{}, "checkpointValues": map[string]any{}},
						View:  validTeachingFrame(t), InteractionAvailability: map[string]any{}, CheckpointResults: map[string]any{},
					},
				}); err != nil {
					return err
				}
				_, err := conn.ReadCommand(ctx)
				return err
			}}
			hub, err := ws.NewHub(ws.NewOriginPolicy(nil, nil), ws.HubOptions{ReadTimeout: time.Second, WriteTimeout: time.Second, PingInterval: time.Second, ReadLimit: 1024})
			if err != nil {
				t.Fatal(err)
			}
			service.wsHub = hub
			result := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				result <- hub.ServeInteractive(w, r, func(socket *ws.Conn) error {
					return service.ServeBackendStream(socket.Context(), socket, 7, 11, 91)
				})
			}))
			defer server.Close()
			client, response, err := websocket.DefaultDialer.Dial(strings.Replace(server.URL, "http://", "ws://", 1), nil)
			if response != nil && response.Body != nil {
				if err := response.Body.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			}()
			if err := client.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			var frame BackendStreamMessage
			if err := client.ReadJSON(&frame); err != nil {
				t.Fatal(err)
			}
			if frame.ReadOnly != (status == SessionArchived) {
				t.Fatalf("adapter overrode authoritative mode: %+v", frame)
			}
			if err := client.WriteJSON(BackendClientMessage{Type: "step"}); err != nil {
				t.Fatal(err)
			}
			if err := <-result; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestArchivedReplaySeeksOnlyRecordedEvents(t *testing.T) {
	history := newRunnerHistory(map[string]any{"max_tick": 2, "max_events": 8})
	if err := history.seed([]Action{{Seq: 1, AtTick: 1, EventType: "select"}, {Seq: 2, AtTick: 1, EventType: "advance"}}); err != nil {
		t.Fatal(err)
	}
	recorded := append([]runnerEvent(nil), history.events...)
	history.reset()
	for _, tc := range []struct {
		command BackendCommandKind
		cursor  int
	}{
		{BackendCommandBack, 0}, {BackendCommandStep, 1}, {BackendCommandStep, 2},
		{BackendCommandBack, 1}, {BackendCommandStep, 2}, {BackendCommandStep, 3},
		{BackendCommandStep, 3}, {BackendCommandRestart, 0}, {BackendCommandStep, 1},
	} {
		if err := history.seekRecorded(recorded, tc.command); err != nil || len(history.events) != tc.cursor {
			t.Fatalf("seek %s: error=%v cursor=%d want=%d", tc.command, err, len(history.events), tc.cursor)
		}
		for i, event := range history.events {
			if event.Seq != recorded[i].Seq || event.Type != recorded[i].Type || event.AtTick != recorded[i].AtTick {
				t.Fatalf("replay changed event %d", i)
			}
		}
	}
	if err := history.seekRecorded(recorded, BackendCommandEvent); err == nil || len(history.events) != 1 {
		t.Fatalf("new event accepted by read-only replay: error=%v history=%+v", err, history)
	}
	history.reset()
	if err := history.seekRecorded([]runnerEvent{}, BackendCommandStep); err != nil || len(history.events) != 0 {
		t.Fatalf("empty replay fabricated an event: error=%v history=%+v", err, history)
	}
}

func TestArchivedStreamRejectsNewEventsAndChangedMode(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int16
		accountID int64
		command   string
		want      error
	}{
		{"step", SessionArchived, 11, "step", nil},
		{"back", SessionArchived, 11, "back", nil},
		{"restart", SessionArchived, 11, "restart", nil},
		{"new event", SessionArchived, 11, "event", apperr.ErrSimSessionStateInvalid},
		{"changed mode", SessionAvailable, 11, "step", apperr.ErrSimSessionStateInvalid},
		{"unauthorized", SessionArchived, 12, "step", apperr.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &simActionStore{status: tc.status}
			service := &Service{store: store, identity: simIdentityStub{}}
			hub, err := ws.NewHub(ws.NewOriginPolicy(nil, nil), ws.HubOptions{ReadTimeout: time.Second, WriteTimeout: time.Second, PingInterval: time.Second, ReadLimit: 1024})
			if err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := hub.ServeInteractive(w, r, func(socket *ws.Conn) error {
					conn := &backendValidatedConn{svc: service, conn: socket, session: SessionWithPackage{Session: Session{ID: 91, Status: SessionArchived}}, tenantID: 7, accountID: tc.accountID}
					_, err := conn.ReadCommand(socket.Context())
					result <- err
					return nil
				}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, response, err := websocket.DefaultDialer.Dial(strings.Replace(server.URL, "http://", "ws://", 1), nil)
			if response != nil && response.Body != nil {
				if err := response.Body.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			}()
			if err := client.WriteJSON(BackendClientMessage{Type: tc.command, EventType: "select"}); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if !errors.Is(err, tc.want) {
					t.Fatalf("command error=%v want=%v", err, tc.want)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("stream command did not finish")
			}
		})
	}
}

func TestIsolatedHistoryPreservesActionsBeyondTickLimit(t *testing.T) {
	scale := map[string]any{"max_tick": 2, "max_events": 8}
	history := newRunnerHistory(scale)
	actions := make([]Action, 6)
	for i := range actions {
		actions[i] = Action{Seq: int32(i + 1), EventType: "select"}
	}
	if err := history.seed(actions); err != nil {
		t.Fatal(err)
	}
	if history.length() != 6 || history.tick != 0 {
		t.Fatalf("valid user actions were silently truncated by the tick limit: length=%d tick=%d", history.length(), history.tick)
	}
}

func TestRunnerHistoryRejectsUnreplayableRecords(t *testing.T) {
	for _, tc := range []struct {
		name    string
		actions []Action
	}{
		{name: "first sequence gap", actions: []Action{{Seq: 2}}},
		{name: "duplicate sequence", actions: []Action{{Seq: 1}, {Seq: 1}}},
		{name: "backward tick", actions: []Action{{Seq: 1, AtTick: 1}, {Seq: 2, AtTick: 0}}},
		{name: "negative tick", actions: []Action{{Seq: 1, AtTick: -1}}},
		{name: "tick over limit", actions: []Action{{Seq: 1, AtTick: 3}}},
		{name: "event count over limit", actions: []Action{{Seq: 1, AtTick: 2}, {Seq: 2, AtTick: 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			history := newRunnerHistory(map[string]any{"max_tick": 2, "max_events": 3})
			if err := history.seed(tc.actions); err == nil {
				t.Fatalf("invalid history was restored or silently truncated: actions=%+v history=%+v", tc.actions, history)
			}
		})
	}
}

func TestRunnerHistoryIndependentLimitsAndConfirmation(t *testing.T) {
	history := newRunnerHistory(map[string]any{"max_tick": 2, "max_events": 5})
	command := BackendCommand{Kind: BackendCommandStep}
	tick, _, err := history.plan(command)
	if err != nil {
		t.Fatal(err)
	}
	if err := history.accept(tick, 0); err != nil || history.length() != 0 || history.seq != 1 {
		t.Fatalf("terminal no-op appended a phantom tick: error=%v history=%+v", err, history)
	}
	if err := history.accept(tick, 2); err == nil || history.length() != 0 {
		t.Fatalf("skipped tick accepted: error=%v history=%+v", err, history)
	}
	for i := int64(1); i <= 2; i++ {
		event, _, err := history.plan(command)
		if err != nil {
			t.Fatal(err)
		}
		if err := history.accept(event, i); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := history.plan(command); err == nil {
		t.Fatal("tick limit did not reject next step")
	}
	command = BackendCommand{Kind: BackendCommandEvent, Event: BackendEvent{EventType: "select"}}
	event, _, err := history.plan(command)
	if err != nil {
		t.Fatalf("tick limit blocked a user event: %v", err)
	}
	if err := history.accept(event, 3); err == nil || history.length() != 2 {
		t.Fatalf("user event changed the tick: error=%v history=%+v", err, history)
	}
	for range 3 {
		event, _, err := history.plan(command)
		if err != nil {
			t.Fatal(err)
		}
		if err := history.accept(event, 2); err != nil {
			t.Fatal(err)
		}
	}
	if history.length() != 5 {
		t.Fatalf("lost confirmed events: %+v", history)
	}
	if _, _, err := history.plan(command); err == nil {
		t.Fatal("event limit did not reject next event")
	}
}

func (f executionLockFunc) WithLease(ctx context.Context, key string, ttl time.Duration, run func(context.Context) error) error {
	return f(ctx, key, ttl, run)
}

func TestExecutionCleanupCannotDeleteLaterOwnersResources(t *testing.T) {
	for _, replaceUID := range []bool{false, true} {
		t.Run(fmt.Sprint("replaceUID=", replaceUID), func(t *testing.T) {
			resources := map[string]types.UID{"sim-old-run": "old-uid"}
			var resourcesMu sync.Mutex
			var deleted []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resourcesMu.Lock()
				defer resourcesMu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				name := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/")
				if r.URL.Path == "/api/v1/namespaces" {
					if !strings.Contains(r.URL.Query().Get("labelSelector"), "chaimir.io/execution-scope=91") {
						t.Error("cleanup escaped its session selector")
					}
					list := corev1.NamespaceList{TypeMeta: metav1.TypeMeta{Kind: "NamespaceList", APIVersion: "v1"}}
					for name, uid := range resources {
						list.Items = append(list.Items, corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid, Labels: map[string]string{"chaimir.io/managed-by": "chaimir-backend", "chaimir.io/sim": "true", "chaimir.io/execution-scope": "91"}}})
					}
					if err := json.NewEncoder(w).Encode(list); err != nil {
						t.Error(err)
					}
					return
				}
				uid, exists := resources[name]
				if r.Method == http.MethodDelete && exists {
					var options metav1.DeleteOptions
					data, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					if _, _, err := scheme.Codecs.UniversalDeserializer().Decode(data, nil, &options); err != nil {
						t.Error(err)
					}
					if options.Preconditions == nil || options.Preconditions.UID == nil || *options.Preconditions.UID != uid {
						w.WriteHeader(http.StatusConflict)
						if err := json.NewEncoder(w).Encode(metav1.Status{Status: "Failure", Reason: metav1.StatusReasonConflict, Code: http.StatusConflict}); err != nil {
							t.Error(err)
						}
						return
					}
					delete(resources, name)
					deleted = append(deleted, name)
					if err := json.NewEncoder(w).Encode(metav1.Status{Status: "Success"}); err != nil {
						t.Error(err)
					}
					return
				}
				if !exists {
					w.WriteHeader(http.StatusNotFound)
					if err := json.NewEncoder(w).Encode(metav1.Status{Status: "Failure", Reason: metav1.StatusReasonNotFound, Code: http.StatusNotFound}); err != nil {
						t.Error(err)
					}
					return
				}
				if err := json.NewEncoder(w).Encode(corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid, Labels: map[string]string{"chaimir.io/managed-by": "chaimir-backend", "chaimir.io/sim": "true", "chaimir.io/execution-scope": "91"}}}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			kubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
			data := fmt.Sprintf(`{"apiVersion":"v1","kind":"Config","clusters":[{"name":"test","cluster":{"server":%q}}],"contexts":[{"name":"test","context":{"cluster":"test"}}],"current-context":"test"}`, server.URL)
			if err := os.WriteFile(kubeconfig, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			client, err := platformk8s.New(config.SandboxConfig{KubeconfigPath: kubeconfig})
			if err != nil {
				t.Fatal(err)
			}
			adapter := &StdioJSONAdapter{k8s: client, cfg: config.SimBackendConfig{NamespacePrefix: "sim-session-", SessionLeaseDurationSeconds: 60, PodReadyTimeoutSeconds: 1}}
			adapter.locker = executionLockFunc(func(ctx context.Context, key string, _ time.Duration, run func(context.Context) error) error {
				if key != "sim.execution:91" {
					t.Fatalf("wrong lock scope: %s", key)
				}
				resourcesMu.Lock()
				resources["sim-new-run"] = "new-uid"
				if replaceUID {
					resources["sim-old-run"] = "replacement-uid"
				}
				resourcesMu.Unlock()
				return run(ctx)
			})
			ran := false
			err = adapter.withExecution(context.Background(), "91", func(context.Context, *stdioJSONExecution) error { ran = true; return nil })
			resourcesMu.Lock()
			defer resourcesMu.Unlock()
			if replaceUID {
				if err == nil || ran || len(deleted) != 0 {
					t.Fatalf("replacement resource was touched: error=%v ran=%v deleted=%v", err, ran, deleted)
				}
			} else if err != nil || !ran || len(deleted) != 1 || deleted[0] != "sim-old-run" {
				t.Fatalf("old resource cleanup failed: error=%v ran=%v deleted=%v", err, ran, deleted)
			}
			if resources["sim-new-run"] != "new-uid" {
				t.Fatal("stale cleanup deleted successor environment")
			}
		})
	}
}

type simSignalStore struct {
	Store
	TxStore
	mu      sync.RWMutex
	session Session
}

func (s *simSignalStore) TenantTx(ctx context.Context, _ int64, run func(context.Context, TxStore) error) error {
	return run(ctx, s)
}
func (s *simSignalStore) GetSession(context.Context, int64, int64) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session, nil
}
func (s *simSignalStore) set(session Session) { s.mu.Lock(); defer s.mu.Unlock(); s.session = session }

func TestSessionChangesCloseStreamsAcrossReplicasUsingCurrentState(t *testing.T) {
	server, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go server.Start()
	t.Cleanup(server.Shutdown)
	if !server.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS not ready")
	}
	cfg := config.NATSConfig{URL: server.ClientURL(), ReconnectWaitSeconds: 1, RequestTimeoutMs: 1000, ConsumerAckWaitSeconds: 2, ConsumerRetryMax: 2, ConsumerRetryDelayMs: 10, StreamMaxBytes: 1 << 20, StreamMaxAgeSeconds: 60, DeadLetterMaxBytes: 1 << 20, StreamReplicas: 1, StreamName: "SIM_TEST", StreamSubjects: []string{"sim.>"}, DeadLetterPrefix: "dead"}
	store := &simSignalStore{session: Session{ID: 91, TenantID: 7, OwnerAccountID: 11, SharedAccountIDs: []int64{12, 13}, Status: SessionAvailable}}
	var services []*Service
	for range 2 {
		bus, err := eventbus.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := bus.Close(); err != nil {
				t.Error(err)
			}
		})
		hub, err := ws.NewHub(ws.NewOriginPolicy(nil, nil), ws.HubOptions{ReadTimeout: 5 * time.Second, WriteTimeout: time.Second, PingInterval: time.Second, ReadLimit: 1024})
		if err != nil {
			t.Fatal(err)
		}
		svc := &Service{store: store, bus: bus.(eventbus.FanoutBus), wsHub: hub}
		if err := RegisterEventSubscriptions(svc); err != nil {
			t.Fatal(err)
		}
		services = append(services, svc)
	}
	open := func(hub *ws.Hub, tenantID, accountID int64, scope string) *websocket.Conn {
		t.Helper()
		httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := hub.ServeInteractive(w, r, func(conn *ws.Conn) error {
				if err := conn.BindSession(conn.Context(), ws.SessionKey{TenantID: tenantID, AccountID: accountID, Scope: scope}); err != nil {
					return err
				}
				if err := conn.SendJSON("ready"); err != nil {
					return err
				}
				for {
					var value string
					if err := conn.ReadJSONContext(conn.Context(), &value); err != nil {
						return err
					}
					if err := conn.SendJSON(value); err != nil {
						return err
					}
				}
			}); err != nil {
				t.Error(err)
			}
		}))
		t.Cleanup(httpServer.Close)
		client, response, err := websocket.DefaultDialer.Dial(strings.Replace(httpServer.URL, "http://", "ws://", 1), nil)
		if response != nil && response.Body != nil {
			if err := response.Body.Close(); err != nil {
				t.Error(err)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := client.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := client.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var ready string
		if err := client.ReadJSON(&ready); err != nil {
			t.Fatal(err)
		}
		return client
	}
	assertOpen := func(client *websocket.Conn) {
		t.Helper()
		if err := client.WriteJSON("ping"); err != nil {
			t.Fatal(err)
		}
		var pong string
		if err := client.ReadJSON(&pong); err != nil || pong != "ping" {
			t.Fatalf("authorized stream closed: %q %v", pong, err)
		}
	}
	assertClosed := func(client *websocket.Conn) {
		t.Helper()
		_, _, err := client.ReadMessage()
		if !websocket.IsCloseError(err, websocket.ClosePolicyViolation) {
			t.Fatalf("revoked stream remained active: %v", err)
		}
	}
	scope := simSessionSessionScope(91)
	owner := open(services[0].wsHub, 7, 11, scope)
	revokedA := open(services[0].wsHub, 7, 12, scope)
	revokedB := open(services[1].wsHub, 7, 12, scope)
	retained := open(services[1].wsHub, 7, 13, scope)
	otherTenant := open(services[0].wsHub, 8, 12, scope)
	otherScope := open(services[1].wsHub, 7, 12, simSessionSessionScope(92))
	store.set(Session{ID: 91, TenantID: 7, OwnerAccountID: 11, SharedAccountIDs: []int64{13}, Status: SessionAvailable})
	ctx := response.WithTrace(context.Background(), "sim-control-test")
	if err := services[0].publishSessionChanged(ctx, 7, 91); err != nil {
		t.Fatal(err)
	}
	assertClosed(revokedA)
	assertClosed(revokedB)
	assertOpen(owner)
	assertOpen(retained)
	assertOpen(otherTenant)
	assertOpen(otherScope)
	store.set(Session{ID: 91, TenantID: 7, OwnerAccountID: 11, SharedAccountIDs: []int64{12, 13}, Status: SessionAvailable})
	rejoined := open(services[1].wsHub, 7, 12, scope)
	if err := services[0].publishSessionChanged(ctx, 7, 91); err != nil {
		t.Fatal(err)
	}
	assertOpen(rejoined)
	store.set(Session{ID: 91, TenantID: 7, OwnerAccountID: 11, Status: SessionArchived})
	if err := services[0].publishSessionChanged(ctx, 7, 91); err != nil {
		t.Fatal(err)
	}
	assertClosed(owner)
	assertClosed(retained)
	assertClosed(rejoined)
	assertOpen(otherTenant)
	assertOpen(otherScope)
	reader := open(services[1].wsHub, 7, 11, simReplaySessionScope(91))
	formerMember := open(services[0].wsHub, 7, 12, simReplaySessionScope(91))
	if err := services[0].publishSessionChanged(ctx, 7, 91); err != nil {
		t.Fatal(err)
	}
	assertOpen(reader)
	assertClosed(formerMember)
}

type previewBatchStore struct {
	Store
	TxStore
	t                                       *testing.T
	packages                                []Package
	claimErr                                error
	onClaim                                 func()
	onMerge                                 func()
	renew                                   func(context.Context, int64) (bool, error)
	renewErrors, releaseErrors, mergeErrors map[int64]error
	renewed, released, merged               []int64
	reports                                 map[int64]ValidationReport
	claims                                  int
}

func (s *previewBatchStore) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s)
}
func (s *previewBatchStore) ExhaustPreviewAttempts(_ context.Context, attempts int32, _ time.Time, report ValidationReport) ([]Review, error) {
	if attempts != 2 || report.WorkerPreview.Status != validationFailed {
		s.t.Fatal("preview changed exhaustion policy")
	}
	return nil, nil
}
func (s *previewBatchStore) ClaimPackagesForPreview(_ context.Context, batch, maxAttempts int32, before, until time.Time, token string) ([]Package, error) {
	if batch != 6 || maxAttempts != 2 || until.Sub(before) > 3*time.Second || token == "" {
		s.t.Fatal("preview changed claim parameters")
	}
	s.claims++
	for i := range s.packages {
		s.packages[i].PreviewLeaseToken = token
	}
	if s.onClaim != nil {
		s.onClaim()
	}
	return s.packages, s.claimErr
}
func (s *previewBatchStore) checkLease(reviewID, id int64, token string) {
	if reviewID != id+100 || token == "" || s.packages[id-1].PreviewLeaseToken != token {
		s.t.Fatal("preview changed its review or lease identity")
	}
}
func (s *previewBatchStore) RenewPreviewLease(ctx context.Context, reviewID, id int64, token string, _ time.Time) (bool, error) {
	s.checkLease(reviewID, id, token)
	s.renewed = append(s.renewed, id)
	if s.renew != nil {
		return s.renew(ctx, id)
	}
	return true, s.renewErrors[id]
}
func (s *previewBatchStore) ReleasePreviewLease(_ context.Context, reviewID, id int64, token string) (int64, error) {
	s.checkLease(reviewID, id, token)
	s.released = append(s.released, id)
	return 1, s.releaseErrors[id]
}
func (s *previewBatchStore) MergeValidationReport(_ context.Context, reviewID, id int64, token string, report ValidationReport) (Review, error) {
	s.checkLease(reviewID, id, token)
	if err := s.mergeErrors[id]; err != nil {
		return Review{}, err
	}
	s.merged = append(s.merged, id)
	s.reports[id] = report
	if s.onMerge != nil {
		s.onMerge()
	}
	return Review{ID: reviewID, PackageID: id, PreviewReport: report}, nil
}

type previewBatchBackend struct {
	BackendAdapter
	t        *testing.T
	result   PreviewResult
	failures map[int64]error
	called   []int64
	preview  func(context.Context) (PreviewResult, error)
}

func (b *previewBatchBackend) Preview(ctx context.Context, pkg Package, runID int64, bundle *ExecutionBundle, frames int) (PreviewResult, error) {
	if runID <= 0 || bundle != nil || frames != 1 || response.TraceFromContext(ctx) != "preview-batch" {
		b.t.Fatal("preview changed native execution inputs or trace")
	}
	b.called = append(b.called, pkg.ID)
	if b.preview != nil {
		return b.preview(ctx)
	}
	result := b.result
	if pkg.ID == 6 {
		result.DeterminismPassed = false
		result.Detail = "algorithm differs"
	}
	return result, b.failures[pkg.ID]
}

func newPreviewFailureBatch(t *testing.T) (*Service, *previewBatchStore, *previewBatchBackend) {
	t.Helper()
	store := &previewBatchStore{t: t, reports: map[int64]ValidationReport{}}
	for id := int64(1); id <= 6; id++ {
		store.packages = append(store.packages, Package{ID: id, Code: fmt.Sprintf("preview-%d", id), Version: "1.0.0", BackendAdapter: "fixed-image", PreviewReviewID: id + 100, PreviewAttemptCount: 1, ScaleLimit: map[string]any{"nodes": 8, "max_tick": 8, "max_events": 8}})
	}
	raw, err := os.ReadFile("testdata/runtime_protocol.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Snapshot map[string]any }
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	frame := BackendSnapshot{Tick: 0, State: fixture.Snapshot["state"].(map[string]any), View: fixture.Snapshot["view"].(map[string]any), InteractionAvailability: fixture.Snapshot["interactionAvailability"].(map[string]any), CheckpointResults: fixture.Snapshot["checkpointResults"].(map[string]any)}
	backend := &previewBatchBackend{t: t, result: PreviewResult{DeterminismPassed: true, Frames: []BackendSnapshot{frame}}}
	ids, err := snowflake.NewNode(0)
	if err != nil {
		t.Fatal(err)
	}
	return &Service{store: store, ids: ids, backends: BackendRegistry{"fixed-image": backend}, packageRunnerCode: "package-runner", previewBatchSize: 6, previewLeaseDurationMs: 3000, previewMaxAttempts: 2, previewFrameCount: 1}, store, backend
}

func TestPreviewBatchRetainsFailuresAndCancellation(t *testing.T) {
	for _, mode := range []string{"success and rejected algorithm", "independent failures", "claim failure", "empty", "cancel before claim", "cancel claimed batch", "cancel empty claim", "claim failure and cancel", "cancel during report"} {
		t.Run(mode, func(t *testing.T) {
			service, store, backend := newPreviewFailureBatch(t)
			ctx, cancel := context.WithCancel(response.WithTrace(context.Background(), "preview-batch"))
			defer cancel()
			causes := []error{errors.New("initial lease failure"), errors.New("preview execution failure"), errors.New("lease release failure"), errors.New("report write failure")}
			wantRenewed := []int64{1, 2, 3, 4, 5, 6}
			wantCalled := []int64{1, 2, 3, 4, 5, 6}
			wantMerged := []int64{1, 2, 3, 4, 5, 6}
			var wantReleased []int64
			var wantErrors []error
			wantClaims := 1
			switch mode {
			case "independent failures":
				store.renewErrors = map[int64]error{1: causes[0]}
				backend.failures = map[int64]error{2: causes[1]}
				store.releaseErrors = map[int64]error{2: causes[2]}
				store.mergeErrors = map[int64]error{3: causes[3]}
				store.packages[3].BackendAdapter = "missing"
				store.packages[3].PreviewAttemptCount = 2
				wantErrors = append(causes, apperr.ErrSimBackendComputeUnavailable)
				wantCalled = []int64{2, 3, 5, 6}
				wantMerged = []int64{4, 5, 6}
				wantReleased = []int64{2}
			case "claim failure":
				store.claimErr = causes[0]
				wantErrors = causes[:1]
				wantRenewed = nil
				wantCalled = nil
				wantMerged = nil
			case "empty":
				store.packages = nil
				wantRenewed = nil
				wantCalled = nil
				wantMerged = nil
			case "cancel before claim":
				cancel()
				wantErrors = []error{context.Canceled}
				wantRenewed = nil
				wantCalled = nil
				wantMerged = nil
				wantClaims = 0
			case "cancel claimed batch", "cancel empty claim", "claim failure and cancel":
				store.onClaim = cancel
				wantErrors = []error{context.Canceled}
				wantRenewed = nil
				wantCalled = nil
				wantMerged = nil
				if mode == "cancel empty claim" {
					store.packages = nil
				}
				if mode == "claim failure and cancel" {
					store.claimErr = causes[0]
					wantErrors = append(wantErrors, causes[0])
				}
			case "cancel during report":
				store.onMerge = cancel
				wantErrors = []error{context.Canceled}
				wantRenewed = []int64{1}
				wantCalled = []int64{1}
				wantMerged = []int64{1}
			}
			err := service.RunPackagePreviewOnce(ctx)
			if len(wantErrors) == 0 && err != nil {
				t.Fatal(err)
			}
			for _, cause := range wantErrors {
				if !errors.Is(err, cause) {
					t.Errorf("error=%v missing %v", err, cause)
				}
			}
			if !reflect.DeepEqual(store.renewed, wantRenewed) || !reflect.DeepEqual(backend.called, wantCalled) || !reflect.DeepEqual(store.merged, wantMerged) || !reflect.DeepEqual(store.released, wantReleased) || store.claims != wantClaims {
				t.Fatalf("renewed=%v executed=%v reports=%v released=%v claims=%d", store.renewed, backend.called, store.merged, store.released, store.claims)
			}
			if report, ok := store.reports[6]; ok && (report.DeterminismCheck.Status != validationFailed || report.WorkerPreview.Status != validationPassed || len(report.PreviewFrames) == 0) {
				t.Fatal("rejected algorithm lost its successfully completed preview report")
			}
			if report, ok := store.reports[4]; ok && mode == "independent failures" && (report.DeterminismCheck.Status != validationFailed || report.WorkerPreview.Status != validationFailed) {
				t.Fatal("exhausted preview did not retain its terminal report")
			}
		})
	}
}

func TestPreviewLeaseLossRetainsExecutionFailure(t *testing.T) {
	service, store, backend := newPreviewFailureBatch(t)
	store.packages = store.packages[:1]
	store.packages[0].PreviewLeaseToken = "owned-lease"
	renewErr, executeErr, releaseErr := errors.New("active lease renewal failed"), errors.New("preview interrupted"), errors.New("failure recovery write failed")
	store.renew = func(context.Context, int64) (bool, error) {
		if len(store.renewed) == 1 {
			return true, nil
		}
		return false, renewErr
	}
	store.releaseErrors = map[int64]error{1: releaseErr}
	backend.preview = func(ctx context.Context) (PreviewResult, error) { <-ctx.Done(); return PreviewResult{}, executeErr }
	ctx := response.WithTrace(context.Background(), "preview-batch")
	err := service.previewPackageWithLease(ctx, store.packages[0], 3*time.Second)
	for _, cause := range []error{renewErr, executeErr, releaseErr} {
		if !errors.Is(err, cause) {
			t.Errorf("error=%v missing %v", err, cause)
		}
	}
	if len(store.renewed) != 2 || !reflect.DeepEqual(backend.called, []int64{1}) || !reflect.DeepEqual(store.released, []int64{1}) || len(store.merged) != 0 {
		t.Fatalf("lost lease changed retry or report boundaries: renewed=%v executed=%v released=%v merged=%v", store.renewed, backend.called, store.released, store.merged)
	}
}
