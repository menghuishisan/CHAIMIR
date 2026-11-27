# infra/aa-bundler

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

ERC-4337 账户抽象 bundler 教学镜像。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；组合必须通过统一初始化资产把当前 EVM 链的 Rundler `chain-spec.toml` 写入 `/runtime-state/public-assets/erc4337/chain-spec.toml`，并由 `CHAIN_SPEC` 指向该文件。
- 该链规范必须声明实际 `id` 和实际部署的 ERC-4337 v0.6 EntryPoint 地址；不能省略、使用 Rundler 默认链地址或用 v0.7 地址冒充 v0.6。

## Secret / KMS

- Secret 环境键：SIGNER_PRIVATE_KEYS；配置键：ENABLED_ENTRY_POINTS、CHAIN_SPEC。

## 持久化与隔离边界

- 持久化目录：只读挂载 `runtime-state`，读取 `/runtime-state/public-assets/erc4337/chain-spec.toml`；bundler 不写入该安全域。
