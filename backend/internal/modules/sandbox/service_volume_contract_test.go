// 沙箱卷域回归使用独立 Kubernetes HTTP 夹具，不依赖集群或本地历史测试。
package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/config"
	platformk8s "chaimir/internal/platform/k8s"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes/fake"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
)

func testSnapshotOrchestrator(t *testing.T, handler http.HandlerFunc) *K8sOrchestrator {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	kubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
	data := fmt.Sprintf("apiVersion: v1\nkind: Config\nclusters:\n- name: test\n  cluster:\n    server: %s\ncontexts:\n- name: test\n  context:\n    cluster: test\n    user: test\ncurrent-context: test\nusers:\n- name: test\n  user: {}\n", server.URL)
	if err := os.WriteFile(kubeconfig, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.SandboxConfig{KubeconfigPath: kubeconfig, ReadyTimeoutSeconds: 2, ReadyPollIntervalSeconds: 1}
	client, err := platformk8s.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return NewK8sOrchestrator(client, cfg)
}

// TestToolReadinessChecksWholeComponentGroup 防止准备状态遮蔽其他组件的失败和查询错误。
func TestToolReadinessChecksWholeComponentGroup(t *testing.T) {
	for _, tt := range []struct {
		name, first, second string
		want                error
		queryFailure        bool
	}{
		{name: "ready"},
		{name: "starting", first: "starting", want: errToolNotReady},
		{name: "missing-before-crash", first: "missing", second: "crash", want: errToolFailed},
		{name: "starting-before-crash", first: "starting", second: "crash", want: errToolFailed},
		{name: "deleting-before-crash", first: "deleting", second: "crash", want: errToolFailed},
		{name: "starting-before-init-failure", first: "starting", second: "init-failure", want: errToolFailed},
		{name: "starting-before-expired", first: "starting", second: "expired", want: errToolFailed},
		{name: "missing-status-expired", first: "missing-status-expired", want: errToolFailed},
		{name: "starting-before-query-error", first: "starting", second: "query-error", queryFailure: true},
		{name: "failed-before-query-error", first: "crash", second: "query-error", queryFailure: true},
		{name: "query-error-before-failed", first: "query-error", second: "crash", queryFailure: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			o := testSnapshotOrchestrator(t, func(w http.ResponseWriter, r *http.Request) {
				name, state := "first", tt.first
				if strings.HasSuffix(r.URL.Path, "-second") {
					name, state = "second", tt.second
				}
				w.Header().Set("Content-Type", "application/json")
				if state == "missing" || state == "query-error" {
					code, reason := http.StatusNotFound, metav1.StatusReasonNotFound
					if state == "query-error" {
						code, reason = http.StatusForbidden, metav1.StatusReasonForbidden
					}
					w.WriteHeader(code)
					if err := json.NewEncoder(w).Encode(metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Reason: reason, Code: int32(code)}); err != nil {
						t.Error(err)
					}
					return
				}
				pod := &corev1.Pod{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
					ObjectMeta: metav1.ObjectMeta{Name: "tool-checker-" + name, UID: "current", CreationTimestamp: metav1.NewTime(time.Now())},
					Status: corev1.PodStatus{Phase: corev1.PodRunning,
						Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
						ContainerStatuses: []corev1.ContainerStatus{{Name: name, Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
				switch state {
				case "starting", "expired":
					pod.Status.Phase = corev1.PodPending
					if state == "expired" {
						pod.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))
					}
				case "deleting":
					pod.DeletionTimestamp = &pod.CreationTimestamp
				case "crash":
					pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}
				case "init-failure":
					pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "init", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}}}
				case "missing-status-expired":
					pod.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))
					pod.Status.ContainerStatuses = nil
				}
				if err := json.NewEncoder(w).Encode(pod); err != nil {
					t.Error(err)
				}
			})
			tool := Tool{Code: "checker", Kind: SandboxToolKindCommand,
				ResourceSpec: ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "first"}, {Name: "second"}}}}
			err := o.ToolReady(context.Background(), Sandbox{Namespace: "sandbox-test"}, tool)
			if tt.queryFailure {
				if !apierrors.IsForbidden(err) {
					t.Fatalf("component query failure was hidden: %v", err)
				}
			} else if !errors.Is(err, tt.want) {
				t.Fatalf("group readiness=%v, want %v", err, tt.want)
			}
		})
	}
}

// TestToolReadinessRequiresCurrentServiceEndpoints 验证代理只能使用当前 Service 和准确目标端口。
func TestToolReadinessRequiresCurrentServiceEndpoints(t *testing.T) {
	for _, scenario := range []string{
		"current", "default-protocol", "wrong-endpoint-port", "wrong-endpoint-protocol", "missing-endpoint-port",
		"old-service-owner", "missing-service-owner", "deleted-slice", "old-pod", "not-ready", "not-serving", "terminating", "no-address",
		"missing-service", "deleting-service", "wrong-service-port", "wrong-service-target", "wrong-service-protocol", "service-query-error", "slice-query-error",
		"pending-service-before-query-error", "pending-service-before-slice-query-error",
	} {
		t.Run(scenario, func(t *testing.T) {
			ready, notReady := true, false
			portName, targetPort, protocol := "web", int32(8080), corev1.ProtocolTCP
			pod := corev1.Pod{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}, ObjectMeta: metav1.ObjectMeta{Name: "tool-checker-worker", UID: "current-pod"},
				Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
					ContainerStatuses: []corev1.ContainerStatus{{Name: "worker", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
			service := corev1.Service{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"}, ObjectMeta: metav1.ObjectMeta{Name: "checker", UID: "current-service"},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: portName, Port: 80, TargetPort: intstr.FromString("http"), Protocol: corev1.ProtocolTCP}}}}
			slice := discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{APIVersion: "v1", Kind: "Service", Name: service.Name, UID: service.UID}}},
				Ports: []discoveryv1.EndpointPort{{Name: &portName, Port: &targetPort, Protocol: &protocol}},
				Endpoints: []discoveryv1.Endpoint{{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready},
					TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: pod.Name, UID: pod.UID}}}}
			switch scenario {
			case "default-protocol":
				slice.Ports[0].Protocol = nil
			case "wrong-endpoint-port":
				targetPort = 80 // Service 端口不是 EndpointSlice 的容器目标端口。
			case "wrong-endpoint-protocol":
				protocol = corev1.ProtocolUDP
			case "missing-endpoint-port":
				slice.Ports[0].Port = nil
			case "old-service-owner":
				slice.OwnerReferences[0].UID = "old-service"
			case "missing-service-owner":
				slice.OwnerReferences = nil
			case "deleted-slice":
				slice.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			case "old-pod":
				slice.Endpoints[0].TargetRef.UID = "old-pod"
			case "not-ready":
				slice.Endpoints[0].Conditions.Ready = &notReady
			case "not-serving":
				slice.Endpoints[0].Conditions.Serving = &notReady
			case "terminating":
				slice.Endpoints[0].Conditions.Terminating = &ready
			case "no-address":
				slice.Endpoints[0].Addresses = nil
			case "deleting-service":
				service.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			case "wrong-service-port":
				service.Spec.Ports[0].Port++
			case "wrong-service-target":
				service.Spec.Ports[0].TargetPort = intstr.FromString("other")
			case "wrong-service-protocol":
				service.Spec.Ports[0].Protocol = corev1.ProtocolUDP
			}
			o := testSnapshotOrchestrator(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var value any = pod
				isService := strings.HasSuffix(r.URL.Path, "/services/checker")
				isNextService := strings.HasSuffix(r.URL.Path, "/services/checker-next")
				isSlice := strings.Contains(r.URL.Path, "endpointslices")
				if isService || isNextService {
					value = service
				}
				if isSlice {
					value = discoveryv1.EndpointSliceList{TypeMeta: metav1.TypeMeta{APIVersion: "discovery.k8s.io/v1", Kind: "EndpointSliceList"}, Items: []discoveryv1.EndpointSlice{slice}}
				}
				if (isService && (scenario == "missing-service" || scenario == "service-query-error" || strings.HasPrefix(scenario, "pending-service-"))) ||
					(isNextService && scenario == "pending-service-before-query-error") ||
					(isSlice && (scenario == "slice-query-error" || scenario == "pending-service-before-slice-query-error")) {
					code, reason := http.StatusForbidden, metav1.StatusReasonForbidden
					if scenario == "missing-service" || (isService && strings.HasPrefix(scenario, "pending-service-")) {
						code, reason = http.StatusNotFound, metav1.StatusReasonNotFound
					}
					w.WriteHeader(code)
					value = metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Reason: reason, Code: int32(code)}
				}
				if err := json.NewEncoder(w).Encode(value); err != nil {
					t.Error(err)
				}
			})
			tool := Tool{Code: "checker", Kind: SandboxToolKindWebEmbed, ResourceSpec: ToolResourceSpec{
				Components: []workload.ComponentSpec{{Name: "worker", Ports: []workload.PortSpec{{Name: "http", ContainerPort: 8080, Protocol: "TCP"}}}},
				Services:   []workload.ServiceSpec{{Name: "checker", Component: "worker", Ports: []workload.ServicePortSpec{{Name: "web", Port: 80, TargetPort: "http", Protocol: "TCP"}}}},
			}}
			if strings.HasPrefix(scenario, "pending-service-") {
				secondService := tool.ResourceSpec.Services[0]
				secondService.Name = "checker-next"
				tool.ResourceSpec.Services = append(tool.ResourceSpec.Services, secondService)
			}
			err := o.ToolReady(context.Background(), Sandbox{Namespace: "sandbox-test"}, tool)
			switch scenario {
			case "current", "default-protocol":
				if err != nil {
					t.Fatal(err)
				}
			case "service-query-error", "slice-query-error", "pending-service-before-query-error", "pending-service-before-slice-query-error":
				if !apierrors.IsForbidden(err) {
					t.Fatalf("service observation error was hidden: %v", err)
				}
			default:
				if !errors.Is(err, errToolNotReady) {
					t.Fatalf("stale or mismatched endpoint admitted: %v", err)
				}
			}
		})
	}
}

