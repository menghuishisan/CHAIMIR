// Package k8s 封装 client-go,供 M2 沙箱编排创建/销毁动态命名空间与 Pod。
// 依据 docs/总-部署架构设计.md §二:后端 ServiceAccount 最小 RBAC。
// 本层只提供通用 Kubernetes 原语(clientset/exec),具体沙箱编排逻辑属于 M2(sandbox)。
package k8s

import (
	"context"
	"fmt"
	"io"

	"chaimir/internal/platform/config"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// Client 封装 K8s 客户端集与沙箱配置。
type Client struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	restConfig    *rest.Config
	imageRegistry string
}

// New 创建 K8s 客户端:KubeconfigPath 为空时用 in-cluster 配置,否则用 kubeconfig 文件。
func New(cfg config.SandboxConfig) (*Client, error) {
	restCfg, err := buildRestConfig(cfg.KubeconfigPath)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("创建 K8s 客户端失败: %w", err)
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("创建 K8s dynamic 客户端失败: %w", err)
	}
	return &Client{clientset: cs, dynamicClient: dyn, restConfig: restCfg, imageRegistry: cfg.ImageRegistry}, nil
}

// buildRestConfig 根据部署形态选择 in-cluster 配置或本地 kubeconfig。
func buildRestConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("加载 in-cluster 配置失败: %w", err)
		}
		return cfg, nil
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("加载 kubeconfig(%s)失败: %w", kubeconfigPath, err)
	}
	return cfg, nil
}

// Clientset 暴露底层客户端集供 M2 编排使用。
func (c *Client) Clientset() *kubernetes.Clientset { return c.clientset }

// Dynamic 暴露 dynamic client,供模块操作已安装的标准 CRD。
func (c *Client) Dynamic() dynamic.Interface { return c.dynamicClient }

// ImageRegistry 返回配置中的镜像仓库前缀,供上层拼运行时/工具镜像。
func (c *Client) ImageRegistry() string { return c.imageRegistry }

// Healthz 探测 API Server 连通。
func (c *Client) Healthz(ctx context.Context) error {
	if _, err := c.clientset.Discovery().ServerVersion(); err != nil {
		return fmt.Errorf("K8s API 连通性检查失败: %w", err)
	}
	return nil
}

// Exec 在目标容器中执行命令并透传输入输出流,供 M2 终端/文件/初始化脚本复用。
func (c *Client) Exec(
	ctx context.Context,
	namespace, podName, container string,
	command []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	tty bool,
) error {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: container,
		Command:   command,
		Stdin:     stdin != nil,
		Stdout:    stdout != nil,
		Stderr:    !tty && stderr != nil,
		TTY:       tty,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("创建 Kubernetes exec 会话失败: %w", err)
	}
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	}); err != nil {
		return fmt.Errorf("执行 Kubernetes exec 失败: %w", err)
	}
	return nil
}
