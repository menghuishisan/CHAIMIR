package sandbox

import (
	"chaimir/internal/platform/response"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	corev1 "k8s.io/api/core/v1"
)

type retainedResumeTx struct {
	*terminalRecheckTx
	quota          TenantQuota
	count          int64
	quotaErr       error
	countErr       error
	quotaLocked    bool
	startRequested bool
}

func TestStartupOrderOnlyIncludesRequiredComponents(t *testing.T) {
	for _, mode := range []string{"required", "optional", "duplicate", "cycle", "unknown-source", "unknown-target"} {
		t.Run(mode, func(t *testing.T) {
			spec := contracts.SandboxCompositionSpec{
				Runtimes: []contracts.CompositionRuntimeRef{{InstanceCode: "chain"}, {InstanceCode: "independent-chain"}},
				Links:    []contracts.CompositionLink{{SourceComponent: "chain", TargetComponent: "database", RequiredAtStart: true}},
			}
			refs := []contracts.CompositionComponentRef{{Code: "database"}, {Code: "editor"}}
			switch mode {
			case "optional":
				spec.Links[0].RequiredAtStart = false
			case "duplicate":
				spec.Links = append(spec.Links, spec.Links[0])
			case "cycle":
				spec.Links = append(spec.Links, contracts.CompositionLink{SourceComponent: "database", TargetComponent: "chain", RequiredAtStart: true})
			case "unknown-source":
				spec.Links[0].SourceComponent = "missing"
			case "unknown-target":
				spec.Links[0].TargetComponent = "missing"
			}
			order, err := compileStartupOrder(spec, refs)
			if mode == "cycle" || strings.HasPrefix(mode, "unknown-") {
				if err == nil {
					t.Fatal("invalid dependency graph was accepted")
				}
				return
			}
			want := []string{"database", "chain"}
			if mode == "optional" {
				want = nil
			}
			if err != nil || !slices.Equal(order, want) {
				t.Fatalf("startup order=%v want=%v err=%v", order, want, err)
			}
		})
	}
}

