# gVisor 沙箱运行时

> 更新：2026-11-27。

运行不可信学生代码和工具的 Pod 必须使用已验证的隔离运行时。后端必填配置 `SANDBOX_RUNTIME_CLASS` 指定 RuntimeClass 名；本仓库部署基线使用 `gvisor`，handler 为 `runsc`，不是后端缺配置时的默认值。

## 节点前置

RuntimeClass 只映射运行时处理器。沙箱节点须安装 runsc 与匹配的 containerd shim，并在 containerd 注册 handler；安装与节点生命周期由部署层负责，不作为业务工作负载镜像。托管节点池须按提供者官方方式安装，并核验实际 handler、调度标签和隔离行为；名为“安全沙箱”的产品不能直接视为 gVisor。

自管 Kind 节点使用项目固定 GCS 对象版本和 SHA512 校验的安装入口：

```sh
cd deploy
make gvisor-up
make gvisor-check
```

节点重建后重新执行安装与检查。没有 handler 或 RuntimeClass 时不能承接沙箱负载；后端缺配置则启动失败，不回退到共享宿主机内核的默认运行时。

## 资源限制与验收

隔离验收核对实际 RuntimeClass、节点 handler 和 Pod 内 gVisor 标识。`make gvisor-check` 的隔离探针不代表资源防滥用已通过。

每个沙箱节点须用 kubelet 原生 `podPidsLimit` 设置 Pod 进程上限并配置节点 PID 预留；使用运行时支持的原生 rlimit 约束文件描述符。明确记录实际配置值，在真实 gVisor Pod 中验证进程/线程与 fd 耗尽受限、节点保留可用性、非法出网被拒及测试资源回收。配置存在和内核隔离标识不能代替超限行为验证。

安全要求见[M2 安全设计](../../../docs/02-沙箱引擎/06-安全设计.md#3-资源限制与防滥用)，部署职责见[部署架构](../../../docs/总-部署架构设计.md)，上线条件见[验收标准](../../../docs/总-验收标准.md)。
