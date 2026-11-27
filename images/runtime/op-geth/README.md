# runtime/op-geth

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Optimism OP Labs 官方 OP Geth 源码重建执行层镜像，并内置平台受控的教学链创世材料。

## 端口与服务

- 服务端口由镜像和组合端点契约固定，不在编排表单中提供改端口选项。

## 初始化材料与运行时注入

- 镜像内置默认创世文件：`/opt/chaimir/genesis/op-geth.json`，由平台候选构建流程固定生成并在入口校验 SHA-256；它是本项目受控教学链的执行层创世，不是公网链或租户私有材料。
- OP-Stack 合法 CompositionSpec 使用同一平台受控材料归档，将 `genesis.json` 注入 `runtime-state/op-geth/genesis.json`，并显式设置 `OP_GETH_GENESIS_PATH`；这不是环境分支，也不是宿主机路径覆盖，材料必须来自 init-assets/Secret-KMS 编排链路并经过来源及 SHA-256 校验。
- 运行时工作目录：`/runtime-state/op-geth`，用于链状态、JWT 运行期材料和节点数据；由 CompositionSpec 的 `runtime-state` 卷持久化。
- 受控 OP-Stack 材料生成会将平台统一部署账户 `0xfdaf0ea969c98d0a8410d76bac4ac8838d13c9d4` 写入 L2 allocs，供合法的真实合约流程使用。该地址须与当前 Secret/KMS 管理的部署身份一致；轮换账户时重新生成并审核配套创世材料，不能只替换私钥。归档不包含私钥。

## Secret / KMS

- 必需材料映射：`OP_GETH_JWT_SECRET` 来自平台 Secret/KMS；默认创世使用镜像内 `/opt/chaimir/genesis/op-geth.json`，OP-Stack CompositionSpec 则使用同一受控归档注入的 `runtime-state/op-geth/genesis.json`，不得使用宿主机路径或公网材料。

## 持久化与隔离边界

- 持久化目录：`/runtime-state/op-geth`。当 Beacon finalized checkpoint 引用已生成的 L1 execution head 时，组合初始化还必须从同一受控归档恢复 `l1-execution-state.tar`；该归档只包含 execution 数据库和 genesis 一致性标记，不包含 JWT 或账户私钥。