func TestStartupCreatesWholeComponentBeforeWaiting(t *testing.T) {
	for _, mode := range []string{"complete", "provider-failure", "create-failure", "no-dependencies", "reset-subset"} {
		t.Run(mode, func(t *testing.T) {
			plan := volumePodPlan()
			plan.Runtimes[0].Runtime.AdapterSpec.StartupOrder = []string{"chain", "editor"}
			plan.Tools[0].ResourceSpec.Components = append(plan.Tools[0].ResourceSpec.Components, workload.ComponentSpec{Name: "backend"})
			plan.Tools = append(plan.Tools, Tool{Code: "analyzer", Category: "tool", Kind: SandboxToolKindCommand, ResourceSpec: ToolResourceSpec{
				Components: []workload.ComponentSpec{{Name: "analyzer"}},
			}})
			if mode == "no-dependencies" {
				plan.Runtimes[0].Runtime.AdapterSpec.StartupOrder = nil
			}
			o := &K8sOrchestrator{cfg: config.SandboxConfig{DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
			pods, err := o.podsForPlan(plan)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "reset-subset" {
				pods = slices.DeleteFunc(pods, func(pod *corev1.Pod) bool { return pod.Name != "chain-node" })
			}
			created, ready := map[string]bool{}, map[string]bool{}
			failure := errors.New("dependency failed")
			_, err = createPodsInStartupOrder(plan, pods, func(pod *corev1.Pod) (*corev1.Pod, error) {
				if mode == "create-failure" && pod.Name == "chain-worker" {
					return nil, failure
				}
				if mode != "no-dependencies" && strings.HasPrefix(pod.Name, "tool-editor-") {
					for _, name := range []string{"chain-node", "chain-worker", "chain-workspace"} {
						if !ready[name] {
							return nil, fmt.Errorf("consumer started before provider %s was ready", name)
						}
					}
				}
				created[pod.Name] = true
				return pod, nil
			}, func(waiting []*corev1.Pod) error {
				if mode == "no-dependencies" {
					return errors.New("independent component was forced to wait")
				}
				if mode == "provider-failure" {
					return failure
				}
				for _, pod := range waiting {
					name := pod.Name
					if strings.Contains(name, "analyzer") {
						return errors.New("unrelated command tool blocks startup")
					}
					if mode != "reset-subset" {
						for _, peer := range pods {
							if strings.HasPrefix(peer.Name, "chain-") == strings.HasPrefix(name, "chain-") && !strings.Contains(peer.Name, "analyzer") && !created[peer.Name] {
								return fmt.Errorf("waited for %s before creating peer %s", name, peer.Name)
							}
						}
					}
					ready[name] = true
				}
				return nil
			})
			if mode == "provider-failure" || mode == "create-failure" {
				if !errors.Is(err, failure) || created[toolComponentPodName("editor", "editor")] {
					t.Fatalf("failure did not stop consumers: err=%v created=%v", err, created)
				}
				return
			}
			if err != nil || len(created) != len(pods) {
				t.Fatalf("startup failed: err=%v created=%v", err, created)
			}
		})
	}
}

func TestToolWorkloadsReceiveDeclaredNetworkLinks(t *testing.T) {
	for _, kind := range []int16{SandboxToolKindCommand, SandboxToolKindWebEmbed, SandboxToolKindInfra} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			plan := volumePodPlan()
			tool := &plan.Tools[0]
			tool.Kind = kind
			if kind == SandboxToolKindInfra {
				tool.Category = "infra"
			}
			tool.ResourceSpec.NetworkRules = []workload.NetworkRuleSpec{{Name: "read-chain", From: "editor", To: "chain-node",
				Ports: []workload.NetworkPortRef{{Port: 8545}}}}
			policies := (&K8sOrchestrator{}).allowToolPodLinkPolicies(plan)
			if len(policies) != 2 {
				t.Fatalf("declared link needs ingress and egress: got=%d", len(policies))
			}
			from := toolComponentPodName("editor", "editor")
			for _, policy := range policies {
				if len(policy.Spec.Ingress) > 0 {
					if policy.Spec.PodSelector.MatchLabels["chaimir.io/pod-role"] != "chain-node" ||
						policy.Spec.Ingress[0].From[0].PodSelector.MatchLabels["chaimir.io/pod-role"] != from ||
						policy.Spec.Ingress[0].Ports[0].Port.IntVal != 8545 {
						t.Fatal("ingress does not match declared target, source or port")
					}
				} else if policy.Spec.PodSelector.MatchLabels["chaimir.io/pod-role"] != from ||
					policy.Spec.Egress[0].To[0].PodSelector.MatchLabels["chaimir.io/pod-role"] != "chain-node" ||
					policy.Spec.Egress[0].Ports[0].Port.IntVal != 8545 {
					t.Fatal("egress does not match declared source, target or port")
				}
			}
		})
	}
}

func (tx *retainedResumeTx) GetCompositionPrepullForUpdate(context.Context, int64, string) (CompositionPrepull, error) {
	return CompositionPrepull{Status: ImagePrepullSucceeded}, nil
}

func (tx *retainedResumeTx) GetTenantQuotaForUpdate(context.Context, int64) (TenantQuota, error) {
	tx.quotaLocked = true
	return tx.quota, tx.quotaErr
}

func (tx *retainedResumeTx) GetSandboxForUpdate(context.Context, int64, int64) (Sandbox, error) {
	if !tx.quotaLocked {
		return Sandbox{}, errors.New("sandbox locked before tenant quota")
	}
	return tx.sandbox, nil
}

func (tx *retainedResumeTx) CountActiveSandboxes(context.Context, int64) (int64, error) {
	if !tx.quotaLocked {
		return 0, errors.New("capacity read without tenant quota lock")
	}
	return tx.count, tx.countErr
}

func (tx *retainedResumeTx) StartRetainedSandbox(context.Context, int64, int64) (bool, error) {
	tx.startRequested = true
	// 停在条件更新处；本组只验证资源启动前的准入和事务顺序。
	return false, nil
}

type retainedResumeStore struct {
	Store
	tx *retainedResumeTx
}

