# infra/op-stack

本镜像只提供官方 `op-node` 组件。完整 Optimism 风格 L2 实验不是一个镜像,由 M2 WorkloadSpec 组合本镜像、`runtime/op-geth` 执行层、`infra/ethereum-beacon` Beacon provider、L1 RPC、rollup 配置、JWT Secret 和 NetworkPolicy。

官方 `op-node:v1.19.2` 预编译镜像当前会触发高危供应链门禁。本目录改为从官方 `ethereum-optimism/optimism` 的 `op-node/v1.19.2` 源码包重建,固定源码 SHA256,使用安全 Go 工具链,并升级 Trivy 指出的 `quic-go`、`otel` 与配套 `go-libp2p` 依赖。构建产物仍必须通过 Trivy、Cosign 和 digest 锁后才能进入 `PLATFORM_IMAGE_ATTESTATIONS_JSON`。

运行期必须通过命名 bindings 提供 `evm-json-rpc` 的 L1 执行端点、`evm-engine-api` 的 L2 Engine API 和 `ethereum-beacon-api` 的 Beacon HTTP 端点;`OP_NODE_ROLLUP_CONFIG` 指向组合生成的只读 rollup 配置文件,`CHAIMIR_OP_NODE_L2_ENGINE_JWT` 由 Secret 注入。入口会把 JWT 写入 runtime-state 临时文件并设置官方 `OP_NODE_L2_ENGINE_AUTH`,缺少任一连接、配置或 Secret 时拒绝启动。