func TestToolControlPlanePortsFollowDeclaredRoutes(t *testing.T) {
	for _, scenario := range []string{"none", "one", "repeated-route", "shared-target", "separate-components", "unknown-service", "unknown-target"} {
		t.Run(scenario, func(t *testing.T) {
			tool := Tool{Code: "checker", ResourceSpec: ToolResourceSpec{
				Components: []workload.ComponentSpec{{Name: "worker", Ports: []workload.PortSpec{{Name: "http", ContainerPort: 8080}}}},
				Services: []workload.ServiceSpec{{Name: "checker", Component: "worker", Ports: []workload.ServicePortSpec{
					{Name: "web", Port: 80, TargetPort: "http"}, {Name: "private", Port: 90, TargetPort: "http"},
				}}},
				Routes: []workload.RouteSpec{{PathPrefix: "/", Service: "checker", Port: "web"}},
			}}
			want := []string{"tool-checker-worker:8080/TCP"}
			switch scenario {
			case "none":
				tool.ResourceSpec.Routes = nil
				want = nil
			case "repeated-route":
				tool.ResourceSpec.Routes = append(tool.ResourceSpec.Routes, workload.RouteSpec{PathPrefix: "/api", Service: "checker", Port: "web"})
			case "shared-target":
				other := tool.ResourceSpec.Services[0]
				other.Name = "checker-other"
				tool.ResourceSpec.Services = append(tool.ResourceSpec.Services, other)
				tool.ResourceSpec.Routes = append(tool.ResourceSpec.Routes, workload.RouteSpec{PathPrefix: "/api", Service: "checker-other", Port: "web"})
			case "separate-components":
				other := tool.ResourceSpec.Components[0]
				other.Name = "other"
				tool.ResourceSpec.Components = append(tool.ResourceSpec.Components, other)
				service := tool.ResourceSpec.Services[0]
				service.Name, service.Component = "checker-other", "other"
				tool.ResourceSpec.Services = append(tool.ResourceSpec.Services, service)
				tool.ResourceSpec.Routes = append(tool.ResourceSpec.Routes, workload.RouteSpec{PathPrefix: "/api", Service: "checker-other", Port: "web"})
				want = append(want, "tool-checker-other:8080/TCP")
			case "unknown-service":
				tool.ResourceSpec.Routes[0].Service = "missing"
				want = nil
			case "unknown-target":
				tool.ResourceSpec.Services[0].Ports[0].TargetPort = "missing"
				want = nil
			}
			var got []string
			for role, ports := range toolControlPlanePorts(tool) {
				for _, port := range ports {
					got = append(got, fmt.Sprintf("%s:%d/%s", role, port.Port.IntVal, *port.Protocol))
				}
			}
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("control plane ports=%v, want %v", got, want)
			}
		})
	}
}

func TestRetainedPVCDoesNotNeedSnapshotDataSource(t *testing.T) {
	existing := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "workspace", Namespace: "sandbox-test", UID: "retained-claim", Labels: map[string]string{"chaimir.io/sandbox-id": "123"}},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
		},
	}
	desired := existing.DeepCopy()
	desired.Spec.DataSource = snapshotSourceForDomain(Sandbox{ID: 123, SnapshotRef: "sandbox-test/snapshot-123-0123456789abcdef0123456789abcdef"}, VolumeDomainSpec{Name: "workspace"})
	client := fake.NewSimpleClientset(existing)
	if err := createOrVerifyPVC(context.Background(), client, desired, true); err != nil {
		t.Fatalf("应复用原始 PVC 而非要求它来自快照: %v", err)
	}
	actual, err := client.CoreV1().PersistentVolumeClaims(existing.Namespace).Get(context.Background(), existing.Name, metav1.GetOptions{})
	if err != nil || actual.Spec.DataSource != nil {
		t.Fatalf("恢复不应修改已有 PVC 的创建来源: pvc=%v err=%v", actual, err)
	}
	mismatch := desired.DeepCopy()
	mismatch.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	if err := createOrVerifyPVC(context.Background(), client, mismatch, true); err == nil {
		t.Fatal("容量漂移必须拒绝")
	}
	// 已从上一代快照恢复的 PVC 仍是当前数据；新快照引用不应改变它的创建来源。
	oldSource := *desired.Spec.DataSource
	oldSource.Name = strings.Replace(oldSource.Name, "0123456789abcdef0123456789abcdef", "11111111111111111111111111111111", 1)
	actual.Spec.DataSource = &oldSource
	if _, err := client.CoreV1().PersistentVolumeClaims(existing.Namespace).Update(context.Background(), actual, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := createOrVerifyPVC(context.Background(), client, desired, true); err != nil {
		t.Fatalf("多次恢复应复用来自上一代快照的原 PVC: %v", err)
	}
}

func TestWorkspacePVCRecoveryPreservesVolumeContents(t *testing.T) {
	const material = "runtime-chain-public-assets"
	for _, tt := range []struct {
		name      string
		phase     int16
		missing   []string
		snapshots []string
		wantError bool
	}{
		{name: "first-start", phase: SandboxPhaseAllocating, missing: []string{"workspace", material}},
		{name: "retained", phase: SandboxPhaseFullyReady},
		{name: "missing-workspace", phase: SandboxPhaseFullyReady, missing: []string{"workspace"}, wantError: true},
		{name: "missing-material", phase: SandboxPhaseFullyReady, missing: []string{material}, wantError: true},
		{name: "interrupted-init", phase: SandboxPhaseInitializing, missing: []string{material}, wantError: true},
		{name: "outside-snapshot", phase: SandboxPhaseFullyReady, missing: []string{"workspace"}, snapshots: []string{material}, wantError: true},
		{name: "partial-restore", phase: SandboxPhaseFullyReady, missing: []string{"workspace", material}, snapshots: []string{"workspace"}, wantError: true},
		{name: "restore-workspace", phase: SandboxPhaseFullyReady, missing: []string{"workspace"}, snapshots: []string{"workspace"}},
		{name: "restore-all", phase: SandboxPhaseFullyReady, missing: []string{"workspace", material}, snapshots: []string{"workspace", material}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plan := CreateSandboxPlan{
				Sandbox: Sandbox{ID: 123, Namespace: "sandbox-test", Phase: tt.phase, SnapshotDomains: tt.snapshots},
				Runtimes: []RuntimePlan{{InstanceCode: "chain", Runtime: Runtime{AdapterSpec: AdapterSpec{
					VolumeDomains: []VolumeDomainSpec{{Name: VolumeDomainWorkspace}, {Name: VolumeDomainPublicAssets}},
					InitAssets:    []InitAssetSpec{{TargetDomain: VolumeDomainPublicAssets}},
				}}}},
			}
			if tt.snapshots != nil {
				plan.Sandbox.SnapshotRef = "sandbox-test/snapshot-123-0123456789abcdef0123456789abcdef"
			}
			var retained []runtime.Object
			for _, name := range []string{"workspace", material} {
				if !slices.Contains(tt.missing, name) {
					retained = append(retained, &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: plan.Sandbox.Namespace, UID: "retained-claim", Labels: map[string]string{
							"app": "chaimir", "module": "sandbox", "chaimir.io/sandbox-id": "123", "chaimir.io/volume": name,
						}},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
						},
					})
				}
			}
			client := fake.NewSimpleClientset(retained...)
			client.PrependReactor("create", "persistentvolumeclaims", func(action clienttesting.Action) (bool, runtime.Object, error) {
				claim := action.(clienttesting.CreateAction).GetObject().(*corev1.PersistentVolumeClaim).DeepCopy()
				claim.UID = "created-claim"
				if err := client.Tracker().Create(corev1.SchemeGroupVersion.WithResource("persistentvolumeclaims"), claim, claim.Namespace); err != nil {
					return true, nil, err
				}
				return true, claim, nil
			})
			o := &K8sOrchestrator{cfg: config.SandboxConfig{WorkspaceStorage: "1Gi"}}
			if err := o.applyWorkspacePVC(context.Background(), client, plan); (err != nil) != tt.wantError {
				t.Fatalf("missing volume recovery: err=%v wantError=%v", err, tt.wantError)
			}
			var created []string
			for _, action := range client.Actions() {
				if action.GetVerb() != "create" {
					continue
				}
				claim := action.(clienttesting.CreateAction).GetObject().(*corev1.PersistentVolumeClaim)
				created = append(created, claim.Name)
				if tt.snapshots != nil {
					source := snapshotSourceForDomain(plan.Sandbox, VolumeDomainSpec{Name: claim.Name})
					if !reflect.DeepEqual(claim.Spec.DataSource, source) {
						t.Errorf("missing volume %s did not use committed snapshot: %v", claim.Name, claim.Spec.DataSource)
					}
				} else if claim.Spec.DataSource != nil {
					t.Errorf("first start used a snapshot: %v", claim.Spec.DataSource)
				}
			}
			if tt.wantError {
				if len(created) != 0 {
					t.Fatalf("recovery created volumes before checking all retained data: %v", created)
				}
			} else if !slices.Equal(created, tt.missing) {
				t.Fatalf("created=%v want=%v", created, tt.missing)
			}
		})
	}
}

func TestPVCBeingDeletedCannotBeReused(t *testing.T) {
	desired := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "workspace", Namespace: "sandbox-test"}}
	existing := desired.DeepCopy()
	now := metav1.Now()
	existing.DeletionTimestamp = &now
	client := fake.NewSimpleClientset(existing)
	if err := createOrVerifyPVC(context.Background(), client, desired, true); err == nil {
		t.Fatal("terminating PVC was accepted as retained data")
	}
}