func (s retainedResumeStore) TenantTx(ctx context.Context, _ int64, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s.tx)
}

func (s retainedResumeStore) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, s.tx)
}

func TestRetainedResumeChecksCapacityBeforeStateChange(t *testing.T) {
	readFailure := errors.New("quota storage unavailable")
	for _, tt := range []struct {
		name      string
		status    int16
		count     int64
		cpu       int32
		memory    int32
		quotaErr  error
		countErr  error
		wantError error
		wantStart bool
	}{
		{name: "concurrency-full", status: SandboxStatusDestroyed, count: 2, cpu: 3, memory: 3072, wantError: apperr.ErrSandboxQuotaExceeded},
		{name: "cpu-full", status: SandboxStatusDestroyed, count: 1, cpu: 1, memory: 3072, wantError: apperr.ErrSandboxResourceQuotaExceeded},
		{name: "memory-full", status: SandboxStatusDestroyed, count: 1, cpu: 3, memory: 1024, wantError: apperr.ErrSandboxResourceQuotaExceeded},
		{name: "paused-already-counted", status: SandboxStatusPaused, count: 2, cpu: 2, memory: 2048, wantError: apperr.ErrSandboxStateInvalid, wantStart: true},
		{name: "snapshot-capacity-available", status: SandboxStatusDestroyed, count: 1, cpu: 2, memory: 2048, wantError: apperr.ErrSandboxStateInvalid, wantStart: true},
		{name: "status-changed", status: SandboxStatusRunning, count: 1, cpu: 2, memory: 2048, wantError: apperr.ErrSandboxStateInvalid},
		{name: "quota-read-failure", status: SandboxStatusDestroyed, quotaErr: readFailure, wantError: readFailure},
		{name: "capacity-read-failure", status: SandboxStatusDestroyed, countErr: readFailure, wantError: readFailure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, base, _ := terminalRecheckFixture(t)
			var snapshot contracts.SandboxCompositionSnapshot
			if err := json.Unmarshal(base.sandbox.CompositionSnapshot, &snapshot); err != nil {
				t.Fatal(err)
			}
			snapshot.Components = nil
			digest := "sha256:" + strings.Repeat("a", 64)
			image := "registry.example/runtime@" + digest
			snapshot.Runtimes[0].ImageURL = image
			var err error
			snapshot.Digest, err = contracts.CanonicalSnapshotDigest(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			base.sandbox.CompositionSnapshot, err = json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			base.sandbox.CompositionDigest = snapshot.Digest
			base.sandbox.Status = tt.status
			base.image.ImageURL = image
			tx := &retainedResumeTx{terminalRecheckTx: base, count: tt.count, quotaErr: tt.quotaErr, countErr: tt.countErr,
				quota: TenantQuota{MaxConcurrentSandbox: 2, MaxCPU: tt.cpu, MaxMemoryMB: tt.memory}}
			svc.store = retainedResumeStore{tx: tx}
			svc.cfg = config.SandboxConfig{MaxCPU: "1", MaxMemory: "1Gi", MaxPods: "5", DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "100m", DefaultMemory: "64Mi", ImageRegistry: "registry.example", PlatformImageAttestations: []config.PlatformImageAttestation{
				{ImageURL: image, Digest: digest, CosignVerified: true, TrivyStatus: "passed"},
			}}
			if err := svc.resumeRetainedSandbox(context.Background(), base.sandbox); !errors.Is(err, tt.wantError) {
				t.Fatalf("resume error=%v want=%v", err, tt.wantError)
			}
			if tx.startRequested != tt.wantStart || base.sandbox.Status != tt.status {
				t.Fatalf("capacity gate changed lifecycle: start=%v status=%d", tx.startRequested, base.sandbox.Status)
			}
		})
	}
}

type recycleIdentityBus struct {
	eventbus.Bus
	wire       []byte
	publishErr error
}

func (bus *recycleIdentityBus) Publish(_ context.Context, subject string, payload any) error {
	if subject != contracts.SubjectSandboxRecycled {
		return errors.New("unexpected recycle event subject")
	}
	var err error
	bus.wire, err = json.Marshal(payload)
	return errors.Join(err, bus.publishErr)
}

