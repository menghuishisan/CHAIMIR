package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
)

type selftestPlanStore struct {
	Store
	TxStore
	runtime       Runtime
	image         RuntimeImage
	started       bool
	finishErr     error
	status        int16
	runtimeStatus int16
	detail        []byte
}

func (s *selftestPlanStore) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s)
}

func (s *selftestPlanStore) GetRuntimeByIDForUpdate(context.Context, int64) (Runtime, error) {
	return s.runtime, nil
}

func (s *selftestPlanStore) ListRuntimeImages(context.Context, int64) ([]RuntimeImage, error) {
	return []RuntimeImage{s.image}, nil
}

func (s *selftestPlanStore) GetRuntimeImageByIDForUpdate(context.Context, int64, int64) (RuntimeImage, error) {
	return s.image, nil
}

func (s *selftestPlanStore) StartRuntimeSelftest(context.Context, int64, []byte) (Runtime, error) {
	s.started = true
	return s.runtime, nil
}

func runtimeSelftestConfig(imageURL string) config.SandboxConfig {
	return config.SandboxConfig{
		ImageRegistry: "registry.example", SelftestTimeoutSeconds: 1, SelftestRecycleTimeoutSeconds: 1, ReadyTimeoutSeconds: 1,
		PlatformImageAttestations: []config.PlatformImageAttestation{{ImageURL: imageURL, Digest: digestFromImageURL(imageURL), CosignVerified: true, TrivyStatus: "passed"}},
	}
}

func TestRuntimeSelftestRechecksCurrentImageAdmission(t *testing.T) {
	for _, scenario := range []string{"missing-runtime-proof", "unsigned-runtime", "failed-runtime-scan", "revoked-infra", "revoked-init", "revoked-pod-container", "all-admitted"} {
		t.Run(scenario, func(t *testing.T) {
			svc, base, _ := terminalRecheckFixture(t)
			svc.cfg = runtimeSelftestConfig(base.image.ImageURL)
			other := "registry.example/infra@sha256:" + strings.Repeat("b", 64)
			component := workload.ComponentSpec{Name: "extra", ImageURL: other}
			wantErr := error(apperr.ErrSandboxImageAttestationInvalid)
			switch scenario {
			case "missing-runtime-proof":
				svc.cfg.PlatformImageAttestations = nil
			case "unsigned-runtime":
				svc.cfg.PlatformImageAttestations[0].CosignVerified = false
			case "failed-runtime-scan":
				svc.cfg.PlatformImageAttestations[0].TrivyStatus = "failed"
			case "revoked-infra":
				base.runtime.AdapterSpec.InfraSidecars = append(base.runtime.AdapterSpec.InfraSidecars, component)
				wantErr = apperr.ErrSandboxSidecarImageInvalid
			case "revoked-init":
				base.runtime.AdapterSpec.Pods[0].InitContainers = []workload.ComponentSpec{component}
				wantErr = apperr.ErrSandboxSidecarImageInvalid
			case "revoked-pod-container":
				base.runtime.AdapterSpec.Pods[0].Containers = append(base.runtime.AdapterSpec.Pods[0].Containers, component)
				wantErr = apperr.ErrSandboxSidecarImageInvalid
			}
			finishErr := errors.New("selftest persistence boundary")
			store := &selftestPlanStore{runtime: base.runtime, image: base.image, finishErr: finishErr}
			orchestrator := &selftestPlanOrchestrator{stop: errors.New("resource preparation boundary")}
			svc.store, svc.orchestrator = store, orchestrator
			_, err := svc.RunRuntimeSelftest(context.Background(), base.runtime.ID)
			if scenario == "all-admitted" {
				if !errors.Is(err, finishErr) || !store.started || orchestrator.plan.Sandbox.ID == 0 || !orchestrator.cleaned {
					t.Fatalf("admitted runtime did not reach resource preparation: err=%v started=%v plan=%+v cleaned=%v", err, store.started, orchestrator.plan, orchestrator.cleaned)
				}
				return
			}
			if !errors.Is(err, wantErr) || store.started || orchestrator.plan.Sandbox.ID != 0 || orchestrator.cleaned {
				t.Fatalf("revoked image reached selftest: err=%v want=%v started=%v sandbox=%d cleaned=%v", err, wantErr, store.started, orchestrator.plan.Sandbox.ID, orchestrator.cleaned)
			}
		})
	}
}

func (s *selftestPlanStore) FinishRuntimeSelftest(_ context.Context, _ int64, _ string, status, runtimeStatus int16, detail []byte) (Runtime, error) {
	s.status, s.runtimeStatus, s.detail = status, runtimeStatus, detail
	// 终点在条件写回；此夹具不代表审计、通知或数据库已验收。
	return Runtime{}, s.finishErr
}

type selftestPlanOrchestrator struct {
	Orchestrator
	plan                   CreateSandboxPlan
	archives               []RuntimeInitArchive
	preflightErr           error
	stop                   error
	cleaned                bool
	cleanupErr             error
	failCommand            string
	waitForCleanupDeadline bool
	afterCreate            func()
}

func (o *selftestPlanOrchestrator) CreateSandboxResources(_ context.Context, plan CreateSandboxPlan, archives []RuntimeInitArchive) error {
	o.plan, o.archives = plan, archives
	o.preflightErr = validateRuntimeIsolation(plan.Runtimes)
	if o.preflightErr != nil {
		return o.preflightErr
	}
	if o.afterCreate != nil {
		o.afterCreate()
	}
	return o.stop
}

func (o *selftestPlanOrchestrator) DestroySandboxResources(ctx context.Context, _ Sandbox) error {
	o.cleaned = true
	if o.waitForCleanupDeadline {
		<-ctx.Done()
		return nil
	}
	return errors.Join(o.cleanupErr, ctx.Err())
}

func (o *selftestPlanOrchestrator) Exec(_ context.Context, _, _ string, command []string, _ []byte, _ bool) ([]byte, []byte, error) {
	if command[0] == o.failCommand {
		return nil, nil, errors.New("injected execution failure")
	}
	return nil, nil, nil
}

func TestRuntimeSelftestUsesValidInstanceNames(t *testing.T) {
	instances := map[string]bool{}
	for _, code := range []string{"evm", strings.Repeat("a", 63), strings.Repeat("a", 63) + "b", strings.Repeat("a", 63) + "c"} {
		t.Run(code, func(t *testing.T) {
			if !codePattern.MatchString(code) {
				t.Fatal("fixture code is outside the registration contract")
			}
			svc, base, _ := terminalRecheckFixture(t)
			base.runtime.Code = code
			finishErr := errors.New("selftest persistence boundary")
			store := &selftestPlanStore{runtime: base.runtime, image: base.image, finishErr: finishErr}
			orchestrator := &selftestPlanOrchestrator{stop: errors.New("resource boundary")}
			svc.store, svc.orchestrator = store, orchestrator
			svc.cfg = runtimeSelftestConfig(base.image.ImageURL)
			if _, err := svc.RunRuntimeSelftest(context.Background(), base.runtime.ID); !errors.Is(err, finishErr) {
				t.Fatal(err)
			}
			if orchestrator.preflightErr != nil {
				t.Fatalf("registered runtime cannot pass selftest resource preflight: %v", orchestrator.preflightErr)
			}
			selected, ok := orchestrator.plan.RuntimeInstance(orchestrator.plan.WorkspaceRuntimeInstance)
			if !ok || selected.Runtime.Code != code {
				t.Error("selftest lost its runtime identity or workspace instance")
			}
			if instances[selected.InstanceCode] {
				t.Error("different runtime codes share a selftest instance name")
			}
			instances[selected.InstanceCode] = true
			if !orchestrator.cleaned || store.status != RuntimeSelftestFailed {
				t.Error("injected preparation failure did not clean up and report failure")
			}
			var detail runtimeSelftestStoredDetail
			if err := json.Unmarshal(store.detail, &detail); err != nil || detail.Stage != "prepare" {
				t.Fatalf("preparation failure detail=%s err=%v", store.detail, err)
			}
		})
	}
}