func TestSandboxResourceQuotaUsesCompletePodPlan(t *testing.T) {
	for _, scenario := range []string{"multiple-runtimes", "pod-count", "init-cpu", "init-memory", "sequential-init", "private-sidecar", "deferred-tool"} {
		t.Run(scenario, func(t *testing.T) {
			resources := workload.ResourceSpec{Requests: map[string]string{"cpu": "600m", "memory": "64Mi"}, Limits: map[string]string{"cpu": "600m", "memory": "64Mi"}}
			plan := CreateSandboxPlan{WorkspaceRuntimeInstance: "chain", Runtimes: []RuntimePlan{{InstanceCode: "chain", Runtime: Runtime{AdapterSpec: AdapterSpec{
				RuntimeContainer: workload.ComponentSpec{Name: "runtime", Resources: resources},
				Pods:             []workload.PodSpec{{Name: "node", Containers: []workload.ComponentSpec{{Name: "runtime"}}}},
			}}}}}
			cfg := config.SandboxConfig{MaxCPU: "1", MaxMemory: "1Gi", MaxPods: "10", DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "100m", DefaultMemory: "64Mi"}
			wantError := true
			switch scenario {
			case "multiple-runtimes", "pod-count":
				other := plan.Runtimes[0]
				other.InstanceCode = "other"
				plan.Runtimes = append(plan.Runtimes, other)
				if scenario == "pod-count" {
					cfg.MaxCPU, cfg.MaxPods = "2", "1"
				}
			case "init-cpu", "init-memory", "sequential-init":
				init := workload.ComponentSpec{Name: "seed", Resources: resources}
				init.Resources.Requests = map[string]string{"cpu": "600m", "memory": "64Mi"}
				init.Resources.Limits = map[string]string{"cpu": "600m", "memory": "64Mi"}
				if scenario == "init-cpu" {
					init.Resources.Limits["cpu"] = "2"
				} else if scenario == "init-memory" {
					init.Resources.Requests["memory"], init.Resources.Limits["memory"] = "2Gi", "2Gi"
				} else {
					wantError = false
				}
				plan.Runtimes[0].Runtime.AdapterSpec.Pods[0].InitContainers = []workload.ComponentSpec{init}
			case "private-sidecar":
				plan.PrivateSidecars = []workload.ComponentSpec{{Name: "judge", Resources: resources}}
			case "deferred-tool":
				plan.DeferTools = true
				plan.Tools = []Tool{{Code: "editor", Kind: SandboxToolKindWebEmbed, ResourceSpec: ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "editor", Resources: resources}}}}}
			}
			input := contracts.SandboxCreateRequest{}
			quota := TenantQuota{MaxConcurrentSandbox: 5, MaxCPU: 10, MaxMemoryMB: 10240}
			err := validateQuotaForCreate(input, quota, 0, cfg, plan)
			if wantError && !errors.Is(err, apperr.ErrSandboxResourceQuotaExceeded) || !wantError && err != nil {
				t.Fatalf("resource quota ignored actual Pod plan: scenario=%s err=%v", scenario, err)
			}
		})
	}
}

func TestSnapshotCapabilityRequiresMatchingStorageDriver(t *testing.T) {
	for _, driver := range []string{"disk.csi.example", "other.csi.example", ""} {
		t.Run(driver, func(t *testing.T) {
			o := testSnapshotOrchestrator(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var obj any
				switch {
				case strings.Contains(r.URL.Path, "/storageclasses/"):
					obj = storagev1.StorageClass{TypeMeta: metav1.TypeMeta{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"}, Provisioner: "disk.csi.example"}
				case strings.Contains(r.URL.Path, "/volumesnapshotclasses/"):
					obj = map[string]any{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshotClass", "driver": driver, "deletionPolicy": "Delete"}
				default:
					obj = map[string]any{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshotList", "items": []any{}}
				}
				if err := json.NewEncoder(w).Encode(obj); err != nil {
					t.Error(err)
				}
			})
			o.cfg.StorageClassName, o.cfg.VolumeSnapshotClassName = "storage", "snapshots"
			got, err := o.SnapshotSupported(context.Background())
			if err != nil || got != (driver == "disk.csi.example") {
				t.Fatalf("driver=%q: supported=%v err=%v", driver, got, err)
			}
		})
	}
}

func TestSnapshotBindingRequiresCurrentObjectUID(t *testing.T) {
	for _, uid := range []string{"current-snapshot", "previous-snapshot", ""} {
		t.Run(uid, func(t *testing.T) {
			snapshot := &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{"name": "snapshot", "namespace": "sandbox", "uid": "current-snapshot"},
				"status":   map[string]any{"readyToUse": true, "boundVolumeSnapshotContentName": "content"},
			}}
			content := &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{"name": "content"},
				"spec":     map[string]any{"volumeSnapshotRef": map[string]any{"name": "snapshot", "namespace": "sandbox", "uid": uid}},
			}}
			if got := volumeSnapshotReadyAndBound(snapshot, content, "sandbox", "snapshot"); got != (uid == "current-snapshot") {
				t.Fatalf("snapshot content UID=%q: bound=%v", uid, got)
			}
		})
	}
}

func TestRuntimePodsShareOnlyTheirOwnedDomains(t *testing.T) {
	for _, topology := range []string{"same-pod", "separate-pods", "separate-runtimes"} {
		t.Run(topology, func(t *testing.T) {
			separate := topology != "same-pod"
			mountWorkspace := false
			writer := workload.ComponentSpec{Name: "writer", MountWorkspace: &mountWorkspace, MountDomains: []string{"public-assets"}, WritableMountDomains: []string{"public-assets"}}
			reader := workload.ComponentSpec{Name: "reader", MountWorkspace: &mountWorkspace, MountDomains: []string{"public-assets"}}
			pods := []workload.PodSpec{{Name: "node", InitContainers: []workload.ComponentSpec{writer}, Containers: []workload.ComponentSpec{reader}}}
			if separate {
				pods = []workload.PodSpec{{Name: "node", Containers: []workload.ComponentSpec{writer}}, {Name: "client", InitContainers: []workload.ComponentSpec{reader}}}
			}
			plan := CreateSandboxPlan{WorkspaceRuntimeInstance: "chain", Runtimes: []RuntimePlan{{InstanceCode: "chain", Runtime: Runtime{AdapterSpec: AdapterSpec{
				RuntimeContainer: writer, Pods: pods, VolumeDomains: []VolumeDomainSpec{{Name: "public-assets", Persistence: VolumePersistenceEphemeral, SnapshotScope: VolumeSnapshotNever}},
			}}}}}
			if topology == "separate-runtimes" {
				plan.Runtimes[0].Runtime.AdapterSpec.Pods = []workload.PodSpec{pods[0]}
				plan.Runtimes = append(plan.Runtimes, RuntimePlan{InstanceCode: "consumer", Runtime: Runtime{AdapterSpec: AdapterSpec{
					RuntimeContainer: reader, VolumeDomains: plan.Runtimes[0].Runtime.AdapterSpec.VolumeDomains,
				}}})
			}
			volumes := volumeDomainsForPlan(plan)
			if got := volumes[0].PersistentVolumeClaim != nil; got != (topology == "separate-pods") {
				t.Fatalf("topology=%s shared PVC=%v; sharing must stay within the same owner", topology, got)
			}
			if topology == "separate-runtimes" && (len(volumes) != 2 || volumes[0].Name == volumes[1].Name) {
				t.Fatalf("different runtime domains were merged: %+v", volumes)
			}
		})
	}
}

func TestToolVolumesRejectUndeclaredDomains(t *testing.T) {
	mountWorkspace := false
	component := workload.ComponentSpec{Name: "tool", MountWorkspace: &mountWorkspace, MountDomains: []string{"missing"}}
	plan := CreateSandboxPlan{Tools: []Tool{{ResourceSpec: ToolResourceSpec{Components: []workload.ComponentSpec{component}}}}}
	if _, err := podVolumesForTool(plan, plan.Tools[0], component); err == nil {
		t.Fatal("tool cannot reference a volume domain absent from the runtime declarations")
	}
}

func volumePodPlan() CreateSandboxPlan {
	noWorkspace := false
	mountWorkspace := true
	node := workload.ComponentSpec{Name: "runtime", MountWorkspace: &noWorkspace, MountDomains: []string{"runtime-state"}, EphemeralMounts: []workload.EphemeralMountSpec{{Name: "cache", MountPath: "/cache"}}}
	workspace := workload.ComponentSpec{Name: "workspace", MountWorkspace: &mountWorkspace}
	worker := workload.ComponentSpec{Name: "worker", MountWorkspace: &noWorkspace}
	seed := workload.ComponentSpec{Name: "seed", MountWorkspace: &noWorkspace, EphemeralMounts: []workload.EphemeralMountSpec{{Name: "tmp", MountPath: "/tmp"}}}
	return CreateSandboxPlan{
		Sandbox: Sandbox{ID: 123, TenantID: 456, Namespace: "sandbox-123"}, WorkspaceRuntimeInstance: "chain",
		Runtimes: []RuntimePlan{{InstanceCode: "chain", Runtime: Runtime{AdapterSpec: AdapterSpec{
			RuntimeContainer: node, InfraSidecars: []workload.ComponentSpec{workspace, worker},
			Pods: []workload.PodSpec{
				{Name: "node", Containers: []workload.ComponentSpec{{Name: "runtime"}}},
				{Name: "workspace", Containers: []workload.ComponentSpec{{Name: "workspace"}}},
				{Name: "worker", InitContainers: []workload.ComponentSpec{seed}, Containers: []workload.ComponentSpec{{Name: "worker"}}},
			},
			VolumeDomains: []VolumeDomainSpec{
				{Name: VolumeDomainWorkspace, MountPath: "/workspace", Persistence: VolumePersistenceObjectStorageCode},
				{Name: "runtime-state", MountPath: "/ledger", Persistence: VolumePersistenceEphemeral},
				{Name: VolumeDomainJudgePrivate, MountPath: "/private", Persistence: VolumePersistenceEphemeral},
			},
			StartupOrder: []string{"editor", "chain"},
		}}}},
		Tools: []Tool{{Code: "editor", Category: "tool", Kind: SandboxToolKindWebEmbed, ResourceSpec: ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "editor", MountWorkspace: &mountWorkspace}}}}},
	}
}