type recycleOutboxDeliveryTx struct {
	TxStore
	published int
	failed    int
}

func (tx *recycleOutboxDeliveryTx) MarkSandboxRecycleOutboxPublished(context.Context, int64, int64, string) (SandboxRecycleOutbox, error) {
	tx.published++
	return SandboxRecycleOutbox{}, nil
}

func (tx *recycleOutboxDeliveryTx) MarkSandboxRecycleOutboxFailed(context.Context, int64, int64, string, string) (SandboxRecycleOutbox, error) {
	tx.failed++
	return SandboxRecycleOutbox{}, nil
}

type recycleOutboxDeliveryStore struct {
	Store
	tx *recycleOutboxDeliveryTx
}

func (store recycleOutboxDeliveryStore) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, store.tx)
}

func TestRecycleOutboxWireRetainsLifecycleScope(t *testing.T) {
	for _, publishFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(publishFailure), func(t *testing.T) {
			scope := "experiment:course:instance:3:run:4"
			item := SandboxRecycleOutbox{ID: 5, TenantID: 1, SandboxID: 2,
				SourceRef: "experiment:course:instance:3", ScopeRef: scope}
			tx := &recycleOutboxDeliveryTx{}
			bus := &recycleIdentityBus{}
			if publishFailure {
				bus.publishErr = errors.New("broker unavailable")
			}
			svc := &Service{store: recycleOutboxDeliveryStore{tx: tx}, bus: bus}
			err := svc.publishSandboxRecycleOutboxItem(context.Background(), item)
			if (err != nil) != publishFailure {
				t.Fatalf("delivery error=%v wantFailure=%v", err, publishFailure)
			}
			var wire map[string]any
			if err := json.Unmarshal(bus.wire, &wire); err != nil {
				t.Fatal(err)
			}
			if wire["scope_ref"] != scope || wire["source_ref"] != item.SourceRef || wire["sandbox_id"] != float64(item.SandboxID) {
				t.Fatalf("recycle identity changed before publication: %s", bus.wire)
			}
			if publishFailure && (tx.published != 0 || tx.failed != 1) || !publishFailure && (tx.published != 1 || tx.failed != 0) {
				t.Fatalf("incorrect delivery state: published=%d failed=%d", tx.published, tx.failed)
			}
		})
	}
}

type outboxErrorBatch struct {
	Store
	TxStore
	t                       *testing.T
	items                   []SandboxRecycleOutbox
	claimErr                error
	publishErr              map[int64]error
	markErr                 map[int64]error
	published, failed, sent []int64
}

func (b *outboxErrorBatch) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	return fn(ctx, b)
}

func (b *outboxErrorBatch) ClaimPendingSandboxRecycleOutbox(_ context.Context, _ int32, _ int32, _, _ time.Time) ([]SandboxRecycleOutbox, error) {
	return b.items, b.claimErr
}

func (b *outboxErrorBatch) checkLease(tenantID, id int64, token string) {
	b.t.Helper()
	item := b.items[id-1]
	if tenantID != item.TenantID || token != item.LeaseToken {
		b.t.Fatalf("outbox identity changed: tenant=%d id=%d lease=%q", tenantID, id, token)
	}
}

func (b *outboxErrorBatch) MarkSandboxRecycleOutboxPublished(_ context.Context, tenantID, id int64, token string) (SandboxRecycleOutbox, error) {
	b.checkLease(tenantID, id, token)
	b.published = append(b.published, id)
	return SandboxRecycleOutbox{}, b.markErr[id]
}

func (b *outboxErrorBatch) MarkSandboxRecycleOutboxFailed(_ context.Context, tenantID, id int64, reason, token string) (SandboxRecycleOutbox, error) {
	b.checkLease(tenantID, id, token)
	if cause := b.publishErr[id]; cause == nil || reason != cause.Error() {
		b.t.Fatalf("wrong persisted failure for id %d: %q", id, reason)
	}
	b.failed = append(b.failed, id)
	return SandboxRecycleOutbox{}, b.markErr[id]
}

