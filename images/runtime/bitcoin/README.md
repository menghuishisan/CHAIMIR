# runtime/bitcoin

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Bitcoin regtest 教学运行时镜像。

## command / args 语义

- `deploy` 的 `data` 必须是非空文本，按 UTF-8 或 `0x` 十六进制写入最多 80 字节的 OP_RETURN；`tx` 接受正数 `amount`（至多八位小数）和可选文本 `to`，省略地址时转入本钱包新地址。
- `query` 只接受 `chain`、`balance`、`tx:<64 位十六进制交易编号>`，不创建/加载钱包、补资金或挖块。余额读取已有加载钱包，尚未准备钱包时显式失败。合法发布/转账才初始化钱包、补充成熟 coinbase 并确认交易；错误动作或载荷在调用节点前拒绝。
- 节点启用 `txindex`，确认后的交易仍可查询。依据当前固定 Bitcoin Core 24 的官方 `getrawtransaction` 契约，无 `blockhash` 的已确认交易查询需要索引；索引随本实例账本卷保存。
