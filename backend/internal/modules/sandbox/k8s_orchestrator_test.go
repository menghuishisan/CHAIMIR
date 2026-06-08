// M2 K8s 编排 manifest 测试:覆盖安全基线和环境变量配置落地。
package sandbox

import (
	"os"
	"strings"
	"testing"

	"chaimir/internal/platform/config"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// TestNamespaceManifestEnforcesRestrictedPodSecurity 确认沙箱 Namespace 强制 Restricted Pod Security。
func TestNamespaceManifestEnforcesRestrictedPodSecurity(t *testing.T) {
	ns := namespaceManifest(SandboxCreateSpec{SandboxID: 9001, TenantID: 1001, Namespace: "sbx-9001"})
	if ns.Labels["pod-security.kubernetes.io/enforce"] != "restricted" {
		t.Fatalf("expected restricted pod security, got labels=%v", ns.Labels)
	}
}

// TestDenyAllNetworkPolicyBlocksIngressAndEgress 确认默认网络策略拒绝出入站。
func TestDenyAllNetworkPolicyBlocksIngressAndEgress(t *testing.T) {
	policy := denyAllNetworkPolicy("sbx-9001")
	if len(policy.Spec.PolicyTypes) != 2 {
		t.Fatalf("expected ingress+egress policy types, got %#v", policy.Spec.PolicyTypes)
	}
	if len(policy.Spec.Ingress) != 0 || len(policy.Spec.Egress) != 0 {
		t.Fatalf("deny-all policy must not include allow rules")
	}
}

// TestControlPlaneNetworkPolicyAllowsOnlyConfiguredBackend 确认控制面代理是精确放行,不是放开整个沙箱网络。
func TestControlPlaneNetworkPolicyAllowsOnlyConfiguredBackend(t *testing.T) {
	policy := controlPlaneNetworkPolicy("sbx-9001", config.SandboxConfig{
		ControlNamespace:     "chaimir-system",
		ControlPodLabelKey:   "app.kubernetes.io/name",
		ControlPodLabelValue: "chaimir-backend",
	}, []networkingv1.NetworkPolicyPort{{Port: intstrPtr(8545)}})
	if policy.Spec.PolicyTypes[0] != networkingv1.PolicyTypeIngress {
		t.Fatalf("expected ingress policy, got %#v", policy.Spec.PolicyTypes)
	}
	if len(policy.Spec.Ingress) != 1 || len(policy.Spec.Ingress[0].From) != 1 {
		t.Fatalf("expected one precise ingress peer, got %#v", policy.Spec.Ingress)
	}
	peer := policy.Spec.Ingress[0].From[0]
	if peer.NamespaceSelector == nil || peer.PodSelector == nil {
		t.Fatalf("control plane allow must include namespace and pod selectors: %#v", peer)
	}
}

// TestControlPlaneNetworkPolicyRestrictsDeclaredPorts 确认控制面来源也只能访问声明式端口。
func TestControlPlaneNetworkPolicyRestrictsDeclaredPorts(t *testing.T) {
	policy := controlPlaneNetworkPolicy("sbx-9001", config.SandboxConfig{
		ControlNamespace:     "chaimir-system",
		ControlPodLabelKey:   "app.kubernetes.io/name",
		ControlPodLabelValue: "chaimir-backend",
	}, []networkingv1.NetworkPolicyPort{{Port: intstrPtr(8545)}})
	if len(policy.Spec.Ingress) != 1 || len(policy.Spec.Ingress[0].Ports) != 1 {
		t.Fatalf("control plane allow must restrict ports, got %#v", policy.Spec.Ingress)
	}
	if policy.Spec.Ingress[0].Ports[0].Port == nil || policy.Spec.Ingress[0].Ports[0].Port.IntVal != 8545 {
		t.Fatalf("expected runtime rpc port 8545, got %#v", policy.Spec.Ingress[0].Ports[0].Port)
	}
}

// TestNamespaceQuotaUsesSandboxConfig 确认资源硬限来自配置,不是代码硬编码。
func TestNamespaceQuotaUsesSandboxConfig(t *testing.T) {
	quota := namespaceQuota(config.SandboxConfig{MaxCPU: "6", MaxMemory: "12Gi", MaxPods: "24"})
	cpu := quota.Spec.Hard[corev1.ResourceLimitsCPU]
	if cpu.String() != "6" {
		t.Fatalf("expected cpu quota from config, got %s", cpu.String())
	}
	memory := quota.Spec.Hard[corev1.ResourceLimitsMemory]
	if memory.String() != "12Gi" {
		t.Fatalf("expected memory quota from config, got %s", memory.String())
	}
}

// TestBuildDeclaredContainerMountsWorkspaceOnlyWhenRequested 确认工具容器仅在声明共享工作区时挂载卷。
func TestBuildDeclaredContainerMountsWorkspaceOnlyWhenRequested(t *testing.T) {
	withWorkspace := buildDeclaredContainer(ContainerSpec{
		Name:           "tool-a",
		ImageURL:       "registry.local/tool-a:v1",
		Command:        []string{"tool-a"},
		MountWorkspace: boolRef(true),
	}, "registry.local/tool-a:v1", "/workspace", config.SandboxConfig{})
	if len(withWorkspace.VolumeMounts) != 1 || withWorkspace.VolumeMounts[0].MountPath != "/workspace" {
		t.Fatalf("expected workspace mount when mount_workspace=true, got %#v", withWorkspace.VolumeMounts)
	}

	withoutWorkspace := buildDeclaredContainer(ContainerSpec{
		Name:           "tool-b",
		ImageURL:       "registry.local/tool-b:v1",
		Command:        []string{"tool-b"},
		MountWorkspace: boolRef(false),
	}, "registry.local/tool-b:v1", "/workspace", config.SandboxConfig{})
	if len(withoutWorkspace.VolumeMounts) != 0 {
		t.Fatalf("expected no workspace mount when mount_workspace=false, got %#v", withoutWorkspace.VolumeMounts)
	}
}

// TestRestrictedSecurityContextUsesReadOnlyRootFilesystem 确认容器根文件系统只读,可写区域只通过挂载卷提供。
func TestRestrictedSecurityContextUsesReadOnlyRootFilesystem(t *testing.T) {
	security := restrictedSecurityContext()
	if security.ReadOnlyRootFilesystem == nil || !*security.ReadOnlyRootFilesystem {
		t.Fatalf("sandbox containers must use read-only root filesystem, got %#v", security.ReadOnlyRootFilesystem)
	}
	if security.AllowPrivilegeEscalation == nil || *security.AllowPrivilegeEscalation {
		t.Fatalf("sandbox containers must block privilege escalation")
	}
}

// TestRuntimeWorkloadUsesRuntimeDefaultSeccomp 确认沙箱 Pod 启用默认 seccomp profile。
func TestRuntimeWorkloadUsesRuntimeDefaultSeccomp(t *testing.T) {
	workload := runtimeWorkloadManifest(SandboxCreateSpec{
		SandboxID: 9001,
		TenantID:  1001,
		Namespace: "sbx-9001",
		Runtime: RuntimeDefinition{AdapterSpec: RuntimeAdapterSpec{
			WorkspaceDir: "/workspace",
			RuntimeContainer: ContainerSpec{
				Name:    "runtime",
				Command: []string{"anvil"},
			},
		}},
		Image: RuntimeImageDefinition{ImageURL: "harbor.chaimir.local/runtime/evm:v1"},
	}, config.SandboxConfig{})
	seccomp := workload.Spec.Template.Spec.SecurityContext.SeccompProfile
	if seccomp == nil || seccomp.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatalf("runtime workload must use RuntimeDefault seccomp profile, got %#v", seccomp)
	}
}

