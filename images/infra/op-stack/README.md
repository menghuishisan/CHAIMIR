# infra/op-stack

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Optimism 风格 L2 教学编排镜像。

## 合法 runtime / provider

- P2P：当前 manifest 不声明 P2P 能力、端口或 peer 材料，因此组件配置固定为 `OP_NODE_P2P_DISABLE=true`；不能使用上游默认公网 bootnode，也不能把未声明的 P2P 行为写成支持能力。
- 受控写入链参数：CompositionSpec 可显式设置 `sequencer_enabled` 和 `sequencer_l1_confs`，分别映射为 `OP_NODE_SEQUENCER_ENABLED`、`OP_NODE_SEQUENCER_L1_CONFS`；只有同时提供受控 L1 execution、L2 Engine、Beacon、rollup 和链配置材料的组合才允许开启。未显式设置时沿用官方默认关闭，不把 verifier 组件误写成 sequencer 能力。

## bindings 与 links

- rollup_config 和 l1_chain_config 由同一 CompositionSpec 显式提供，信标地址只由 l1_beacon 连接生成。
- 监听地址、服务端口与禁用 P2P 是固定运行约束，不作为可编辑参数。

## command / args 语义

- 构建阶段使用固定的官方 mikefarah/yq v4.44.3 解析 superchain-registry TOML；版本和 SHA-256 由 manifest 固定，不能替换为 Debian 的 Python yq 包。

## 初始化材料与运行时注入

- L1 genesis、L2 genesis、rollup、checkpoint、与 finalized checkpoint 对齐的 `l1-execution-state.tar`、Hyperlane、Chainlink、合约或 subgraph 材料必须由平台受控对象存储、初始化资产和 CompositionSpec 注入并校验来源及 SHA-256；本镜像不内置创世或 rollup 材料，缺失时严格失败。
- OP_NODE_ROLLUP_L1_CHAIN_CONFIG 必须指向与 OP_NODE_ROLLUP_CONFIG 同一受控材料归档中的 L1 execution genesis，且 chainId 必须有效。
