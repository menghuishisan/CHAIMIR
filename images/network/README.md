# network

`network` 管理集群 CNI 与策略数据面镜像。Cilium 的运行、构建和 Envoy 组件都必须通过本目录的 manifest、Harbor digest 锁、Trivy、SBOM 与 Cosign 门禁后才能安装。

这里不包含 Kubernetes Chart 副本。Chart 保持其上游发布来源，部署层只将通过门禁的 `network/*@sha256:...` 注入 Chart values；不得将默认 Kindnet 与 Cilium 叠装。