// TestBuildProbeUsesConfiguredDefaults 确认探针缺省阈值来自 SandboxConfig。
func TestBuildProbeUsesConfiguredDefaults(t *testing.T) {
	probe := buildProbe(ProbeSpec{Type: "tcp", Port: "rpc"}, config.SandboxConfig{
		ProbeDefaultPeriodSeconds:    7,
		ProbeDefaultFailureThreshold: 11,
	})
	if probe == nil {
		t.Fatalf("expected probe")
	}
	if probe.PeriodSeconds != 7 || probe.FailureThreshold != 11 {
		t.Fatalf("probe defaults must come from config, got period=%d failure=%d", probe.PeriodSeconds, probe.FailureThreshold)
	}
}

// TestPrepullDaemonSetUsesRuntimeDefaultSeccomp 确认预拉取 DaemonSet 同样启用默认 seccomp profile。
func TestPrepullDaemonSetUsesRuntimeDefaultSeccomp(t *testing.T) {
	ds := prepullDaemonSetManifest(ImagePrepullSpec{
		RuntimeImageID: 88,
		ImageURL:       "harbor.chaimir.local/runtime/evm:v1",
	}, config.SandboxConfig{})
	seccomp := ds.Spec.Template.Spec.SecurityContext.SeccompProfile
	if seccomp == nil || seccomp.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatalf("prepull daemonset must use RuntimeDefault seccomp profile, got %#v", seccomp)
	}
}

