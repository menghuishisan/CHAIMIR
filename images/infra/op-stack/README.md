# infra/op-stack

本镜像只提供官方 `op-node` 组件。完整 Optimism 风格 L2 实验不是一个镜像,必须由 M2 WorkloadSpec 组合 op-node、执行层、批次组件、L1/L2 RPC、Secret 和 NetworkPolicy。

官方 `op-node:v1.19.0` 预编译镜像当前会触发高危供应链门禁。本目录改为从官方 `ethereum-optimism/optimism` 的 `op-node/v1.19.0` 源码包重建,固定源码 SHA256,使用安全 Go 工具链,并升级 Trivy 指出的 `quic-go`、`otel` 与配套 `go-libp2p` 依赖。构建产物仍必须通过 Trivy、Cosign 和 digest 锁后才能进入 `SANDBOX_IMAGE_ATTESTATIONS_JSON`。