func TestRuntimePodsOnlyDeclareVolumesTheyMount(t *testing.T) {
	o := &K8sOrchestrator{cfg: config.SandboxConfig{DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
	pods, err := o.podsForPlan(volumePodPlan())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"chain-node":                             {"runtime-state", "rt-runtime-cache"},
		"chain-workspace":                        {VolumeDomainWorkspace},
		"chain-worker":                           {"rt-seed-tmp"},
		toolComponentPodName("editor", "editor"): {VolumeDomainWorkspace},
	}
	for _, pod := range pods {
		var got []string
		for _, volume := range pod.Spec.Volumes {
			got = append(got, volume.Name)
		}
		slices.Sort(got)
		expected := slices.Clone(want[pod.Name])
		slices.Sort(expected)
		if !slices.Equal(got, expected) {
			t.Errorf("Pod %s references unused or missing volumes: got=%v want=%v", pod.Name, got, expected)
		}
	}
}

func TestSandboxPVCConsumersCanStartInDeclaredOrder(t *testing.T) {
	plan := volumePodPlan()
	o := &K8sOrchestrator{cfg: config.SandboxConfig{DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
	pods, err := o.podsForPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	var created []string
	if _, err := createPodsInStartupOrder(plan, pods, func(pod *corev1.Pod) (*corev1.Pod, error) {
		created = append(created, pod.Name)
		return pod, nil
	}, func([]*corev1.Pod) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if created[0] != toolComponentPodName("editor", "editor") {
		t.Fatalf("tool must actually start before the workspace: %v", created)
	}
	for _, pod := range pods {
		consumer := slices.ContainsFunc(pod.Spec.Volumes, func(v corev1.Volume) bool { return v.PersistentVolumeClaim != nil })
		if !consumer {
			if pod.Spec.Affinity != nil {
				t.Errorf("Pod %s has no PVC but waits for volume consumers", pod.Name)
			}
			continue
		}
		if pod.Spec.Affinity == nil || pod.Spec.Affinity.PodAffinity == nil {
			t.Errorf("PVC consumer %s has no node affinity", pod.Name)
			continue
		}
		for _, term := range pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
			selector, err := metav1.LabelSelectorAsSelector(term.LabelSelector)
			if err != nil {
				t.Fatal(err)
			}
			// The first PVC consumer must be schedulable without waiting for a later startup dependency.
			if !selector.Matches(labels.Set(pod.Labels)) || term.TopologyKey != "kubernetes.io/hostname" || len(term.Namespaces) != 0 || term.NamespaceSelector != nil {
				t.Errorf("Pod %s cannot establish its own sandbox-local volume affinity: %+v", pod.Name, term)
			}
			for _, peer := range pods {
				peerConsumer := slices.ContainsFunc(peer.Spec.Volumes, func(v corev1.Volume) bool { return v.PersistentVolumeClaim != nil })
				if selector.Matches(labels.Set(peer.Labels)) != peerConsumer {
					t.Errorf("Pod %s affinity selects %s inconsistently with actual PVC use", pod.Name, peer.Name)
				}
			}
			otherSandbox := labels.Set(maps.Clone(pod.Labels))
			otherSandbox["chaimir.io/sandbox-id"] = "999"
			if selector.Matches(otherSandbox) {
				t.Errorf("Pod %s affinity crosses sandbox boundaries", pod.Name)
			}
		}
	}
}

func TestInitializationWaitsOnlyForCreatedPods(t *testing.T) {
	plan := volumePodPlan()
	plan.DeferTools = true
	plan.Tools[0].Category = "infra"
	freezeStartupPlan(t, &plan, nil)
	o := &K8sOrchestrator{cfg: config.SandboxConfig{DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
	pods, err := o.podsForPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, waiting := range runtimePods(plan, pods) {
		if !slices.Contains(pods, waiting) {
			t.Errorf("初始化等待了未创建的 Pod %s", waiting.Name)
		}
	}
}

func TestInitializationStartsTransitiveRuntimeDependencies(t *testing.T) {
	plan := volumePodPlan()
	plan.DeferTools = true
	plan.Runtimes[0].Runtime.AdapterSpec.StartupOrder = []string{"database", "gateway", "chain", "editor"}
	plan.Tools = []Tool{
		{Code: "database", Category: "infra", ResourceSpec: ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "database"}}}},
		{Code: "gateway", Category: "tool", Kind: SandboxToolKindCommand, ResourceSpec: ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "gateway"}}}},
		{Code: "editor", Category: "infra", ResourceSpec: ToolResourceSpec{Components: []workload.ComponentSpec{{Name: "editor"}}}},
	}
	freezeStartupPlan(t, &plan, []contracts.CompositionLink{
		{SourceComponent: "gateway", TargetComponent: "database", RequiredAtStart: true},
		{SourceComponent: "chain", TargetComponent: "gateway", RequiredAtStart: true},
		{SourceComponent: "chain", TargetComponent: "editor"},
	})
	o := &K8sOrchestrator{cfg: config.SandboxConfig{DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
	pods, err := o.podsForPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	var created []string
	_, err = createPodsInStartupOrder(plan, pods, func(pod *corev1.Pod) (*corev1.Pod, error) {
		created = append(created, pod.Name)
		if pod.Name == "chain-node" && !slices.Contains(created, toolComponentPodName("gateway", "gateway")) {
			return nil, errors.New("链先于必需的网关启动")
		}
		return pod, nil
	}, func(waiting []*corev1.Pod) error {
		if slices.ContainsFunc(waiting, func(pod *corev1.Pod) bool { return pod.Name == toolComponentPodName("gateway", "gateway") }) && !slices.Contains(created, toolComponentPodName("database", "database")) {
			return errors.New("网关先于必需的数据库启动")
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	for _, code := range []string{"database", "gateway"} {
		if !slices.Contains(created, toolComponentPodName(code, code)) {
			t.Errorf("初始化跳过了链启动必需的组件 %s", code)
		}
	}
	if slices.Contains(created, toolComponentPodName("editor", "editor")) {
		t.Error("非前置工具应在个性化初始化完成后启动")
	}
}

func TestInitializationPreflightsDeferredTools(t *testing.T) {
	plan := volumePodPlan()
	plan.DeferTools = true
	plan.Tools[0].ResourceSpec.VolumeSource = "runtime:chain"
	plan.Tools[0].ResourceSpec.Components[0].MountDomains = []string{"missing"}
	freezeStartupPlan(t, &plan, nil)
	o := &K8sOrchestrator{cfg: config.SandboxConfig{DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
	if _, err := o.podsForPlan(plan); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("延后启动不能跳过完整资源计划预检: %v", err)
	}
}

func freezeStartupPlan(t *testing.T, plan *CreateSandboxPlan, links []contracts.CompositionLink) {
	t.Helper()
	adapter, err := json.Marshal(plan.Runtimes[0].Runtime.AdapterSpec)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := contracts.SandboxCompositionSnapshot{
		Spec: contracts.SandboxCompositionSpec{
			ID: "startup", WorkspaceRuntimeInstance: plan.Runtimes[0].InstanceCode, AccessProfile: contracts.SandboxAccessExperiment,
			Runtimes: []contracts.CompositionRuntimeRef{{InstanceCode: plan.Runtimes[0].InstanceCode, Code: "evm", ImageVersion: "1"}}, Links: links,
		},
		Runtimes: []contracts.CompiledRuntimeSnapshot{{InstanceCode: plan.Runtimes[0].InstanceCode, RuntimeID: 1, ImageID: 2, Code: "evm", ImageVersion: "1", AdapterSpec: adapter}},
	}
	snapshot.Digest, err = contracts.CanonicalSnapshotDigest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	plan.Sandbox.CompositionDigest = snapshot.Digest
	plan.Sandbox.CompositionSnapshot, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeTopologyExpansionDoesNotChangeTheDeclaration(t *testing.T) {
	plan := volumePodPlan().Runtimes[0]
	before, err := json.Marshal(plan.Runtime.AdapterSpec)
	if err != nil {
		t.Fatal(err)
	}
	first := runtimePodSpecs(plan)
	after, err := json.Marshal(plan.Runtime.AdapterSpec)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("expanding Pod references mutated the authoritative adapter declaration")
	}
	if !reflect.DeepEqual(first, runtimePodSpecs(plan)) {
		t.Error("repeated topology expansion changed the result")
	}
}

func TestRuntimeMaterializationKeepsInstancesIndependent(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			adapter := AdapterSpec{
				RuntimeContainer: workload.ComponentSpec{Name: "runtime"},
				NetworkRules:     []workload.NetworkRuleSpec{{From: "sandbox", To: "sandbox"}},
				StartupOrder:     []string{"sandbox"},
				WorkspaceOps:     WorkspaceOps{ExecTarget: "sandbox/runtime"},
			}
			if explicit {
				adapter.Pods = []workload.PodSpec{{Name: "sandbox", Containers: []workload.ComponentSpec{{Name: "runtime"}}}}
			}
			before, err := json.Marshal(adapter)
			if err != nil {
				t.Fatal(err)
			}
			left := MaterializeRuntimeTopology("left", adapter)
			right := MaterializeRuntimeTopology("right", adapter)
			after, err := json.Marshal(adapter)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Error("materialization changed the source declaration")
			}
			for instance, result := range map[string]AdapterSpec{"left": left, "right": right} {
				name := instance + "-sandbox"
				if result.Pods[0].Name != name || result.NetworkRules[0].From != name || result.NetworkRules[0].To != name ||
					result.StartupOrder[0] != "sandbox" || result.WorkspaceOps.ExecTarget != name+"/runtime" {
					t.Errorf("instance %s inherited another instance's topology: %+v", instance, result)
				}
			}
		})
	}
}

func TestSelftestCleanupRespectsItsPersistedDeadline(t *testing.T) {
	now := time.Now().UTC()
	for _, tt := range []struct {
		name      string
		deadline  string
		change    func(*corev1.Namespace)
		deleted   bool
		wantError bool
	}{
		{name: "expired", deadline: now.Add(-time.Second).Format(time.RFC3339Nano), deleted: true},
		{name: "at-deadline", deadline: now.Format(time.RFC3339Nano), deleted: true},
		{name: "still-executing", deadline: now.Add(time.Hour).Format(time.RFC3339Nano)},
		{name: "still-cleaning", deadline: now.Add(time.Minute).Format(time.RFC3339Nano)},
		{name: "missing-deadline", wantError: true},
		{name: "invalid-deadline", deadline: "invalid", wantError: true},
		{name: "before-creation", deadline: now.Add(-2 * time.Hour).Format(time.RFC3339Nano), wantError: true},
		{name: "tenant-workload", change: func(ns *corev1.Namespace) { ns.Labels["chaimir.io/tenant-id"] = "1" }},
		{name: "wrong-id", change: func(ns *corev1.Namespace) { ns.Labels["chaimir.io/sandbox-id"] = "456" }},
		{name: "ordinary-namespace", change: func(ns *corev1.Namespace) { ns.Name = "sandbox-123" }},
		{name: "already-deleting", change: func(ns *corev1.Namespace) { ts := metav1.NewTime(now); ns.DeletionTimestamp = &ts }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ns := namespaceObject("sbx-selftest-123", Sandbox{ID: 123})
			ns.UID, ns.ResourceVersion = "selftest-123", "1"
			ns.CreationTimestamp = metav1.NewTime(now.Add(-time.Hour))
			ns.Annotations = map[string]string{"chaimir.io/selftest-cleanup-after": tt.deadline}
			if tt.change != nil {
				tt.change(ns)
			}
			client := fake.NewSimpleClientset(ns)
			err := cleanupExpiredRuntimeSelftestNamespaces(context.Background(), client, 1, now)
			if (err != nil) != tt.wantError {
				t.Errorf("cleanup error=%v wantError=%v", err, tt.wantError)
			}
			deleted := slices.ContainsFunc(client.Actions(), func(action clienttesting.Action) bool { return action.GetVerb() == "delete" })
			if deleted != tt.deleted {
				t.Errorf("deleted=%v want=%v", deleted, tt.deleted)
			}
		})
	}
}