// TestPrepullNamespaceUsesSandboxConfig 确认镜像预拉取命名空间来自环境配置。
func TestPrepullNamespaceUsesSandboxConfig(t *testing.T) {
	cfg := config.SandboxConfig{
		PrepullNamespace:     "chaimir-prepull-prod",
		PrepullRequestCPU:    "20m",
		PrepullRequestMemory: "48Mi",
		PrepullLimitCPU:      "200m",
		PrepullLimitMemory:   "256Mi",
	}
	ns := prepullNamespaceManifest(cfg)
	if ns.Name != "chaimir-prepull-prod" {
		t.Fatalf("expected prepull namespace from config, got %s", ns.Name)
	}
	ds := prepullDaemonSetManifest(ImagePrepullSpec{RuntimeImageID: 88, ImageURL: "harbor.chaimir.local/runtime/evm:v1"}, cfg)
	if ds.Namespace != "chaimir-prepull-prod" {
		t.Fatalf("expected daemonset namespace from config, got %s", ds.Namespace)
	}
}

// TestPrepullDaemonSetUsesConfiguredResources 确认预拉取 DaemonSet 资源来自环境配置。
func TestPrepullDaemonSetUsesConfiguredResources(t *testing.T) {
	ds := prepullDaemonSetManifest(ImagePrepullSpec{
		RuntimeImageID: 88,
		ImageURL:       "harbor.chaimir.local/runtime/evm:v1",
	}, config.SandboxConfig{
		PrepullRequestCPU:    "20m",
		PrepullRequestMemory: "48Mi",
		PrepullLimitCPU:      "200m",
		PrepullLimitMemory:   "256Mi",
	})
	container := ds.Spec.Template.Spec.Containers[0]
	if container.Resources.Requests.Cpu().String() != "20m" {
		t.Fatalf("expected configured prepull cpu request, got %s", container.Resources.Requests.Cpu().String())
	}
	if container.Resources.Limits.Memory().String() != "256Mi" {
		t.Fatalf("expected configured prepull memory limit, got %s", container.Resources.Limits.Memory().String())
	}
}

// TestPrepullImageChecksFailureEvents 确认预拉取成功判定会检查 ImagePullBackOff/调度失败等 K8s 事件。
func TestPrepullImageChecksFailureEvents(t *testing.T) {
	src, err := os.ReadFile("k8s_orchestrator.go")
	if err != nil {
		t.Fatalf("read k8s orchestrator: %v", err)
	}
	body := string(src)
	start := strings.Index(body, "func (o *k8sOrchestrator) PrepullImage(")
	end := strings.Index(body, "// Create 创建沙箱完整数据面")
	if start < 0 || end < start {
		t.Fatalf("PrepullImage function block not found")
	}
	if !strings.Contains(body[start:end], "prepullFailureFromEvents") {
		t.Fatalf("PrepullImage must inspect Kubernetes failure events before marking prepull complete")
	}
}

// TestRuntimeBindingCarriesServicePorts 确认 L2 能力拿到声明式端口,不会回退硬编码 RPC 端点。
func TestRuntimeBindingCarriesServicePorts(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "runtime-pod"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{
			{Name: "runtime"},
		}},
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "runtime-svc"},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
			{Name: "rpc", Port: 18545},
			{Name: "remix", Port: 18080},
		}},
	}

	binding := runtimeBindingFromPodAndService("sbx-9001", pod, service)
	if binding.ServiceName != "runtime-svc" {
		t.Fatalf("expected service name from Service, got %q", binding.ServiceName)
	}
	if binding.PortByName["rpc"] != 18545 || binding.PortByName["remix"] != 18080 {
		t.Fatalf("expected named service ports, got %#v", binding.PortByName)
	}
}

// TestK8sManifestBuildersAvoidMustParse 确认坏配置不会通过 MustParse 在请求路径触发 panic。
func TestK8sManifestBuildersAvoidMustParse(t *testing.T) {
	src, err := os.ReadFile("k8s_orchestrator.go")
	if err != nil {
		t.Fatalf("read k8s orchestrator: %v", err)
	}
	if strings.Contains(string(src), "resource.MustParse") {
		t.Fatalf("k8s manifest builders must use checked quantity parsing, not resource.MustParse")
	}
}

// TestBuildResourcesDoesNotPanicOnInvalidQuantity 确认声明式资源异常不会触发 panic。
func TestBuildResourcesDoesNotPanicOnInvalidQuantity(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("buildResources must not panic on invalid quantity: %v", r)
		}
	}()
	_ = buildResources(ResourceSpec{Requests: ResourcePair{CPU: "bad-cpu"}})
}

func boolRef(v bool) *bool {
	return &v
}

func intstrPtr(v int32) *intstr.IntOrString {
	p := intstr.FromInt32(v)
	return &p
}