func TestRuntimeSelftestPreparesPlatformInitAssets(t *testing.T) {
	svc, base, _ := terminalRecheckFixture(t)
	plan, _ := runtimeAssetPlan()
	data := initArchiveTar(t)
	adapter := plan.Runtimes[0].Runtime.AdapterSpec
	for i := range adapter.InfraSidecars {
		adapter.InfraSidecars[i].ImageURL = base.image.ImageURL
	}
	for i := range adapter.Pods {
		for j := range adapter.Pods[i].InitContainers {
			adapter.Pods[i].InitContainers[j].ImageURL = base.image.ImageURL
		}
	}
	adapter.InitAssets[0].SourceRef = "s3://attach/0/sandbox/runtime-assets/genesis.tar"
	adapter.InitAssets[0].ExpectedSHA256 = pkgcrypto.SHA256Hex(data)
	base.runtime.AdapterSpec = adapter
	finishErr := errors.New("selftest persistence boundary")
	store := &selftestPlanStore{runtime: base.runtime, image: base.image, finishErr: finishErr}
	orchestrator := &selftestPlanOrchestrator{stop: errors.New("resource creation boundary")}
	objects := &initArchiveStorage{data: data}
	svc.store, svc.orchestrator, svc.objectStorage = store, orchestrator, objects
	svc.cfg = runtimeSelftestConfig(base.image.ImageURL)
	svc.cfg.InitArchiveMaxBytes, svc.cfg.InitArchiveMaxFiles, svc.cfg.InitArchiveMaxUnpackedBytes = 1<<20, 10, 1<<20
	if _, err := svc.RunRuntimeSelftest(context.Background(), base.runtime.ID); !errors.Is(err, finishErr) {
		t.Fatal(err)
	}
	if objects.reads != 1 || len(orchestrator.archives) != 1 || orchestrator.preflightErr != nil {
		t.Fatalf("platform runtime asset did not reach resource preparation: reads=%d archives=%v err=%v", objects.reads, orchestrator.archives, orchestrator.preflightErr)
	}
	archive := orchestrator.archives[0]
	if orchestrator.plan.Sandbox.TenantID != 0 || archive.RuntimeInstance != orchestrator.plan.WorkspaceRuntimeInstance || archive.TargetDomain != VolumeDomainRuntimeState || len(archive.Tarball) == 0 || !orchestrator.cleaned {
		t.Fatal("selftest lost asset ownership, destination or cleanup")
	}
}

func TestRuntimeSelftestWritesExecutionAndCleanupResult(t *testing.T) {
	for _, tt := range []struct {
		name        string
		failCommand string
		cleanupErr  error
		stage       string
	}{
		{name: "passed"},
		{name: "selected-image-disabled", stage: "prepare"},
		{name: "workspace-failure", failCommand: "clean", stage: "workspace"},
		{name: "toolchain-failure", failCommand: "toolchain", stage: "toolchain"},
		{name: "cleanup-failure", cleanupErr: errors.New("cleanup failed"), stage: "recycle"},
		{name: "execution-and-cleanup-failure", failCommand: "toolchain", cleanupErr: errors.New("cleanup failed"), stage: "toolchain"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, base, _ := terminalRecheckFixture(t)
			adapter := &base.runtime.AdapterSpec
			adapter.RuntimeContainer.PrepullCommand = []string{"toolchain", "selftest"}
			mount := true
			adapter.Pods[0].Containers = append(adapter.Pods[0].Containers, workload.ComponentSpec{
				Name: "worker", MountWorkspace: &mount, EphemeralMounts: []workload.EphemeralMountSpec{{Name: "tmp", MountPath: "/tmp"}},
			})
			pod := adapter.Pods[0].Name
			adapter.WorkspaceOps = WorkspaceOps{ExecTarget: pod + "/student", ScriptExecTarget: pod + "/worker",
				RunScript: []string{"run"}, CleanWorkspace: []string{"clean"}, Selftest: []string{"workspace", "selftest"}}
			finishErr := errors.New("selftest persistence boundary")
			store := &selftestPlanStore{runtime: base.runtime, image: base.image, finishErr: finishErr}
			orchestrator := &selftestPlanOrchestrator{cleanupErr: tt.cleanupErr, failCommand: tt.failCommand}
			if tt.name == "selected-image-disabled" {
				orchestrator.afterCreate = func() { store.image.Status = RuntimeImageStatusDisabled }
			}
			svc.store, svc.orchestrator = store, orchestrator
			svc.cfg = runtimeSelftestConfig(base.image.ImageURL)
			ctx := response.WithTrace(context.Background(), "selftest-trace")
			_, err := svc.RunRuntimeSelftest(ctx, base.runtime.ID)
			if !errors.Is(err, finishErr) {
				t.Fatal(err)
			}
			if tt.cleanupErr != nil && !errors.Is(err, tt.cleanupErr) {
				t.Fatalf("persistence failure hid the cleanup failure: %v", err)
			}
			status, runtimeStatus, result := RuntimeSelftestPassed, RuntimeStatusAvailable, "passed"
			if tt.stage != "" {
				status, runtimeStatus, result = RuntimeSelftestFailed, RuntimeStatusOnboarding, "failed"
			}
			var detail runtimeSelftestStoredDetail
			if err := json.Unmarshal(store.detail, &detail); err != nil {
				t.Fatal(err)
			}
			if !orchestrator.cleaned || store.status != status || store.runtimeStatus != runtimeStatus || detail.Result != result || detail.Stage != tt.stage {
				t.Fatalf("unexpected selftest outcome: status=%d runtime=%d cleaned=%v detail=%s", store.status, store.runtimeStatus, orchestrator.cleaned, store.detail)
			}
			if tt.stage != "" && detail.TraceID != "selftest-trace" {
				t.Fatalf("selftest lost the request trace: %s", store.detail)
			}
		})
	}
}

type platformSelftestStore struct {
	Store
	*terminalRecheckTx
	components    map[string]Tool
	reads         int
	lookupErr     error
	createErr     error
	createInput   CreateSandboxInput
	createCalls   int
	prepullStatus int16
}

func (s *platformSelftestStore) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s)
}

func (s *platformSelftestStore) TenantTx(ctx context.Context, _ int64, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s)
}

func (s *platformSelftestStore) EnsureTenantQuota(_ context.Context, quota TenantQuota) (TenantQuota, error) {
	return quota, nil
}

func (s *platformSelftestStore) GetTenantQuotaForUpdate(context.Context, int64) (TenantQuota, error) {
	return TenantQuota{MaxConcurrentSandbox: 10, MaxCPU: 100, MaxMemoryMB: 1 << 20}, nil
}

func (s *platformSelftestStore) CountActiveSandboxes(context.Context, int64) (int64, error) {
	return 0, nil
}

func (s *platformSelftestStore) GetCompositionPrepullForUpdate(context.Context, int64, string) (CompositionPrepull, error) {
	if s.prepullStatus != 0 {
		return CompositionPrepull{Status: s.prepullStatus}, nil
	}
	return CompositionPrepull{Status: ImagePrepullSucceeded}, nil
}

func (s *platformSelftestStore) CreateSandbox(_ context.Context, input CreateSandboxInput) (Sandbox, error) {
	s.createInput = input
	s.createCalls++
	return Sandbox{}, s.createErr
}

