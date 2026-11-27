# infra/rpc-gateway

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

DApp 多节点 RPC 网关教学镜像。

## 合法 runtime / provider

- 上游 URL 必须带显式端口，且不含凭据、路径、查询、片段或 IPv6 zone。HTTP 明文连接；HTTPS 使用 Envoy 原生 TLS、系统 CA 和 DNS/IP SAN 校验，DNS 同时发送 SNI。

## bindings 与 links

- 必需绑定 `upstream`：`evm-json-rpc` 能力、`rpc` 端点、HTTP 协议，源组件 `infra`；允许明确选择一个或多个目标，全部目标就绪后才启动。
- M2 将完整地址数组写入 `CHAIMIR_RPC_UPSTREAM_URLS`，同一绑定的多条 link 各自生成网络规则。没有单地址环境变量或手工填上游的旁路。
- 所有节点进入同一 Envoy `STRICT_DNS` / `ROUND_ROBIN` 集群，每个端点独立匹配明文或 TLS 配置；不自动重试交易。上游须属于同一条链，独立账本使用不同网关。
