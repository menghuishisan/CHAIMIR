# infra/rpc-gateway

本镜像提供单个 EVM JSON-RPC 上游的 Envoy 网关。上游不是固定 Service；组合编译器通过 `evm-json-rpc` 的 `upstream` 绑定注入 `CHAIMIR_RPC_UPSTREAM_URL`，入口在 `/tmp` 生成本实例配置后启动 Envoy。需要多个链时使用多个 gateway 实例分别绑定对应 runtime，不把链组合写进镜像。