func TestSelftestNamespaceFreezesExecutionAndCleanupWindows(t *testing.T) {
	for _, tt := range []struct {
		name     string
		tenantID int64
		bounded  bool
		cleanup  int
		created  bool
	}{
		{name: "long-platform-selftest", bounded: true, cleanup: 3, created: true},
		{name: "missing-execution-budget", cleanup: 3},
		{name: "missing-cleanup-budget", bounded: true},
		{name: "tenant-sandbox", tenantID: 456, created: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var namespace *corev1.Namespace
			o := testSnapshotOrchestrator(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/namespaces" {
					t.Errorf("unexpected API request: %s %s", r.Method, r.URL.Path)
				} else {
					raw, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					} else {
						namespace = &corev1.Namespace{}
						if _, _, err := kubescheme.Codecs.UniversalDeserializer().Decode(raw, nil, namespace); err != nil {
							t.Error(err)
						}
					}
				}
				// 在创建请求边界结束，不启动 RBAC、Pod 或其他集群资源。
				w.WriteHeader(http.StatusServiceUnavailable)
			})
			o.cfg.DefaultReqCPU, o.cfg.DefaultReqMemory, o.cfg.DefaultCPU, o.cfg.DefaultMemory = "100m", "64Mi", "1", "1Gi"
			o.cfg.SelftestRecycleTimeoutSeconds = tt.cleanup
			plan := volumePodPlan()
			plan.Sandbox.TenantID = tt.tenantID
			if tt.tenantID == 0 {
				plan.Sandbox.Namespace = "sbx-selftest-123"
			}
			ctx := context.Background()
			deadline := time.Now().Add(time.Hour)
			if tt.bounded {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, deadline)
				defer cancel()
			}
			if err := o.CreateSandboxResources(ctx, plan, nil); err == nil {
				t.Fatal("creation should stop at the injected API error")
			}
			if (namespace != nil) != tt.created {
				t.Fatalf("Namespace creation=%v want=%v", namespace != nil, tt.created)
			}
			if namespace == nil {
				return
			}
			value := namespace.Annotations[selftestCleanupAfterAnnotation]
			if tt.tenantID != 0 {
				if value != "" {
					t.Error("tenant resources received a selftest expiry")
				}
				return
			}
			actual, err := time.Parse(time.RFC3339Nano, value)
			if err != nil || !actual.Equal(deadline.Add(5*time.Second)) {
				t.Fatalf("persisted expiry=%s error=%v", value, err)
			}
		})
	}
}

func TestRuntimeResetDoesNotRecreateOtherChains(t *testing.T) {
	pods := map[string]*corev1.Pod{}
	var deleted, created []string
	o := testSnapshotOrchestrator(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/namespaces/sandbox-123") {
			ns := namespaceObject("sandbox-123", Sandbox{ID: 123, TenantID: 456})
			ns.TypeMeta = metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"}
			json.NewEncoder(w).Encode(ns)
			return
		}
		name := filepath.Base(r.URL.Path)
		var obj any
		switch r.Method {
		case http.MethodDelete:
			deleted = append(deleted, name)
			delete(pods, name)
			obj = metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Status: "Success"}
		case http.MethodPost:
			var pod corev1.Pod
			raw, err := io.ReadAll(r.Body)
			if err == nil {
				_, _, err = kubescheme.Codecs.UniversalDeserializer().Decode(raw, nil, &pod)
			}
			if err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			created = append(created, pod.Name)
			pod.UID = "created-pod"
			pod.Status = corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}
			pods[pod.Name] = &pod
			obj = &pod
		default:
			if name == "pods" {
				list := &corev1.PodList{}
				for _, pod := range pods {
					list.Items = append(list.Items, *pod)
				}
				obj = list
				break
			}
			var ok bool
			obj, ok = pods[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				obj = metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Reason: metav1.StatusReasonNotFound, Code: http.StatusNotFound}
			}
		}
		if err := json.NewEncoder(w).Encode(obj); err != nil {
			t.Error(err)
		}
	})
	o.cfg.DefaultReqCPU, o.cfg.DefaultReqMemory, o.cfg.DefaultCPU, o.cfg.DefaultMemory = "100m", "64Mi", "1", "1Gi"
	plan := volumePodPlan()
	plan.Tools = nil
	plan.Runtimes = append(plan.Runtimes, RuntimePlan{InstanceCode: "other", Runtime: Runtime{AdapterSpec: AdapterSpec{RuntimeContainer: workload.ComponentSpec{Name: "runtime"}}}})
	initial, err := o.podsForPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, pod := range initial {
		pod.UID = "original-pod"
		pods[pod.Name] = pod
	}
	other := pods["other-sandbox"]
	workspace := pods["chain-workspace"]
	if err := o.ResetSandboxRuntime(context.Background(), plan, "chain", nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(deleted, []string{"chain-node"}) || !slices.Equal(created, []string{"chain-node"}) || pods["other-sandbox"] != other || pods["chain-workspace"] != workspace {
		t.Fatalf("one chain reset changed unrelated resources: deleted=%v created=%v", deleted, created)
	}
	for _, target := range []string{"", "missing"} {
		if err := o.ResetSandboxRuntime(context.Background(), plan, target, nil); err == nil {
			t.Errorf("invalid reset target %q accepted", target)
		}
	}
	if len(deleted) != 1 || len(created) != 1 {
		t.Fatalf("invalid reset targets changed resources: deleted=%v created=%v", deleted, created)
	}
}