func (s *platformSelftestStore) GetToolByCode(_ context.Context, code string) (Tool, error) {
	s.reads++
	return s.components[code], s.lookupErr
}

func platformSelftestFixture(t *testing.T) (*Service, *platformSelftestStore, *selftestPlanOrchestrator, contracts.SandboxPlatformSelftestRequest) {
	t.Helper()
	svc, base, _ := terminalRecheckFixture(t)
	var snapshot contracts.SandboxCompositionSnapshot
	if err := json.Unmarshal(base.sandbox.CompositionSnapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	image := "registry.example/runtime@" + digest
	snapshot.Spec.AccessProfile = contracts.SandboxAccessJudgePrivate
	snapshot.Components = nil
	snapshot.Runtimes[0].ImageURL = image
	base.image.ImageURL = image
	store := &platformSelftestStore{terminalRecheckTx: base, components: map[string]Tool{}}
	orchestrator := &selftestPlanOrchestrator{}
	svc.store, svc.orchestrator = store, orchestrator
	svc.cfg = config.SandboxConfig{ImageRegistry: "registry.example", ReadyTimeoutSeconds: 1, SelftestRecycleTimeoutSeconds: 1,
		PlatformImageAttestations: []config.PlatformImageAttestation{{ImageURL: image, Digest: digest, CosignVerified: true, TrivyStatus: "passed"}}}
	snapshot.Digest = platformSnapshotDigest(t, snapshot)
	return svc, store, orchestrator, contracts.SandboxPlatformSelftestRequest{CompositionSnapshot: snapshot, ExecTarget: "sandbox/runtime", Commands: [][]string{{"selftest"}}, TimeoutSec: 1}
}

func platformSnapshotDigest(t *testing.T, snapshot contracts.SandboxCompositionSnapshot) string {
	t.Helper()
	digest, err := contracts.CanonicalSnapshotDigest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

type sandboxAcceptedStore struct {
	*platformSelftestStore
	created   Sandbox
	existing  []Sandbox
	committed bool
	commitErr error
	readErr   error
	reads     int
	claims    chan bool
}

func (s *sandboxAcceptedStore) TenantTx(ctx context.Context, _ int64, fn func(context.Context, TxStore) error) error {
	if err := fn(ctx, s); err != nil {
		return err
	}
	if s.created.ID > 0 && !s.committed {
		if s.commitErr != nil {
			return s.commitErr
		}
		s.committed = true
	}
	return nil
}

func (s *sandboxAcceptedStore) CreateSandbox(ctx context.Context, input CreateSandboxInput) (Sandbox, error) {
	if _, err := s.platformSelftestStore.CreateSandbox(ctx, input); err != nil {
		return Sandbox{}, err
	}
	s.created = Sandbox{
		ID: input.ID, TenantID: input.TenantID, Namespace: input.Namespace,
		SourceRef: input.SourceRef, ScopeRef: input.ScopeRef, OwnerAccountID: input.OwnerAccountID,
		CompositionDigest: input.CompositionDigest, CompositionSnapshot: input.CompositionSnapshot,
		Phase: input.Phase, Status: input.Status, WorkspaceRevision: 1,
	}
	return s.created, nil
}

func (s *sandboxAcceptedStore) GetSandbox(context.Context, int64, int64) (Sandbox, error) {
	s.reads++
	return s.sandbox, s.readErr
}

func (s *sandboxAcceptedStore) ListSandboxesByScopeRef(context.Context, int64, string) ([]Sandbox, error) {
	return s.existing, nil
}

func (s *sandboxAcceptedStore) ClaimSandboxResources(context.Context, int64, int64, int16, time.Duration) (Sandbox, bool, error) {
	s.claims <- s.committed
	return Sandbox{}, false, nil
}

func sandboxAcceptedFixture(t *testing.T) (*Service, *sandboxAcceptedStore, *startupEventBus, contracts.SandboxCreateRequest) {
	t.Helper()
	svc, base, _, fixture := platformSelftestFixture(t)
	svc.cfg.MaxCPU, svc.cfg.MaxMemory, svc.cfg.MaxPods = "100", "100Gi", "100"
	svc.cfg.DefaultReqCPU, svc.cfg.DefaultReqMemory, svc.cfg.DefaultCPU, svc.cfg.DefaultMemory = "100m", "64Mi", "100m", "64Mi"
	svc.cfg.ExecTimeoutSeconds = 1
	store := &sandboxAcceptedStore{platformSelftestStore: base, claims: make(chan bool, 1)}
	bus := &startupEventBus{}
	svc.store, svc.audit, svc.bus = store, recycleAuditWriter{}, bus
	req := contracts.SandboxCreateRequest{TenantID: 5, OwnerAccountID: 6, CompositionSnapshot: fixture.CompositionSnapshot,
		SourceRef: "experiment:2026:instance:4", ScopeRef: "experiment:2026:instance:4:run:1"}
	return svc, store, bus, req
}

func TestSandboxCreationReturnsAcceptedSnapshot(t *testing.T) {
	for _, withComponents := range []bool{false, true} {
		t.Run(fmt.Sprintf("components=%v", withComponents), func(t *testing.T) {
			svc, store, _, req := sandboxAcceptedFixture(t)
			if withComponents {
				for index, category := range []string{"infra", "tool"} {
					code := category + "-component"
					raw := json.RawMessage(`{"components":[]}`)
					req.CompositionSnapshot.Components = append(req.CompositionSnapshot.Components, contracts.CompiledComponentSnapshot{
						ComponentID: int64(10 + index), Code: code, Category: category, Kind: SandboxToolKindCommand, ResourceSpec: raw,
					})
					store.components[code] = Tool{ID: int64(10 + index), Code: code, Status: ToolStatusAvailable}
				}
				extra := req.CompositionSnapshot.Runtimes[0]
				extra.InstanceCode = "second-chain"
				req.CompositionSnapshot.Runtimes = append(req.CompositionSnapshot.Runtimes, extra)
				req.CompositionSnapshot.Spec.Runtimes = append(req.CompositionSnapshot.Spec.Runtimes,
					contracts.CompositionRuntimeRef{InstanceCode: extra.InstanceCode, Code: extra.Code, ImageVersion: extra.ImageVersion})
				req.CompositionSnapshot.Digest = platformSnapshotDigest(t, req.CompositionSnapshot)
			}
			store.readErr = errors.New("detail unavailable after commit")
			info, err := svc.CreateSandbox(response.WithTrace(context.Background(), "create-trace"), req)
			if err != nil {
				t.Fatalf("accepted creation reported failure: %v", err)
			}
			if !store.committed || store.createCalls != 1 || store.reads != 0 || info.SandboxID == 0 || info.SandboxID != store.created.ID ||
				info.TenantID != req.TenantID || info.OwnerAccountID != req.OwnerAccountID || info.Namespace != store.created.Namespace ||
				info.SourceRef != req.SourceRef || info.ScopeRef != req.ScopeRef || info.CompositionDigest != req.CompositionSnapshot.Digest ||
				info.RuntimeCode != "evm" || info.RuntimeImageVersion != "1" || info.WorkspaceRevision != 1 ||
				info.Phase != SandboxPhaseAllocating || info.Status != SandboxStatusCreating || info.ResourceUsage != nil || !info.Capabilities.Terminal {
				t.Fatalf("incomplete accepted snapshot: %+v, reads=%d committed=%v", info, store.reads, store.committed)
			}
			wantInstances := []string{"chain"}
			if withComponents {
				wantInstances = append(wantInstances, "second-chain")
				if len(info.ToolAccess) != 1 || info.ToolAccess[0].ToolCode != "tool-component" ||
					info.ToolAccess[0].Status != SandboxToolStatusStarting || !info.Capabilities.CommandTools ||
					info.ToolAccess[0].Endpoint != toolEndpoint(info.SandboxID, Tool{Code: "tool-component", Kind: SandboxToolKindCommand}) {
					t.Fatalf("tool capability or initial readiness lost: %+v", info)
				}
			} else if len(info.ToolAccess) != 0 || info.Capabilities.CommandTools {
				t.Fatalf("creation fabricated tool capabilities: %+v", info)
			}
			if !slices.Equal(info.RuntimeInstances, wantInstances) {
				t.Fatalf("runtime instances=%v, want %v", info.RuntimeInstances, wantInstances)
			}
			select {
			case committed := <-store.claims:
				if !committed {
					t.Fatal("startup scheduled before commit")
				}
			case <-time.After(time.Second):
				t.Fatal("accepted creation did not schedule startup")
			}
		})
	}
}

func TestSandboxCreationDoesNotAcceptFailedTransaction(t *testing.T) {
	for _, stage := range []string{"record", "event", "audit", "commit"} {
		t.Run(stage, func(t *testing.T) {
			svc, store, bus, req := sandboxAcceptedFixture(t)
			failure := errors.New(stage + " failed")
			switch stage {
			case "record":
				store.createErr = failure
			case "event":
				store.eventErr = failure
			case "audit":
				svc.audit = recycleAuditWriter{err: failure}
			case "commit":
				store.commitErr = failure
			}
			info, err := svc.CreateSandbox(context.Background(), req)
			if !errors.Is(err, failure) || info.SandboxID != 0 || store.committed || store.reads != 0 || len(bus.subjects) != 0 {
				t.Fatalf("failed transaction was accepted: info=%+v err=%v committed=%v", info, err, store.committed)
			}
		})
	}
}

func TestSandboxCreationKeepsExistingState(t *testing.T) {
	for _, readFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("read-fails=%v", readFails), func(t *testing.T) {
			svc, store, bus, req := sandboxAcceptedFixture(t)
			req.ScopeRef = req.SourceRef
			store.sandbox.Status = SandboxStatusFailed
			store.existing = []Sandbox{store.sandbox}
			if readFails {
				store.readErr = errors.New("existing sandbox unreadable")
			}
			info, err := svc.CreateSandbox(context.Background(), req)
			if readFails {
				if !errors.Is(err, store.readErr) || info.SandboxID != 0 {
					t.Fatalf("existing read failure hidden: info=%+v err=%v", info, err)
				}
			} else if err != nil || info.SandboxID != store.sandbox.ID || info.Status != SandboxStatusFailed || info.Phase != store.sandbox.Phase {
				t.Fatalf("existing state replaced with accepted state: info=%+v err=%v", info, err)
			}
			if store.createCalls != 0 || store.reads != 1 || len(bus.subjects) != 0 {
				t.Fatal("idempotent request created or scheduled another sandbox")
			}
		})
	}
}

