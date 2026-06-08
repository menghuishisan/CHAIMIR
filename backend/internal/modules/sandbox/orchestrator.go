// M2 沙箱编排接口:定义模块内部对 K8s 数据面的最小依赖,便于生产实现与测试替身分离。
package sandbox

import (
	"context"
	"io"
)

// Orchestrator 创建/销毁沙箱 K8s 资源;业务状态机留在 service 层。
type Orchestrator interface {
	// PrepullImage 创建或更新受控 DaemonSet,并返回目标节点真实拉取进度。
	PrepullImage(ctx context.Context, spec ImagePrepullSpec) (ImagePrepullStatus, error)
	// Create 分配 Namespace、NetworkPolicy、Pod/Service 等数据面资源。
	Create(ctx context.Context, spec SandboxCreateSpec) error
	// WaitReady 等待阶段一环境就绪,失败时返回 K8s 原始错误供日志记录。
	WaitReady(ctx context.Context, namespace string) error
	// SnapshotAvailable 检查集群是否安装可用的 CSI VolumeSnapshot 能力。
	SnapshotAvailable(ctx context.Context) error
	// SnapshotWorkspace 对沙箱工作区 PVC 创建 CSI VolumeSnapshot。
	SnapshotWorkspace(ctx context.Context, spec SnapshotSpec) (SnapshotResult, error)
	// Pause 暂停沙箱内工作负载,保留持久卷与命名空间。
	Pause(ctx context.Context, binding SandboxRuntimeBinding) error
	// Resume 恢复已暂停的沙箱工作负载。
	Resume(ctx context.Context, spec SandboxCreateSpec) error
	// Recycle 删除沙箱 Namespace 并释放资源。
	Recycle(ctx context.Context, namespace string) error
	// RuntimeBinding 返回沙箱运行时主容器的定位信息。
	RuntimeBinding(ctx context.Context, namespace string) (SandboxRuntimeBinding, error)
	// ToolEndpoint 返回工具代理的 Service 目标。
	ToolEndpoint(ctx context.Context, namespace, toolCode string) (SandboxToolEndpoint, error)
	// Exec 在沙箱容器内执行命令,供终端/文件/初始化脚本/自检复用。
	Exec(ctx context.Context, binding SandboxRuntimeBinding, command []string, stdin io.Reader, stdout, stderr io.Writer, tty bool) error
}
