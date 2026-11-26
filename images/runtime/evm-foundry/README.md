# runtime/evm-foundry

提供 Anvil EVM 链和固定版本的 Forge、Cast、Chisel 与 Solidity 编译器。版本、参数、端口和供应链约束以 manifest 与 Dockerfile 为准；不在运行时联网下载编译器。

原生适配器通过标准输入的一份 JSON 对象执行动作：

- `deploy` 接受字节码或已有项目产物；仅在显式 `compile:true` 时按 `project_dir`、`artifact` 离线编译。`contract_ref` 保存已验证部署的地址，构造参数可按声明引用已有地址。
- `tx` 使用 `to` 或 `target_ref`、`data` 和可选金额。账户来自显式私钥、解锁账户或运行时 Secret；助记词只在容器内派生地址。`expect_revert:true` 只适用于显式解锁账户和正整数 `gas_limit`，必须取得真实回滚回执。
- 已签名交易使用独立载荷 `{"raw_transaction":"0x..."}`，须为非空完整字节，不能混入其他字段；原样广播，不读取平台账户或重签。
- `query` 支持 `latest`、`blockNumber`、`chainId`、`balance:<address>`、`code:<address>`、`counter:last`；查询不改变现场，其他目标明确失败。

普通部署和交易必须取得成功回执及合法交易哈希；部署还须取得非零地址后才更新引用。部署引用不能含路径或特殊目录名。普通发送复用 Cast 的超时；原始交易复用 `cast publish` 与系统 `timeout`，时限统一来自 `FOUNDRY_TRANSACTION_TIMEOUT`，非法配置拒绝执行。

镜像内置的 genesis 仅用于受控平台自检。业务链材料须由冻结组合注入并校验来源与摘要，不能把自检状态当作业务快照。主容器挂载共享工作区、独立账本和公开素材，不提供学生 shell；学生文件操作走 workspace 工具。链重置由 M2 管理，保留其他链账本和共享工作区。

验收先执行 manifest 自检，再验证实际部署、调用、签名广播、回滚、查询、引用绑定和链重置。镜像启动、真实链结果、持久化与资源回收分别留证；纯脚本替身不能代替真实组合验收。