func TestPlatformSelftestUsesEveryFrozenComponent(t *testing.T) {
	for _, mode := range []string{"complete", "disabled", "identity-changed", "image-revoked", "invalid-frozen-spec", "lookup-failure"} {
		t.Run(mode, func(t *testing.T) {
			svc, store, orchestrator, req := platformSelftestFixture(t)
			for index, category := range []string{"infra", "tool"} {
				code := category + "-component"
				digest := "sha256:" + strings.Repeat(string(rune('b'+index)), 64)
				image := "registry.example/" + code + "@" + digest
				raw, err := json.Marshal(ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "worker", ImageURL: image}}})
				if err != nil {
					t.Fatal(err)
				}
				req.CompositionSnapshot.Components = append(req.CompositionSnapshot.Components, contracts.CompiledComponentSnapshot{
					ComponentID: int64(10 + index), Code: code, Category: category, Kind: SandboxToolKindCommand, ResourceSpec: raw,
				})
				// 目录执行规格故意不同；它只能撤销准入，不能替换已冻结镜像与组件类型。
				store.components[code] = Tool{ID: int64(10 + index), Code: code, Status: ToolStatusAvailable, Category: "changed", Kind: SandboxToolKindBuiltin}
				svc.cfg.PlatformImageAttestations = append(svc.cfg.PlatformImageAttestations, config.PlatformImageAttestation{ImageURL: image, Digest: digest, CosignVerified: true, TrivyStatus: "passed"})
			}
			tool := store.components["tool-component"]
			switch mode {
			case "disabled":
				tool.Status = ToolStatusDisabled
			case "identity-changed":
				tool.ID++
			case "image-revoked":
				svc.cfg.PlatformImageAttestations = svc.cfg.PlatformImageAttestations[:1]
			case "invalid-frozen-spec":
				req.CompositionSnapshot.Components[1].ResourceSpec = json.RawMessage(`{"unknown":true}`)
			case "lookup-failure":
				store.lookupErr = errors.New("component lookup unavailable")
			}
			if mode != "lookup-failure" {
				store.components["tool-component"] = tool
			}
			req.CompositionSnapshot.Digest = platformSnapshotDigest(t, req.CompositionSnapshot)
			err := svc.RunPlatformSelftest(context.Background(), req)
			if mode != "complete" {
				if err == nil || orchestrator.plan.Sandbox.ID != 0 || orchestrator.cleaned {
					t.Fatalf("invalid component reached resource execution: err=%v created=%v cleaned=%v", err, orchestrator.plan.Sandbox.ID != 0, orchestrator.cleaned)
				}
				return
			}
			if err != nil || store.reads != 2 || len(orchestrator.plan.Tools) != 2 || !orchestrator.cleaned {
				t.Fatalf("selftest skipped frozen components: err=%v reads=%d tools=%v", err, store.reads, orchestrator.plan.Tools)
			}
			for index, item := range orchestrator.plan.Tools {
				frozen := req.CompositionSnapshot.Components[index]
				if item.Code != frozen.Code || item.Category != frozen.Category || item.Kind != frozen.Kind || len(item.ResourceSpec.Components) != 1 {
					t.Fatalf("selftest replaced frozen component with current catalog values: %+v", item)
				}
			}
		})
	}
}

func TestPlatformSelftestRejectsMissingCleanupBudget(t *testing.T) {
	svc, _, orchestrator, req := platformSelftestFixture(t)
	svc.cfg.SelftestRecycleTimeoutSeconds = 0
	if err := svc.RunPlatformSelftest(context.Background(), req); !errors.Is(err, apperr.ErrSandboxSelftestRecycleConfigInvalid) || orchestrator.plan.Sandbox.ID != 0 || orchestrator.cleaned {
		t.Fatalf("invalid cleanup budget reached execution: err=%v cleaned=%v", err, orchestrator.cleaned)
	}
}

func TestPlatformSelftestCannotPassAfterCleanupDeadline(t *testing.T) {
	svc, _, orchestrator, req := platformSelftestFixture(t)
	orchestrator.waitForCleanupDeadline = true
	if err := svc.RunPlatformSelftest(context.Background(), req); !errors.Is(err, context.DeadlineExceeded) || !orchestrator.cleaned {
		t.Fatalf("expired cleanup reported success: err=%v cleaned=%v", err, orchestrator.cleaned)
	}
}