func TestRuntimeResetReplacesOnlyTargetLedgerPVC(t *testing.T) {
	plan := volumePodPlan()
	plan.Sandbox.SnapshotEnabled = true
	plan.Runtimes[0].Runtime.AdapterSpec.VolumeDomains[1].SnapshotScope = VolumeSnapshotEnabled
	plan.Runtimes[0].Runtime.AdapterSpec.InfraSidecars[1].MountDomains = []string{VolumeDomainRuntimeState}
	other := plan.Runtimes[0]
	other.InstanceCode = "other"
	plan.Runtimes = append(plan.Runtimes, other)
	plan.Tools = []Tool{{Code: "reader", Category: "infra", ResourceSpec: ToolResourceSpec{
		VolumeSource: "runtime:chain", Components: []workload.ComponentSpec{{Name: "reader", MountDomains: []string{VolumeDomainRuntimeState}}},
	}}}
	pods := map[string]*corev1.Pod{}
	claims := map[string]*corev1.PersistentVolumeClaim{}
	var deletedClaims, createdClaims []string
	o := testSnapshotOrchestrator(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/namespaces/"+plan.Sandbox.Namespace) {
			ns := namespaceObject(plan.Sandbox.Namespace, plan.Sandbox)
			ns.TypeMeta = metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"}
			json.NewEncoder(w).Encode(ns)
			return
		}
		name := filepath.Base(r.URL.Path)
		isClaim := strings.Contains(r.URL.Path, "/persistentvolumeclaims")
		var obj any
		switch r.Method {
		case http.MethodDelete:
			if isClaim {
				for _, pod := range pods {
					for _, volume := range pod.Spec.Volumes {
						if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == name {
							t.Errorf("ledger deleted while consumer %s is still running", pod.Name)
						}
					}
				}
				deletedClaims = append(deletedClaims, name)
				delete(claims, name)
			} else {
				delete(pods, name)
			}
			obj = &metav1.Status{Status: "Success"}
		case http.MethodPost:
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if isClaim {
				var claim corev1.PersistentVolumeClaim
				if _, _, err := kubescheme.Codecs.UniversalDeserializer().Decode(raw, nil, &claim); err != nil {
					t.Error(err)
					return
				}
				if claim.Spec.DataSource != nil || claim.Spec.DataSourceRef != nil {
					t.Error("reset restored the old ledger snapshot")
				}
				claim.UID = "created-claim"
				createdClaims = append(createdClaims, claim.Name)
				claims[claim.Name], obj = &claim, &claim
			} else {
				var pod corev1.Pod
				if _, _, err := kubescheme.Codecs.UniversalDeserializer().Decode(raw, nil, &pod); err != nil {
					t.Error(err)
					return
				}
				pod.UID = "created-pod"
				pod.Status = corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}
				pods[pod.Name], obj = &pod, &pod
			}
		default:
			if name == "pods" {
				list := &corev1.PodList{}
				for _, pod := range pods {
					list.Items = append(list.Items, *pod)
				}
				obj = list
			} else if isClaim && claims[name] != nil {
				obj = claims[name]
			} else if !isClaim && pods[name] != nil {
				obj = pods[name]
			} else {
				w.WriteHeader(http.StatusNotFound)
				obj = &metav1.Status{Reason: metav1.StatusReasonNotFound, Code: http.StatusNotFound}
			}
		}
		if err := json.NewEncoder(w).Encode(obj); err != nil {
			t.Error(err)
		}
	})
	o.cfg.DefaultReqCPU, o.cfg.DefaultReqMemory, o.cfg.DefaultCPU, o.cfg.DefaultMemory, o.cfg.WorkspaceStorage = "100m", "64Mi", "1", "1Gi", "1Gi"
	o.cfg.ReadyTimeoutSeconds = 10
	initial, err := o.podsForPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, pod := range initial {
		pod.UID = "original-pod"
		pods[pod.Name] = pod
	}
	for _, domain := range persistentVolumeDomains(plan) {
		name := domain.claimName()
		claims[name] = &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: plan.Sandbox.Namespace, UID: "original-claim", Labels: map[string]string{
			"app": "chaimir", "module": "sandbox", "chaimir.io/sandbox-id": fmt.Sprint(plan.Sandbox.ID), "chaimir.io/volume": name,
		}}}
	}
	workspace, otherLedger := claims["workspace"], claims["runtime-other-runtime-state"]
	otherNode, workspacePod := pods["other-node"], pods["chain-workspace"]
	if err := o.ResetSandboxRuntime(context.Background(), plan, "chain", nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(deletedClaims, []string{"runtime-chain-runtime-state"}) || !slices.Equal(createdClaims, deletedClaims) {
		t.Fatalf("reset left old ledger or replaced unrelated claims: deleted=%v created=%v", deletedClaims, createdClaims)
	}
	if claims["workspace"] != workspace || claims["runtime-other-runtime-state"] != otherLedger || pods["other-node"] != otherNode || pods["chain-workspace"] != workspacePod {
		t.Fatal("reset changed workspace or another chain")
	}
	for _, name := range []string{"chain-node", "chain-worker", toolComponentPodName("reader", "reader")} {
		if pods[name] == nil {
			t.Errorf("ledger consumer %s was not restarted", name)
		}
	}
}

func TestRuntimeResetRejectsUnsafeResourcesBeforeDeletion(t *testing.T) {
	for _, name := range []string{"unknown-consumer", "foreign-ledger", "invalid-storage"} {
		t.Run(name, func(t *testing.T) {
			plan := volumePodPlan()
			plan.Tools = nil
			plan.Sandbox.SnapshotEnabled = true
			plan.Runtimes[0].Runtime.AdapterSpec.VolumeDomains[1].SnapshotScope = VolumeSnapshotEnabled
			mutations := 0
			o := testSnapshotOrchestrator(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/namespaces/"+plan.Sandbox.Namespace) {
					ns := namespaceObject(plan.Sandbox.Namespace, plan.Sandbox)
					ns.TypeMeta = metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"}
					json.NewEncoder(w).Encode(ns)
					return
				}
				if r.Method != http.MethodGet {
					mutations++
					w.WriteHeader(http.StatusForbidden)
					if _, err := w.Write([]byte(`{"kind":"Status","apiVersion":"v1","reason":"Forbidden","code":403}`)); err != nil {
						t.Error(err)
					}
					return
				}
				var obj any
				if filepath.Base(r.URL.Path) == "pods" {
					list := &corev1.PodList{}
					if name == "unknown-consumer" {
						list.Items = append(list.Items, corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "rogue"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "runtime-chain-runtime-state"}}}}}})
					}
					obj = list
				} else {
					owner := "123"
					if name == "foreign-ledger" {
						owner = "999"
					}
					obj = &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "runtime-chain-runtime-state", Labels: map[string]string{"app": "chaimir", "module": "sandbox", "chaimir.io/sandbox-id": owner, "chaimir.io/volume": "runtime-chain-runtime-state"}}}
				}
				if err := json.NewEncoder(w).Encode(obj); err != nil {
					t.Error(err)
				}
			})
			o.cfg.DefaultReqCPU, o.cfg.DefaultReqMemory, o.cfg.DefaultCPU, o.cfg.DefaultMemory, o.cfg.WorkspaceStorage = "100m", "64Mi", "1", "1Gi", "1Gi"
			if name == "invalid-storage" {
				o.cfg.WorkspaceStorage = "invalid"
			}
			if err := o.ResetSandboxRuntime(context.Background(), plan, "chain", nil); err == nil || mutations != 0 {
				t.Fatalf("unsafe reset changed live resources: err=%v mutations=%d", err, mutations)
			}
		})
	}
}

func runtimeAssetPlan() (CreateSandboxPlan, []RuntimeInitArchive) {
	plan := volumePodPlan()
	plan.Tools = nil
	adapter := &plan.Runtimes[0].Runtime.AdapterSpec
	noWorkspace := false
	adapter.InfraSidecars = append(adapter.InfraSidecars, workload.ComponentSpec{
		Name: "init-assets", ImageURL: "archive@sha256:" + strings.Repeat("a", 64), MountWorkspace: &noWorkspace,
		MountDomains: []string{VolumeDomainRuntimeState, VolumeDomainJudgePrivate}, WritableMountDomains: []string{VolumeDomainRuntimeState},
		Env:             []workload.EnvVarSpec{{Name: "SOURCE_CONFIG", Value: "source-only"}},
		Labels:          map[string]string{"chaimir.io/student-access": "false"},
		EphemeralMounts: []workload.EphemeralMountSpec{{Name: "tmp", MountPath: "/tmp"}},
	})
	adapter.InitAssetOps = PrivateArchiveOps{ExecTarget: "node/init-assets", UnpackTar: []string{"/archive", "unpack", WorkspacePlaceholderDomain}}
	adapter.InitAssets = []InitAssetSpec{{Name: "genesis", SourceRef: "s3://attach/456/genesis.tar", ExpectedSHA256: strings.Repeat("a", 64), TargetDomain: VolumeDomainRuntimeState}}
	*adapter = MaterializeRuntimeTopology("chain", *adapter)
	return plan, []RuntimeInitArchive{{RuntimeInstance: "chain", TargetDomain: VolumeDomainRuntimeState, Tarball: []byte("prepared tarball")}}
}

func TestRuntimeAssetsUseIsolatedStagingPod(t *testing.T) {
	plan, archives := runtimeAssetPlan()
	o := &K8sOrchestrator{cfg: config.SandboxConfig{DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
	pods, err := o.podsForPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(pods)
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := initAssetPodsForPlan(plan, pods, archives)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("unexpected asset Pod count: %d", len(jobs))
	}
	pod := jobs[0].pod
	if pod.Name == "chain-node" || len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Name != "init-assets" || len(pod.Spec.InitContainers) != 0 {
		t.Fatalf("asset staging started runtime processes: %+v", pod.Spec)
	}
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName != "runtime-chain-runtime-state" {
			t.Errorf("asset staging can access another domain: %+v", volume)
		}
	}
	container := pod.Spec.Containers[0]
	if len(container.Env) != 0 || len(container.EnvFrom) != 0 || len(container.Ports) != 0 {
		t.Fatal("asset staging inherited runtime credentials or endpoints")
	}
	if !slices.ContainsFunc(container.VolumeMounts, func(m corev1.VolumeMount) bool {
		return m.Name == VolumeDomainRuntimeState && !m.ReadOnly && m.MountPath == "/ledger"
	}) {
		t.Fatal("asset staging lost its declared writable target")
	}
	after, err := json.Marshal(pods)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("staging mutated the authoritative runtime Pod plan")
	}
	for _, archive := range []RuntimeInitArchive{
		{RuntimeInstance: "missing", TargetDomain: VolumeDomainRuntimeState, Tarball: []byte("data")},
		{RuntimeInstance: "chain", TargetDomain: VolumeDomainWorkspace, Tarball: []byte("data")},
		{RuntimeInstance: "chain", TargetDomain: VolumeDomainRuntimeState},
	} {
		if _, err := initAssetPodsForPlan(plan, pods, []RuntimeInitArchive{archive}); err == nil {
			t.Errorf("invalid asset admitted: %+v", archive)
		}
	}
}