type outboxErrorBus struct {
	eventbus.Bus
	batch *outboxErrorBatch
}

func (bus outboxErrorBus) Publish(ctx context.Context, subject string, payload any) error {
	b := bus.batch
	event, ok := payload.(contracts.SandboxRecycledEvent)
	if !ok || subject != contracts.SubjectSandboxRecycled {
		b.t.Fatalf("unexpected event subject=%s payload=%T", subject, payload)
	}
	item := b.items[len(b.sent)]
	expected := contracts.SandboxRecycledEvent{TenantID: item.TenantID, TraceID: item.TraceID, SandboxID: item.SandboxID, SourceRef: item.SourceRef, ScopeRef: item.ScopeRef, Reason: item.Reason, RecycledAt: item.RecycledAt}
	if event != expected || response.TraceFromContext(ctx) != item.TraceID {
		b.t.Fatalf("outbox payload or trace changed: got=%+v want=%+v", event, expected)
	}
	b.sent = append(b.sent, item.ID)
	return b.publishErr[item.ID]
}

func TestOutboxBatchPreservesEveryFailure(t *testing.T) {
	for _, mode := range []string{"empty", "success", "publish", "published-mark", "failure-mark", "claim"} {
		t.Run(mode, func(t *testing.T) {
			first, second := errors.New("publish first"), errors.New("publish second")
			markFirst, markSecond := errors.New("mark first"), errors.New("mark second")
			b := &outboxErrorBatch{t: t, publishErr: map[int64]error{}, markErr: map[int64]error{}}
			at := time.Date(2026, 11, 28, 0, 0, 0, 0, time.UTC)
			for id := int64(1); id <= 3; id++ {
				b.items = append(b.items, SandboxRecycleOutbox{ID: id, TenantID: id + 10, TraceID: fmt.Sprint("outbox-", id), LeaseToken: fmt.Sprint("lease-", id), SandboxID: id + 20, SourceRef: fmt.Sprint("source:", id), ScopeRef: fmt.Sprint("scope:", id), Reason: "expired", RecycledAt: at})
			}
			var expected []error
			switch mode {
			case "empty":
				b.items = nil
			case "publish", "failure-mark":
				b.publishErr[1], b.publishErr[2] = first, second
				expected = append(expected, first, second)
				if mode == "failure-mark" {
					b.markErr[1], b.markErr[2] = markFirst, markSecond
					expected = append(expected, markFirst, markSecond)
				}
			case "published-mark":
				b.markErr[1], b.markErr[2] = markFirst, markSecond
				expected = append(expected, markFirst, markSecond)
			case "claim":
				b.claimErr = first
				expected = append(expected, first)
			}

			svc := &Service{store: b, bus: outboxErrorBus{batch: b}, cfg: config.SandboxConfig{RecycleOutboxBatchSize: 3, RecycleOutboxStaleMs: 1000, RecycleOutboxMaxAttempts: 5}}
			err := svc.RunSandboxRecycleOutboxOnce(context.Background())
			for _, cause := range expected {
				if !errors.Is(err, cause) {
					t.Errorf("lost outbox cause %q: %v", cause, err)
				}
			}
			if len(expected) == 0 && err != nil || len(expected) > 0 && !errors.Is(err, apperr.ErrSandboxRecycleEventPublishFailed) {
				t.Errorf("wrong worker result: %v", err)
			}
			wantSent := len(b.items)
			if mode == "claim" {
				wantSent = 0
			}
			if len(b.sent) != wantSent || len(b.published)+len(b.failed) != wantSent {
				t.Errorf("batch stopped or acknowledgement duplicated: sent=%v published=%v failed=%v", b.sent, b.published, b.failed)
			}
			for _, id := range b.published {
				if b.publishErr[id] != nil {
					t.Errorf("failed publish acknowledged as success: id=%d", id)
				}
			}
			if mode != "claim" && len(b.failed) != len(b.publishErr) {
				t.Errorf("missing failed acknowledgements: %v", b.failed)
			}

		})
	}
}