func TestSandboxCreationPreservesInitializationAndCallerInput(t *testing.T) {
	for _, scenario := range []string{"accepted", "tampered-digest", "invalid-lifetime"} {
		t.Run(scenario, func(t *testing.T) {
			svc, store, _, fixture := platformSelftestFixture(t)
			store.createErr = errors.New("sandbox persistence boundary")
			svc.cfg.MaxCPU, svc.cfg.MaxMemory, svc.cfg.MaxPods = "100", "100Gi", "100"
			svc.cfg.DefaultReqCPU, svc.cfg.DefaultReqMemory, svc.cfg.DefaultCPU, svc.cfg.DefaultMemory = "100m", "64Mi", "100m", "64Mi"
			svc.identity = sandboxIdentityStub{accounts: []contracts.AccountInfo{{AccountID: 7, TenantID: 5}}}
			req := contracts.SandboxCreateRequest{TenantID: 5, OwnerAccountID: 6, AuthorizedAccountIDs: []int64{6, 7, 7},
				CompositionSnapshot: fixture.CompositionSnapshot, SourceRef: " experiment:2026:instance:4 ",
				ScopeRef: " experiment:2026:instance:4:run:1 ", InitCodeRef: " s3://code/5/saved.tar ", InitScriptRef: " s3://attachments/5/init.sh "}
			want := store.createErr
			if scenario == "tampered-digest" {
				req.CompositionSnapshot.Digest = "sha256:" + strings.Repeat("f", 64)
				want = apperr.ErrSandboxCreateRequestInvalid
			} else if scenario == "invalid-lifetime" {
				req.KeepAliveMinutes = 1
				want = apperr.ErrSandboxCreateRequestInvalid
			}
			before, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.CreateSandbox(context.Background(), req)
			if !errors.Is(err, want) {
				t.Fatalf("creation returned %v, want %v", err, want)
			}
			after, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("creation mutated caller input")
			}
			if scenario != "accepted" {
				if store.createCalls != 0 {
					t.Fatal("invalid request reached persistence")
				}
				return
			}
			input := store.createInput
			if store.createCalls != 1 || input.SourceRef != strings.TrimSpace(req.SourceRef) || input.ScopeRef != strings.TrimSpace(req.ScopeRef) ||
				input.InitCodeRef != strings.TrimSpace(req.InitCodeRef) || input.InitScriptRef != strings.TrimSpace(req.InitScriptRef) ||
				input.OwnerAccountID != 6 || !slices.Equal(input.SharedAccountIDs, []int64{7}) {
				t.Fatalf("creation lost normalized input or restored initialization: %+v", input)
			}
			var stored contracts.SandboxCompositionSnapshot
			if err := json.Unmarshal(input.CompositionSnapshot, &stored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(stored, req.CompositionSnapshot) {
				t.Fatal("creation changed the frozen composition")
			}
		})
	}
}

func TestFrozenPlanAdmissionMatchesAcrossEntrypoints(t *testing.T) {
	for _, mode := range []string{"runtime-target", "tool-target", "unknown-target", "image-disabled", "tool-disabled", "missing-private-script", "prepull-pending", "private-sidecar", "private-image-revoked"} {
		for _, entry := range []string{"template", "create", "restore", "platform-selftest"} {
			t.Run(mode+"/"+entry, func(t *testing.T) {
				svc, store, orchestrator, req := platformSelftestFixture(t)
				store.createErr = errors.New("sandbox record boundary")
				snapshot := &req.CompositionSnapshot
				var adapter AdapterSpec
				if err := json.Unmarshal(snapshot.Runtimes[0].AdapterSpec, &adapter); err != nil {
					t.Fatal(err)
				}
				adapter.RuntimeContainer.Ports = []workload.PortSpec{{Name: "rpc", ContainerPort: 8545}}
				if mode == "missing-private-script" {
					adapter.WorkspaceOps.ScriptExecTarget = ""
				}
				raw, err := json.Marshal(adapter)
				if err != nil {
					t.Fatal(err)
				}
				snapshot.Runtimes[0].AdapterSpec = raw
				peer := snapshot.Runtimes[0]
				peer.InstanceCode = "peer"
				snapshot.Runtimes = append(snapshot.Runtimes, peer)
				snapshot.Spec.Runtimes = append(snapshot.Spec.Runtimes, contracts.CompositionRuntimeRef{InstanceCode: "peer", Code: peer.Code, ImageVersion: peer.ImageVersion})
				for index, code := range []string{"relay", "indexer"} {
					spec := ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "worker", ImageURL: store.image.ImageURL,
						Ports: []workload.PortSpec{{Name: "api", ContainerPort: 9000}}}}}
					if code == "relay" && (mode == "runtime-target" || mode == "tool-target" || mode == "unknown-target") {
						target, port := "peer-sandbox", workload.NetworkPortRef{Name: "rpc", Port: 8545}
						if mode == "tool-target" {
							target, port = toolComponentPodName("indexer", "worker"), workload.NetworkPortRef{Name: "api", Port: 9000}
						} else if mode == "unknown-target" {
							target = "outside-composition"
						}
						spec.NetworkRules = []workload.NetworkRuleSpec{{Name: "relay-provider", From: "worker", To: target, Ports: []workload.NetworkPortRef{port}}}
					}
					raw, err := json.Marshal(spec)
					if err != nil {
						t.Fatal(err)
					}
					store.components[code] = Tool{ID: int64(10 + index), Code: code, Status: ToolStatusAvailable}
					snapshot.Components = append(snapshot.Components, contracts.CompiledComponentSnapshot{ComponentID: int64(10 + index), Code: code,
						Category: "tool", Kind: SandboxToolKindCommand, ResourceSpec: raw})
				}
				if mode == "image-disabled" {
					store.image.Status = RuntimeImageStatusDisabled
				} else if mode == "tool-disabled" {
					tool := store.components["relay"]
					tool.Status = ToolStatusDisabled
					store.components["relay"] = tool
				} else if mode == "prepull-pending" {
					store.prepullStatus = ImagePrepullRunning
				}
				if strings.HasPrefix(mode, "private-") {
					image := "registry.example/judge@sha256:" + strings.Repeat("b", 64)
					snapshot.PrivateSidecars = []contracts.SandboxPrivateSidecarSpec{{Name: "judge", ImageURL: image}}
					if mode == "private-sidecar" {
						svc.cfg.PlatformImageAttestations = append(svc.cfg.PlatformImageAttestations, config.PlatformImageAttestation{
							ImageURL: image, Digest: "sha256:" + strings.Repeat("b", 64), CosignVerified: true, TrivyStatus: "passed",
						})
					}
				}
				snapshot.Digest = platformSnapshotDigest(t, *snapshot)
				svc.cfg.MaxCPU, svc.cfg.MaxMemory, svc.cfg.MaxPods = "100", "100Gi", "100"
				svc.cfg.DefaultReqCPU, svc.cfg.DefaultReqMemory, svc.cfg.DefaultCPU, svc.cfg.DefaultMemory = "100m", "64Mi", "100m", "64Mi"
				source := "experiment:2026:instance:4"
				switch entry {
				case "template":
					err = svc.ValidateSandboxTemplate(context.Background(), contracts.SandboxCreateRequest{TenantID: 5, OwnerAccountID: 6,
						SourceRef: source, ScopeRef: source + ":run:1", CompositionSnapshot: *snapshot})
				case "create":
					_, err = svc.CreateSandbox(context.Background(), contracts.SandboxCreateRequest{TenantID: 5, OwnerAccountID: 6,
						SourceRef: source, ScopeRef: source + ":run:1", CompositionSnapshot: *snapshot})
				case "restore":
					raw, marshalErr := json.Marshal(snapshot)
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
					_, err = svc.planForExistingSandbox(context.Background(), Sandbox{CompositionDigest: snapshot.Digest, CompositionSnapshot: raw})
				case "platform-selftest":
					err = svc.RunPlatformSelftest(context.Background(), req)
				}
				wantOK := mode == "runtime-target" || mode == "tool-target" || mode == "private-sidecar" || mode == "prepull-pending" && entry == "template"
				accepted := err == nil || entry == "create" && errors.Is(err, store.createErr)
				if accepted != wantOK {
					t.Fatalf("admission disagrees with frozen composition: err=%v wantOK=%v", err, wantOK)
				}
				if !wantOK && orchestrator.plan.Sandbox.ID != 0 {
					t.Fatal("invalid frozen plan reached resource creation")
				}
			})
		}
	}
}

