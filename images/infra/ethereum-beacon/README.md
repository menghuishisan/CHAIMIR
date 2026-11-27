# infra/ethereum-beacon

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Sigma Prime Lighthouse 官方 Beacon Node 镜像。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；`beacon-testnet` 目录、`config.yaml`、`genesis.ssz`、`genesis-block.ssz` 必须由平台受控对象存储、初始化资产和 CompositionSpec 注入，并校验归档 SHA-256。
- 不支持公网 checkpoint、内置主网替代、自行生成空 checkpoint 或 `--allow-insecure-genesis-sync`；材料缺失、路径越界或文件为空时严格失败。

## 持久化与隔离边界

- 持久化目录：`/runtime-state/beacon-testnet` 保存受控 testnet/checkpoint 材料，`/runtime-state/lighthouse` 保存 Beacon 数据库；两者均由 `runtime-state` 安全域注入或持久化，未声明持久化数据时不得自行落盘。

## readiness / health

- 就绪同时要求 Lighthouse 已按受控 testnet 目录加载 state/block checkpoint 并连接绑定的 Engine API。
