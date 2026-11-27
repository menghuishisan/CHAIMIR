# runtime/evm-besu

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Hyperledger Besu 企业 EVM 教学运行时镜像。

## 初始化材料与运行时注入

- QBFT genesis 的预置部署账户为平台统一 EVM 部署账户 `0xfdaf0ea969c98d0a8410d76bac4ac8838d13c9d4`，地址不是密钥；真实签名仍只能来自平台 Secret/KMS 的 `CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY`。轮换该 Secret 时必须按同一材料生成流程重新生成 genesis。