type networkCompositionStore struct{ *platformSelftestStore }

func (s networkCompositionStore) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s)
}

func (s networkCompositionStore) GetRuntimeByCode(context.Context, string) (Runtime, error) {
	return s.runtime, nil
}

func (s networkCompositionStore) GetRuntimeImageByVersionForShare(context.Context, int64, string) (RuntimeImage, error) {
	return s.image, nil
}

func (s networkCompositionStore) ListTools(context.Context) ([]Tool, error) {
	tools := make([]Tool, 0, len(s.components))
	for _, tool := range s.components {
		tools = append(tools, tool)
	}
	return tools, nil
}

func (s networkCompositionStore) UpsertCompositionSnapshot(context.Context, string, []byte) error {
	return nil
}

func networkCompositionFixture(t *testing.T) (*Service, *platformSelftestStore) {
	t.Helper()
	svc, store, _, request := platformSelftestFixture(t)
	if err := json.Unmarshal(request.CompositionSnapshot.Runtimes[0].AdapterSpec, &store.runtime.AdapterSpec); err != nil {
		t.Fatal(err)
	}
	return svc, store
}

func TestRuntimeCanCompileAndLoadInfraDependency(t *testing.T) {
	svc, store := networkCompositionFixture(t)
	store.runtime.AdapterSpec.RuntimeContainer.Ports = []workload.PortSpec{{Name: "rpc", ContainerPort: 8545, ServicePort: 8545, Protocol: "TCP"}}
	noWorkspace := false
	store.components["database"] = Tool{ID: 10, Code: "database", Category: "infra", Kind: SandboxToolKindInfra, Status: ToolStatusAvailable,
		ResourceSpec: ToolResourceSpec{
			Components: []workload.ComponentSpec{{Name: "worker", ImageURL: store.image.ImageURL, MountWorkspace: &noWorkspace,
				Ports: []workload.PortSpec{{Name: "api", ContainerPort: 9000, ServicePort: 9000, Protocol: "TCP"}}}},
			Services: []workload.ServiceSpec{{Name: "infra-database", Component: "worker",
				Ports: []workload.ServicePortSpec{{Name: "api", Port: 9000, TargetPort: "api", Protocol: "TCP"}}}},
			Capabilities: CapabilityConstraints{Placement: "sandbox"},
		}}
	spec := contracts.SandboxCompositionSpec{ID: "runtime-dependency", AccessProfile: contracts.SandboxAccessJudgePrivate, WorkspaceRuntimeInstance: "chain",
		Runtimes: []contracts.CompositionRuntimeRef{{InstanceCode: "chain", Code: store.runtime.Code, ImageVersion: store.image.Version}},
		Infra:    []contracts.CompositionComponentRef{{Code: "database", Selection: "explicit"}},
		Links: []contracts.CompositionLink{{SourceComponent: "chain", SourceEndpoint: "rpc", TargetComponent: "database", TargetEndpoint: "api",
			Protocol: "TCP", AccessScope: "sandbox", ConfigBinding: "env:DATABASE_URL", RequiredAtStart: true}},
	}
	snapshot, err := CompilePlatformSandboxComposition(context.Background(), networkCompositionStore{store}, spec)
	if err != nil {
		t.Fatalf("runtime cannot compile a declared infra dependency: %v", err)
	}
	plan, err := svc.planForSnapshot(context.Background(), snapshot)
	if err != nil || len(plan.Runtimes) != 1 || len(plan.Tools) != 1 {
		t.Fatalf("compiled dependency cannot load: plan=%+v err=%v", plan, err)
	}
	rules := plan.Runtimes[0].Runtime.AdapterSpec.NetworkRules
	if len(rules) != 1 || rules[0].From != "chain-sandbox" || rules[0].To != toolComponentPodName("database", "worker") {
		t.Fatalf("runtime dependency did not retain real Pod targets: %+v", rules)
	}
}