func TestRuntimeAssetWritesFinishBeforeStagingPodCleanup(t *testing.T) {
	writeFailure, cleanupFailure := errors.New("unpack failed"), errors.New("cleanup failed")
	for _, name := range []string{"success", "write-failure", "canceled", "cleanup-failure", "both-fail"} {
		t.Run(name, func(t *testing.T) {
			plan, archives := runtimeAssetPlan()
			o := &K8sOrchestrator{cfg: config.SandboxConfig{ReadyTimeoutSeconds: 2, ReadyPollIntervalSeconds: 1, DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
			pods, err := o.podsForPlan(plan)
			if err != nil {
				t.Fatal(err)
			}
			jobs, err := initAssetPodsForPlan(plan, pods, archives)
			if err != nil {
				t.Fatal(err)
			}
			cs := fake.NewSimpleClientset()
			var actions []string
			cs.PrependReactor("create", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
				pod := action.(clienttesting.CreateAction).GetObject().(*corev1.Pod)
				pod.UID = "original-assets"
				actions = append(actions, "create:"+pod.Name)
				pod.Status = corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}
				return false, nil, nil
			})
			cs.PrependReactor("delete", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
				actions = append(actions, "delete:"+action.(clienttesting.DeleteAction).GetName())
				if name == "cleanup-failure" || name == "both-fail" {
					return true, nil, cleanupFailure
				}
				return false, nil, nil
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err = o.initializeRuntimeAssets(ctx, cs, jobs, func(execCtx context.Context, namespace, target string, command []string, data []byte, tty bool) ([]byte, []byte, error) {
				actions = append(actions, "write:"+target)
				if namespace != plan.Sandbox.Namespace || target != "assets-chain/init-assets" || !slices.Equal(command, []string{"/archive", "unpack", "/ledger"}) || string(data) != string(archives[0].Tarball) || tty {
					t.Errorf("asset write escaped its declared execution boundary: ns=%s target=%s command=%v", namespace, target, command)
				}
				if name == "canceled" {
					cancel()
					return nil, nil, execCtx.Err()
				}
				if name == "write-failure" || name == "both-fail" {
					return nil, nil, writeFailure
				}
				return nil, nil, nil
			})
			if name == "success" && err != nil || name != "success" && err == nil {
				t.Fatalf("incorrect initialization result: %v", err)
			}
			if (name == "write-failure" || name == "both-fail") && !errors.Is(err, writeFailure) {
				t.Fatalf("lost write failure: %v", err)
			}
			if (name == "cleanup-failure" || name == "both-fail") && !errors.Is(err, cleanupFailure) {
				t.Fatalf("lost cleanup failure: %v", err)
			}
			if name == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}
			if !slices.Equal(actions, []string{"create:assets-chain", "write:assets-chain/init-assets", "delete:assets-chain"}) {
				t.Fatalf("wrong asset lifecycle: %v", actions)
			}
		})
	}
}

func TestRuntimeResetRejectsInvalidPlanBeforeDeletion(t *testing.T) {
	var requests []string
	o := testSnapshotOrchestrator(t, func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		status := metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Status: "Success"}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			status.Reason, status.Code = metav1.StatusReasonNotFound, http.StatusNotFound
		}
		if err := json.NewEncoder(w).Encode(status); err != nil {
			t.Error(err)
		}
	})
	o.cfg.DefaultReqCPU, o.cfg.DefaultReqMemory, o.cfg.DefaultCPU, o.cfg.DefaultMemory = "100m", "64Mi", "1", "1Gi"
	plan := volumePodPlan()
	plan.Tools[0].ResourceSpec.Components[0].MountDomains = []string{"missing"}
	if err := o.ResetSandboxRuntime(context.Background(), plan, "chain", nil); err == nil || len(requests) != 0 {
		t.Fatalf("invalid replacement plan touched running resources: err=%v requests=%v", err, requests)
	}
}

func TestPodReadinessReportsFailureBehindPendingPod(t *testing.T) {
	plan := volumePodPlan()
	client := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "chain-node", Namespace: plan.Sandbox.Namespace, UID: "pending"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "chain-workspace", Namespace: plan.Sandbox.Namespace, UID: "failed"}, Status: corev1.PodStatus{Phase: corev1.PodFailed, Reason: "fixture_failed"}},
	)
	o := &K8sOrchestrator{cfg: config.SandboxConfig{ReadyPollIntervalSeconds: 1}}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := o.waitPodsReady(ctx, client, []*corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "chain-node", Namespace: plan.Sandbox.Namespace, UID: "pending"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "chain-workspace", Namespace: plan.Sandbox.Namespace, UID: "failed"}},
	})
	if err == nil || errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "chain-workspace") || !strings.Contains(err.Error(), "fixture_failed") {
		t.Fatalf("pending Pod concealed another Pod's terminal failure: %v", err)
	}
}

func TestRuntimeLedgerVolumesStayWithinTheirInstance(t *testing.T) {
	plan := volumePodPlan()
	plan.Tools = nil
	plan.Sandbox.SnapshotEnabled = true
	plan.Runtimes[0].Runtime.AdapterSpec.VolumeDomains[1].SnapshotScope = VolumeSnapshotEnabled
	other := plan.Runtimes[0]
	other.InstanceCode = "other"
	plan.Runtimes = append(plan.Runtimes, other)
	o := &K8sOrchestrator{cfg: config.SandboxConfig{DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
	pods, err := o.podsForPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]string{}
	for _, pod := range pods {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				claims[pod.Name+"/"+volume.Name] = volume.PersistentVolumeClaim.ClaimName
			}
		}
	}
	if claims["chain-node/runtime-state"] != "runtime-chain-runtime-state" || claims["other-node/runtime-state"] != "runtime-other-runtime-state" {
		t.Fatalf("different chain ledgers share their backing volume: %v", claims)
	}
	if claims["chain-workspace/workspace"] != VolumeDomainWorkspace || claims["other-workspace/workspace"] != VolumeDomainWorkspace {
		t.Fatalf("workspace sharing changed: %v", claims)
	}
	if got := snapshotDomainsForPlan(plan); !slices.Contains(got, "runtime-chain-runtime-state") || !slices.Contains(got, "runtime-other-runtime-state") {
		t.Fatalf("snapshot domains lost ledger ownership: %v", got)
	}
}

func TestOwnedVolumeNamesRemainValidAndDistinct(t *testing.T) {
	owner := "runtime-" + strings.Repeat("chain-", 11) + "end"
	first := volumeClaimName(owner, VolumeDomainRuntimeState)
	second := volumeClaimName(owner+"-other", VolumeDomainRuntimeState)
	if problems := validation.IsDNS1123Label(first); len(problems) > 0 {
		t.Fatalf("derived volume name cannot be used as a Pod volume or ownership label: %s %v", first, problems)
	}
	if first == second || first != volumeClaimName(owner, VolumeDomainRuntimeState) {
		t.Fatal("volume names lost owner identity or stability")
	}
	left := RuntimePlan{InstanceCode: "alpha", Runtime: Runtime{AdapterSpec: AdapterSpec{VolumeDomains: []VolumeDomainSpec{{Name: "beta-runtime-state"}}}}}
	right := RuntimePlan{InstanceCode: "alpha-beta", Runtime: Runtime{AdapterSpec: AdapterSpec{VolumeDomains: []VolumeDomainSpec{{Name: VolumeDomainRuntimeState}}}}}
	if err := validateRuntimeIsolation([]RuntimePlan{left, right}); err == nil {
		t.Fatal("different runtime/domain pairs can silently resolve to the same physical volume")
	}
}

func TestRuntimeTopologyNamesFitKubernetesContracts(t *testing.T) {
	for _, instance := range []string{"chain", strings.Repeat("c", 63)} {
		for _, pod := range []string{"", strings.Repeat("n", 63)} {
			t.Run(instance+"/"+pod, func(t *testing.T) {
				name := pod
				if name == "" {
					name = "sandbox"
				}
				adapter := AdapterSpec{
					RuntimeContainer: workload.ComponentSpec{Name: "runtime"},
					WorkspaceOps:     WorkspaceOps{ExecTarget: name + "/runtime"},
					NetworkRules:     []workload.NetworkRuleSpec{{From: name, To: name}},
				}
				if pod != "" {
					adapter.Pods = []workload.PodSpec{{Name: pod, Containers: []workload.ComponentSpec{{Name: "runtime"}}}}
				}
				first := MaterializeRuntimeTopology(instance, adapter)
				podName := first.Pods[0].Name
				for _, candidate := range []string{podName, runtimeServiceName(podName)} {
					if problems := validation.IsDNS1123Label(candidate); len(problems) != 0 {
						t.Errorf("运行时衍生名称无效: %q %v", candidate, problems)
					}
				}
				if first.WorkspaceOps.ExecTarget != podName+"/runtime" || first.NetworkRules[0].From != podName || first.NetworkRules[0].To != podName {
					t.Error("执行目标或网络规则没有跟随实际 Pod 名称")
				}
				before, err := json.Marshal(first)
				if err != nil {
					t.Fatal(err)
				}
				second, err := json.Marshal(MaterializeRuntimeTopology(instance, first))
				if err != nil {
					t.Fatal(err)
				}
				if string(before) != string(second) || runtimePodSpecs(RuntimePlan{InstanceCode: instance, Runtime: Runtime{AdapterSpec: first}})[0].Name != podName {
					t.Error("重复物化改变了冻结拓扑或实际执行目标")
				}
			})
		}
	}
}

