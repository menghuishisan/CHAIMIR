# gvisor 组件:沙箱强制隔离运行时

不可信沙箱 Pod(运行学生代码/工具)强制在 gVisor(runsc)隔离运行时中执行:后端对沙箱运行时/工具 Pod 注入 `runtimeClassName`(取自必填配置 `SANDBOX_RUNTIME_CLASS`,默认 `gvisor`),`gvisor-runtimeclass.yaml` 声明对应 `RuntimeClass/gvisor`(handler=`runsc`)。

`RuntimeClass` 只是"引用名 → containerd 运行时处理器"的映射。**要真正生效,沙箱节点必须先装 runsc 并在 containerd 注册 `runsc` 处理器。** runsc 是节点/集群前置运行时(与 CNI、containerd 同类),不作为 `images/` 工作负载镜像治理,按集群形态供给:

## 生产 / 私有化(云厂商原生 gVisor 节点池,优先)

- **GKE**:节点池加 `--sandbox type=gvisor`。
- **阿里云 ACK / 腾讯 TKE / 华为 CCE**:创建"安全沙箱/gVisor 运行时"节点池,节点选择器对齐 `SANDBOX_NODE_SELECTOR`。
- **AWS EKS / Azure AKS / 自建 kubeadm**:按 https://gvisor.dev/docs/user_guide/install/ 在沙箱节点装 `runsc` 与 `containerd-shim-runsc-v1`,在 containerd 注册 `runsc` 运行时后重启;建议烘进节点镜像。
- 装好节点后 `kubectl apply -k deploy/components/gvisor` 装入 RuntimeClass(生产由部署流程统一执行)。

## 本地开发(Docker Desktop / kind)

节点直连 gVisor 发布源常被网络阻断,故用宿主下载 + 分发到节点的方式,统一封装为 make 目标:

```sh
cd deploy
make gvisor-up-local   # 宿主下载 runsc → 分发到各节点 → 注册 containerd runsc 运行时并重启 → apply RuntimeClass
make gvisor-check      # 校验 RuntimeClass 就绪并起一个 gVisor 测试 Pod 验证内核隔离
```

注意:Docker Desktop/kind 节点是容器,上述改动**不持久**——集群重建/重置后需重跑 `make gvisor-up-local`。这是本地权宜,生产以云节点池/节点镜像为准。

## 缺失时的行为(fail-closed)

集群未装 runsc / 无 `RuntimeClass/gvisor` 时,沙箱 Pod 会一直 Pending、后端 `SANDBOX_RUNTIME_CLASS` 缺失则启动失败。这是有意的安全前置,不提供共享内核回退。

## 验证要点

`make gvisor-check` 起的测试 Pod 内 `dmesg` 应含 gVisor 标识;建议把它连同"沙箱内 fork 炸弹受限、出网被拒"两条断言纳入部署冒烟(见 `docs/总-验收标准.md`)fail-closed 门禁。