func TestCompositionLinksUseServiceAddressAndPodPort(t *testing.T) {
	for _, sourceRuntime := range []bool{true, false} {
		for _, targetRuntime := range []bool{true, false} {
			for _, mapping := range []string{"same", "remapped", "renamed", "collision"} {
				if targetRuntime && (mapping == "renamed" || mapping == "collision") {
					continue
				}
				for _, protocol := range []string{"TCP", "HTTP", "HTTPS", "WS", "WSS", "GRPC"} {
					t.Run(fmt.Sprintf("source-runtime=%t/target-runtime=%t/%s/%s", sourceRuntime, targetRuntime, mapping, protocol), func(t *testing.T) {
						svc, store := networkCompositionFixture(t)
						const servicePort int32 = 8000
						containerPort, portName := servicePort, "api"
						if mapping != "same" {
							containerPort = 9000
							if mapping != "remapped" {
								portName = "backend"
							}
						}
						noWorkspace := false
						store.runtime.AdapterSpec.RuntimeContainer.Ports = []workload.PortSpec{{Name: "rpc", ContainerPort: containerPort, ServicePort: servicePort, Protocol: "TCP"}}
						store.runtime.AdapterSpec.Capabilities.Provides = []string{"test-api"}
						spec := contracts.SandboxCompositionSpec{ID: "port-mapping", AccessProfile: contracts.SandboxAccessJudgePrivate, WorkspaceRuntimeInstance: "chain",
							Runtimes: []contracts.CompositionRuntimeRef{{InstanceCode: "chain", Code: store.runtime.Code, ImageVersion: store.image.Version}}}
						source, sourceEndpoint, sourceRole := "chain", "rpc", "chain-sandbox"
						if !sourceRuntime {
							source, sourceEndpoint, sourceRole = "client", "component", toolComponentPodName("client", "worker")
							store.components["client"] = Tool{ID: 11, Code: "client", Category: "tool", Kind: SandboxToolKindCommand, Status: ToolStatusAvailable,
								ResourceSpec: ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "worker", ImageURL: store.image.ImageURL, MountWorkspace: &noWorkspace}},
									Capabilities: CapabilityConstraints{Placement: "sandbox"},
									Bindings:     []ComponentBinding{{Name: "target", SourceComponent: "worker", SourceEndpoint: "component", Capability: "test-api", Endpoint: "api", Protocol: protocol, ConfigBinding: "env:TARGET"}}}}
							spec.Tools = []contracts.CompositionComponentRef{{Code: "client", Selection: "explicit"}}
						}
						target, targetEndpoint, targetRole, targetService := "database", "api", toolComponentPodName("database", "worker"), "infra-database"
						if targetRuntime {
							target, targetEndpoint, targetRole, targetService = "peer", "rpc", "peer-sandbox", "runtime-peer-sandbox"
							spec.Runtimes = append(spec.Runtimes, contracts.CompositionRuntimeRef{InstanceCode: "peer", Code: store.runtime.Code, ImageVersion: store.image.Version})
							if !sourceRuntime {
								client := store.components["client"]
								client.ResourceSpec.Bindings[0].Endpoint = "rpc"
								store.components["client"] = client
							}
						} else {
							ports := []workload.PortSpec{{Name: portName, ContainerPort: containerPort, ServicePort: servicePort, Protocol: "TCP"}}
							if mapping == "collision" {
								ports = append(ports, workload.PortSpec{Name: "api", ContainerPort: 9001, Protocol: "TCP"})
							}
							store.components["database"] = Tool{ID: 10, Code: "database", Category: "infra", Kind: SandboxToolKindInfra, Status: ToolStatusAvailable,
								ResourceSpec: ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "worker", ImageURL: store.image.ImageURL, MountWorkspace: &noWorkspace, Ports: ports}},
									Services:     []workload.ServiceSpec{{Name: targetService, Component: "worker", Ports: []workload.ServicePortSpec{{Name: "api", Port: servicePort, TargetPort: portName, Protocol: "TCP"}}}},
									Capabilities: CapabilityConstraints{Placement: "sandbox", Provides: []string{"test-api"}}}}
							spec.Infra = []contracts.CompositionComponentRef{{Code: "database", Selection: "explicit"}}
						}
						spec.Links = []contracts.CompositionLink{{SourceComponent: source, SourceEndpoint: sourceEndpoint, TargetComponent: target, TargetEndpoint: targetEndpoint,
							Protocol: protocol, AccessScope: "sandbox", ConfigBinding: "env:TARGET"}}
						snapshot, err := CompilePlatformSandboxComposition(context.Background(), networkCompositionStore{store}, spec)
						if err != nil {
							t.Fatal(err)
						}
						plan, err := svc.planForSnapshot(context.Background(), snapshot)
						if err != nil {
							t.Fatal(err)
						}
						var env []workload.EnvVarSpec
						if sourceRuntime {
							runtime, _ := plan.RuntimeInstance(source)
							env = runtime.Runtime.AdapterSpec.RuntimeContainer.Env
						} else {
							for _, tool := range plan.Tools {
								if tool.Code == source {
									env = tool.ResourceSpec.Components[0].Env
								}
							}
						}
						want := fmt.Sprintf("%s:%d", targetService, servicePort)
						if protocol != "TCP" && protocol != "GRPC" {
							want = strings.ToLower(protocol) + "://" + want
						}
						if len(env) != 1 || env[0].Name != "TARGET" || env[0].Value != want {
							t.Fatalf("connection address=%+v want=%s", env, want)
						}
						orchestrator := &K8sOrchestrator{}
						policies := append(orchestrator.allowSandboxPodLinkPolicies(plan), orchestrator.allowToolPodLinkPolicies(plan)...)
						if len(policies) != 2 {
							t.Fatalf("expected two link policies, got %d", len(policies))
						}
						for _, policy := range policies {
							if len(policy.Spec.Ingress) > 0 {
								rule := policy.Spec.Ingress[0]
								if policy.Spec.PodSelector.MatchLabels["chaimir.io/pod-role"] != targetRole || rule.From[0].PodSelector.MatchLabels["chaimir.io/pod-role"] != sourceRole || len(rule.Ports) != 1 || rule.Ports[0].Port.IntVal != containerPort {
									t.Fatalf("ingress does not match target Pod port: %+v", policy.Spec)
								}
							} else {
								rule := policy.Spec.Egress[0]
								if policy.Spec.PodSelector.MatchLabels["chaimir.io/pod-role"] != sourceRole || rule.To[0].PodSelector.MatchLabels["chaimir.io/pod-role"] != targetRole || len(rule.Ports) != 1 || rule.Ports[0].Port.IntVal != containerPort {
									t.Fatalf("egress does not match target Pod port: %+v", policy.Spec)
								}
							}
						}
					})
				}
			}
		}
	}
}

