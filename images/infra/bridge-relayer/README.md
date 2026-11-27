# infra/bridge-relayer

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

基于 Hyperlane 官方 agent 的跨链桥中继教学服务镜像。

## 持久化与隔离边界

- 持久化目录：`/runtime-state/hyperlane/db` 仅保存中继数据库状态；含 signer Secret 的合并配置只写入临时挂载 `/tmp/chaimir-hyperlane/merged-config.json`，权限为 `0600`，Pod 生命周期结束后销毁。

## 安全边界

- 持久化例外仅用于 relayer 数据库；签名私钥只存在 Secret 投影和临时配置文件，不写入 `runtime-state`。
- 合并配置使用新建的 `0600` 临时文件，写入与关闭成功后原子替换；失败保留上一份文件并清理临时产物。配置目录强制为 `0700`，交接官方 agent 前清除原始签名私钥环境变量。

## source/destination 配置校验

- `HYP_RELAYCHAINS` 必须恰好包含两个不同且无空白的链名；source 与 destination 的 `chainId`、`domainId` 必须分别为正整数，domain 不得相同。
- base config 必须是无重复 JSON 对象键的完整文档；必需字段缺失、空值、首尾空白或地址格式非法时，配置入口立即失败。
- 每个链的 `chainId` 必须与该链显式 RPC binding 返回的 `eth_chainId` 一致；Mailbox `localDomain()` 必须与 `domainId` 一致。
- mailbox、interchainGasPaymaster、validatorAnnounce 和 merkleTreeHook 必须是非零 EVM 地址，且在对应 runtime 上 `eth_getCode` 返回已部署合约字节码。
- RPC 检查使用显式 source/destination binding；任一查询错误、错链、错域或未部署地址均严格失败，不启动 relayer。
- RPC 地址禁止查询和片段，包括空的 `?` 或 `#`；响应成员名称区分大小写，成功响应不得含 `error` 字段，重复键、错误编号、读取或关闭失败均拒绝。