func TestToolPodLabelsAcceptRegisteredCodeLengths(t *testing.T) {
	for _, code := range []string{"editor", strings.Repeat("a", 63), strings.Repeat("a", 64)} {
		t.Run(code, func(t *testing.T) {
			if !codePattern.MatchString(code) {
				t.Fatal("fixture code is outside the registration contract")
			}
			plan := volumePodPlan()
			plan.Tools[0].Code = code
			o := &K8sOrchestrator{cfg: config.SandboxConfig{DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
			pods, err := o.podsForPlan(plan)
			if err != nil {
				t.Fatal(err)
			}
			for _, pod := range pods {
				for key, value := range pod.Labels {
					if problems := validation.IsValidLabelValue(value); len(problems) != 0 {
						t.Errorf("Pod %s label %s=%q is invalid: %v", pod.Name, key, value, problems)
					}
				}
			}
		})
	}
}

func TestRuntimePodCollisionsFailBeforeKubernetesAccess(t *testing.T) {
	for _, name := range []string{"runtime-runtime", "runtime-deferred-tool"} {
		t.Run(name, func(t *testing.T) {
			plan := volumePodPlan()
			left := plan.Runtimes[0]
			left.InstanceCode = "alpha"
			left.Runtime.AdapterSpec.Pods = []workload.PodSpec{{Name: "beta-node", Containers: []workload.ComponentSpec{{Name: "runtime"}}}}
			right := left
			right.InstanceCode = "alpha-beta"
			right.Runtime.AdapterSpec.Pods = []workload.PodSpec{{Name: "node", Containers: []workload.ComponentSpec{{Name: "runtime"}}}}
			plan.Runtimes = []RuntimePlan{left, right}
			if name == "runtime-deferred-tool" {
				left.InstanceCode = "tool-editor"
				left.Runtime.AdapterSpec.Pods = []workload.PodSpec{{Name: "editor", Containers: []workload.ComponentSpec{{Name: "runtime"}}}}
				plan.Runtimes = []RuntimePlan{left}
				plan.DeferTools = true
				freezeStartupPlan(t, &plan, nil)
			}
			o := &K8sOrchestrator{cfg: config.SandboxConfig{DefaultReqCPU: "100m", DefaultReqMemory: "64Mi", DefaultCPU: "1", DefaultMemory: "1Gi"}}
			if err := o.CreateSandboxResources(context.Background(), plan, nil); err == nil || !strings.Contains(err.Error(), "Pod") {
				t.Fatalf("Pod 重名应在访问 Kubernetes 前拒绝: %v", err)
			}
		})
	}
}

func TestInvalidServicesFailBeforeKubernetesAccess(t *testing.T) {
	for _, mode := range []string{"tool-runtime", "runtime-port-collision", "invalid-name", "unknown-target-port"} {
		t.Run(mode, func(t *testing.T) {
			plan := volumePodPlan()
			plan.Runtimes[0].Runtime.AdapterSpec.RuntimeContainer.Ports = []workload.PortSpec{{Name: "rpc", ContainerPort: 9000, ServicePort: 8000, Protocol: "TCP"}}
			tool := &plan.Tools[0]
			tool.ResourceSpec.Components[0].Ports = []workload.PortSpec{{Name: "api", ContainerPort: 9000, Protocol: "TCP"}}
			tool.ResourceSpec.Services = []workload.ServiceSpec{{Name: "runtime-chain-node", Component: "editor", Ports: []workload.ServicePortSpec{{Name: "api", Port: 8000, TargetPort: "api", Protocol: "TCP"}}}}
			switch mode {
			case "runtime-port-collision":
				tool.ResourceSpec.Services[0].Name = "editor-service"
				plan.Runtimes[0].Runtime.AdapterSpec.RuntimeContainer.Ports = append(plan.Runtimes[0].Runtime.AdapterSpec.RuntimeContainer.Ports,
					workload.PortSpec{Name: "rpc-alt", ContainerPort: 9001, ServicePort: 8000, Protocol: "TCP"})
			case "invalid-name":
				tool.ResourceSpec.Services[0].Name = "1invalid"
			case "unknown-target-port":
				tool.ResourceSpec.Services[0].Name = "editor-service"
				tool.ResourceSpec.Services[0].Ports[0].TargetPort = "missing"
			}
			requests := 0
			orchestrator := testSnapshotOrchestrator(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				http.Error(w, "unexpected resource operation", http.StatusForbidden)
			})
			orchestrator.cfg.DefaultReqCPU, orchestrator.cfg.DefaultReqMemory = "100m", "64Mi"
			orchestrator.cfg.DefaultCPU, orchestrator.cfg.DefaultMemory = "1", "1Gi"
			if err := orchestrator.CreateSandboxResources(context.Background(), plan, nil); err == nil || !strings.Contains(err.Error(), "Service") {
				t.Fatalf("invalid Service should fail during preflight: %v", err)
			}
			if requests != 0 {
				t.Fatalf("invalid Service reached Kubernetes: requests=%d", requests)
			}
		})
	}
}

func TestToolVolumeSourcesAreExplicit(t *testing.T) {
	plan := volumePodPlan()
	plan.Sandbox.SnapshotEnabled = true
	component := workload.ComponentSpec{Name: "tool", MountDomains: []string{VolumeDomainRuntimeState}}
	for _, tt := range []struct {
		source string
		claim  string
	}{
		{source: "runtime:chain", claim: "runtime-chain-runtime-state"},
		{source: "self", claim: "tool-checker-runtime-state"},
		{source: "runtime:missing"},
		{source: "binding:chain"},
		{},
	} {
		tool := Tool{Code: "checker", ResourceSpec: ToolResourceSpec{VolumeSource: tt.source, Components: []workload.ComponentSpec{component}}}
		plan.Tools = []Tool{tool}
		volumes, err := podVolumesForTool(plan, tool, component)
		if tt.claim == "" {
			if err == nil {
				t.Errorf("unresolved volume source %q accepted", tt.source)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		index := slices.IndexFunc(volumes, func(v corev1.Volume) bool { return v.Name == VolumeDomainRuntimeState })
		if index < 0 || volumes[index].PersistentVolumeClaim == nil || volumes[index].PersistentVolumeClaim.ClaimName != tt.claim {
			t.Errorf("wrong owner for source %q: %+v", tt.source, volumes)
		}
	}
}

func TestToolVolumeSourceFollowsItsNamedBinding(t *testing.T) {
	for _, target := range []string{"selected", "unavailable", ""} {
		tool := Tool{Code: "observer", ResourceSpec: ToolResourceSpec{
			VolumeSource: "binding:ledger", Components: []workload.ComponentSpec{{Name: "web", MountDomains: []string{VolumeDomainRuntimeState}}},
			Bindings: []ComponentBinding{{Name: "ledger", ConfigBinding: "env:LEDGER_RPC"}},
		}}
		spec := contracts.SandboxCompositionSpec{WorkspaceRuntimeInstance: "workspace", Links: []contracts.CompositionLink{
			{SourceComponent: "observer", ConfigBinding: "env:OTHER_RPC", TargetComponent: "workspace"},
		}}
		if target != "" {
			spec.Links = append(spec.Links, contracts.CompositionLink{SourceComponent: "observer", ConfigBinding: "env:LEDGER_RPC", TargetComponent: target})
		}
		err := compileToolVolumeSource(spec, &tool, map[string]AdapterSpec{"workspace": {}, "selected": {}})
		if target == "selected" {
			if err != nil || tool.ResourceSpec.VolumeSource != "runtime:selected" {
				t.Fatalf("volume source followed a different binding or workspace default: source=%s err=%v", tool.ResourceSpec.VolumeSource, err)
			}
		} else if err == nil {
			t.Fatalf("invalid volume target %q accepted", target)
		}
	}
}

func TestWorkspaceVolumeSourceOnlyReadsPublicAssets(t *testing.T) {
	for _, name := range []string{"valid", "runtime-state", "judge-private", "writable", "missing-workspace"} {
		t.Run(name, func(t *testing.T) {
			component := workload.ComponentSpec{Name: "editor", MountDomains: []string{VolumeDomainPublicAssets}}
			if name == "runtime-state" || name == "judge-private" {
				component.MountDomains = []string{name}
			}
			if name == "writable" {
				component.WritableMountDomains = []string{VolumeDomainPublicAssets}
			}
			tool := Tool{Code: "ide", ResourceSpec: ToolResourceSpec{VolumeSource: "workspace", Components: []workload.ComponentSpec{component}}}
			spec := contracts.SandboxCompositionSpec{WorkspaceRuntimeInstance: "selected"}
			runtimes := map[string]AdapterSpec{"other": {}}
			if name != "missing-workspace" {
				runtimes["selected"] = AdapterSpec{}
			}
			err := compileToolVolumeSource(spec, &tool, runtimes)
			if name == "valid" {
				if err != nil || tool.ResourceSpec.VolumeSource != "runtime:selected" {
					t.Fatalf("public assets used wrong runtime: source=%s err=%v", tool.ResourceSpec.VolumeSource, err)
				}
			} else if err == nil {
				t.Fatal("workspace source escaped public read-only assets")
			}
		})
	}
}

func TestVolumePersistenceRequiresARealSavePath(t *testing.T) {
	for _, tt := range []struct {
		name        string
		persistence string
		scope       string
		wantError   bool
	}{
		{name: "snapshot-always", persistence: VolumePersistenceSnapshot, scope: VolumeSnapshotAlways},
		{name: "snapshot-opt-in", persistence: VolumePersistenceSnapshot, scope: VolumeSnapshotEnabled},
		{name: "ephemeral", persistence: VolumePersistenceEphemeral, scope: VolumeSnapshotNever},
		{name: "snapshot-disabled", persistence: VolumePersistenceSnapshot, scope: VolumeSnapshotNever, wantError: true},
		{name: "archive-outside-workspace", persistence: VolumePersistenceObjectStorageCode, scope: VolumeSnapshotNever, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spec := AdapterSpec{WorkspaceDir: "/workspace", VolumeDomains: []VolumeDomainSpec{
				{Name: VolumeDomainWorkspace, MountPath: "/workspace", StudentAccess: VolumeAccessReadWrite, Persistence: VolumePersistenceObjectStorageCode, SnapshotScope: VolumeSnapshotAlways},
				{Name: "ledger", MountPath: "/ledger", StudentAccess: VolumeAccessNone, Persistence: tt.persistence, SnapshotScope: tt.scope},
			}}
			if err := normalizeAndValidateVolumeDomains(&spec); (err != nil) != tt.wantError {
				t.Fatalf("save path admission mismatch: %v", err)
			}
		})
	}
}