func TestFrozenToolNetworkPortsResolveBeforePolicy(t *testing.T) {
	for _, target := range []string{"local", "runtime", "tool"} {
		for _, tt := range []struct {
			name  string
			ports []workload.NetworkPortRef
			valid bool
		}{
			{"named", []workload.NetworkPortRef{{Name: "rpc"}}, true},
			{"name-overrides-number", []workload.NetworkPortRef{{Name: "rpc", Port: 8000}}, true},
			{"number", []workload.NetworkPortRef{{Port: 9000}}, true},
			{"unknown-name", []workload.NetworkPortRef{{Name: "missing", Port: 9000}}, false},
			{"unknown-number", []workload.NetworkPortRef{{Port: 8000}}, false},
			{"duplicate", []workload.NetworkPortRef{{Name: "rpc"}, {Port: 9000}}, false},
			{"empty", nil, false},
		} {
			t.Run(target+"/"+tt.name, func(t *testing.T) {
				svc, store := networkCompositionFixture(t)
				store.runtime.AdapterSpec.RuntimeContainer.Ports = []workload.PortSpec{{Name: "rpc", ContainerPort: 9000, ServicePort: 8000, Protocol: "TCP"}}
				noWorkspace := false
				to := "backend"
				if target == "runtime" {
					to = "chain-sandbox"
				} else if target == "tool" {
					to = toolComponentPodName("database", "backend")
				}
				store.components["database"] = Tool{ID: 10, Code: "database", Category: "infra", Kind: SandboxToolKindInfra, Status: ToolStatusAvailable,
					ResourceSpec: ToolResourceSpec{Capabilities: CapabilityConstraints{Placement: "sandbox"},
						Components: []workload.ComponentSpec{
							{Name: "worker", ImageURL: store.image.ImageURL, MountWorkspace: &noWorkspace},
							{Name: "backend", ImageURL: store.image.ImageURL, MountWorkspace: &noWorkspace, Ports: []workload.PortSpec{{Name: "rpc", ContainerPort: 9000, Protocol: "TCP"}}},
						},
						NetworkRules: []workload.NetworkRuleSpec{{Name: "read-api", From: "worker", To: to, Ports: append([]workload.NetworkPortRef(nil), tt.ports...)}},
					}}
				spec := contracts.SandboxCompositionSpec{ID: "named-network", AccessProfile: contracts.SandboxAccessJudgePrivate, WorkspaceRuntimeInstance: "chain",
					Runtimes: []contracts.CompositionRuntimeRef{{InstanceCode: "chain", Code: store.runtime.Code, ImageVersion: store.image.Version}},
					Infra:    []contracts.CompositionComponentRef{{Code: "database", Selection: "explicit"}}}
				snapshot, err := CompilePlatformSandboxComposition(context.Background(), networkCompositionStore{store}, spec)
				if !tt.valid {
					if err == nil {
						t.Fatal("invalid network port reached frozen snapshot")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				var frozen ToolResourceSpec
				if err := json.Unmarshal(snapshot.Components[0].ResourceSpec, &frozen); err != nil {
					t.Fatal(err)
				}
				if got := frozen.NetworkRules[0].Ports[0].Port; got != 9000 {
					t.Errorf("snapshot did not resolve named port: got=%d", got)
				}
				// 冻结装载也必须解析名称，不能依赖编译时留在内存里的数值。
				if tt.ports[0].Name != "" {
					frozen.NetworkRules[0].Ports[0].Port = 0
					snapshot.Components[0].ResourceSpec, err = json.Marshal(frozen)
					if err != nil {
						t.Fatal(err)
					}
					snapshot.Digest = platformSnapshotDigest(t, snapshot)
				}
				plan, err := svc.planForSnapshot(context.Background(), snapshot)
				if err != nil {
					t.Fatal(err)
				}
				for _, policy := range (&K8sOrchestrator{}).allowToolPodLinkPolicies(plan) {
					if len(policy.Spec.Ingress) > 0 {
						if got := policy.Spec.Ingress[0].Ports[0].Port.IntVal; got != 9000 {
							t.Errorf("ingress uses unresolved port %d", got)
						}
					} else if got := policy.Spec.Egress[0].Ports[0].Port.IntVal; got != 9000 {
						t.Errorf("egress uses unresolved port %d", got)
					}
				}
			})
		}
	}
}

func TestCompositionRejectsUnsupportedLinkProtocols(t *testing.T) {
	for _, protocol := range []string{"UDP", "SCTP", "ICMP", ""} {
		t.Run(protocol, func(t *testing.T) {
			_, store := networkCompositionFixture(t)
			transport := "TCP"
			if protocol == "UDP" {
				transport = "UDP"
			}
			store.runtime.AdapterSpec.RuntimeContainer.Ports = []workload.PortSpec{{Name: "rpc", ContainerPort: 9000, ServicePort: 8000, Protocol: transport}}
			spec := contracts.SandboxCompositionSpec{ID: "unsupported-protocol", AccessProfile: contracts.SandboxAccessJudgePrivate, WorkspaceRuntimeInstance: "chain",
				Runtimes: []contracts.CompositionRuntimeRef{
					{InstanceCode: "chain", Code: store.runtime.Code, ImageVersion: store.image.Version},
					{InstanceCode: "peer", Code: store.runtime.Code, ImageVersion: store.image.Version},
				},
				Links: []contracts.CompositionLink{{SourceComponent: "chain", SourceEndpoint: "rpc", TargetComponent: "peer", TargetEndpoint: "rpc", Protocol: protocol, AccessScope: "sandbox", ConfigBinding: "env:TARGET"}},
			}
			if _, err := CompilePlatformSandboxComposition(context.Background(), networkCompositionStore{store}, spec); err == nil {
				t.Fatal("unsupported protocol produced a frozen connection")
			}
		})
	}
}

func TestServiceAdmissionMatchesCompilationAndFrozenLoading(t *testing.T) {
	for _, mode := range []string{"distinct", "service-default-tcp", "service-udp", "tool-tool", "tool-runtime", "same-tool", "invalid-name", "runtime-port-collision", "runtime-invalid-port-name", "unknown-component", "unknown-target-port", "service-tcp-to-udp", "service-udp-to-tcp", "route-duplicate", "route-udp", "route-query", "route-fragment"} {
		for _, entry := range []string{"compile", "frozen-load"} {
			t.Run(mode+"/"+entry, func(t *testing.T) {
				svc, store := networkCompositionFixture(t)
				store.runtime.AdapterSpec.RuntimeContainer.Ports = []workload.PortSpec{{Name: "rpc", ContainerPort: 9000, ServicePort: 8000, Protocol: "TCP"}}
				noWorkspace := false
				for i, code := range []string{"left", "right"} {
					store.components[code] = Tool{ID: int64(10 + i), Code: code, Category: "infra", Kind: SandboxToolKindInfra, Status: ToolStatusAvailable,
						ResourceSpec: ToolResourceSpec{Capabilities: CapabilityConstraints{Placement: "sandbox"},
							Components: []workload.ComponentSpec{{Name: "worker", ImageURL: store.image.ImageURL, MountWorkspace: &noWorkspace,
								Ports: []workload.PortSpec{{Name: "api", ContainerPort: 9000, Protocol: "TCP"}}}},
							Services: []workload.ServiceSpec{{Name: "service-" + code, Component: "worker", Ports: []workload.ServicePortSpec{{Name: "api", Port: 8000, TargetPort: "api", Protocol: "TCP"}}}},
						}}
				}
				spec := contracts.SandboxCompositionSpec{ID: "service-admission", AccessProfile: contracts.SandboxAccessJudgePrivate, WorkspaceRuntimeInstance: "chain",
					Runtimes: []contracts.CompositionRuntimeRef{{InstanceCode: "chain", Code: store.runtime.Code, ImageVersion: store.image.Version}},
					Infra:    []contracts.CompositionComponentRef{{Code: "left", Selection: "explicit"}, {Code: "right", Selection: "explicit"}}}
				var snapshot contracts.SandboxCompositionSnapshot
				var err error
				if entry == "frozen-load" {
					snapshot, err = CompilePlatformSandboxComposition(context.Background(), networkCompositionStore{store}, spec)
					if err != nil {
						t.Fatal(err)
					}
				}
				right := store.components["right"]
				switch mode {
				case "tool-tool":
					right.ResourceSpec.Services[0].Name = "service-left"
				case "tool-runtime":
					right.ResourceSpec.Services[0].Name = "runtime-chain-sandbox"
				case "same-tool":
					right.ResourceSpec.Services = append(right.ResourceSpec.Services, right.ResourceSpec.Services[0])
				case "invalid-name":
					right.ResourceSpec.Services[0].Name = "1invalid"
				case "runtime-port-collision":
					store.runtime.AdapterSpec.RuntimeContainer.Ports = append(store.runtime.AdapterSpec.RuntimeContainer.Ports,
						workload.PortSpec{Name: "rpc-alt", ContainerPort: 9001, ServicePort: 8000, Protocol: "TCP"})
				case "runtime-invalid-port-name":
					store.runtime.AdapterSpec.RuntimeContainer.Ports[0].Name = "rpc_invalid"
				case "unknown-component":
					right.ResourceSpec.Services[0].Component = "missing"
				case "unknown-target-port":
					right.ResourceSpec.Services[0].Ports[0].TargetPort = "missing"
				case "service-tcp-to-udp":
					right.ResourceSpec.Components[0].Ports[0].Protocol = "UDP"
				case "service-udp-to-tcp":
					right.ResourceSpec.Services[0].Ports[0].Protocol = "UDP"
				case "service-default-tcp":
					right.ResourceSpec.Services[0].Ports[0].Protocol = ""
					right.ResourceSpec.Components[0].Ports[0].Protocol = ""
				case "service-udp":
					right.ResourceSpec.Services[0].Ports[0].Protocol = "UDP"
					right.ResourceSpec.Components[0].Ports[0].Protocol = "UDP"
				case "route-duplicate", "route-udp", "route-query", "route-fragment":
					right.ResourceSpec.Routes = []workload.RouteSpec{{PathPrefix: "/", Service: "service-right", Port: "api"}}
					switch mode {
					case "route-duplicate":
						right.ResourceSpec.Routes = append(right.ResourceSpec.Routes, right.ResourceSpec.Routes[0])
					case "route-udp":
						right.ResourceSpec.Services[0].Ports[0].Protocol = "UDP"
						right.ResourceSpec.Components[0].Ports[0].Protocol = "UDP"
					case "route-query":
						right.ResourceSpec.Routes[0].PathPrefix = "/api?bad=1"
					case "route-fragment":
						right.ResourceSpec.Routes[0].PathPrefix = "/api#bad"
					}
				}
				store.components["right"] = right
				if entry == "compile" {
					_, err = CompilePlatformSandboxComposition(context.Background(), networkCompositionStore{store}, spec)
				} else {
					snapshot.Runtimes[0].AdapterSpec, err = json.Marshal(store.runtime.AdapterSpec)
					if err != nil {
						t.Fatal(err)
					}
					for i := range snapshot.Components {
						component := &snapshot.Components[i]
						component.ResourceSpec, err = json.Marshal(store.components[component.Code].ResourceSpec)
						if err != nil {
							t.Fatal(err)
						}
					}
					snapshot.Digest = platformSnapshotDigest(t, snapshot)
					_, err = svc.planForSnapshot(context.Background(), snapshot)
				}
				validMode := mode == "distinct" || mode == "service-default-tcp" || mode == "service-udp"
				if (err == nil) != validMode {
					t.Fatalf("service admission disagrees: error=%v wantValid=%t", err, validMode)
				}
			})
		}
	}
}
