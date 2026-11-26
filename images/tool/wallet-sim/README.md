# tool/wallet-sim

在当前沙箱内创建或导入临时教学账户，生成可恢复验证的 Ethereum 消息签名与 EVM 交易签名。交易广播由工作台交给链运行时完成，工具不直连节点。

- `GET /`：操作页面；`GET /healthz`：就绪检查；`GET /accounts`：当前教学账户地址。
- `POST /import`：导入教学测试私钥，返回该次导入的账户地址。
- `POST /sign`：按 personal_sign 语义签名消息，返回签名、签名账户和恢复地址。
- `POST /tx`：按填写的目标、金额、Nonce、Gas、Gas Price、Chain ID 和 Data 签名交易，返回原始交易及哈希。

签名后把 `{"raw_transaction":"0x..."}` 填入工作台链交易参数，选定对应的 Geth、Foundry 或 Hardhat 实例再提交；工具不会自动向链发送，账户须有足够余额。

每个签名操作固定使用进入该操作时的账户；并发导入只影响后续操作，返回地址始终与签名一致。账户仅留在工具进程内存，重启后重新生成，不写入平台数据库或工作区。不得使用生产私钥；平台与链运行身份仍由 Secret/KMS 注入。

页面请求使用工具入口的相对路径，保留实验或竞赛网关前缀及其授权范围。

运行配置为 `CHAIMIR_IMAGE_PORT`、`CHAIMIR_MAX_BODY_BYTES`、`CHAIMIR_HTTP_TIMEOUT_SECONDS`，均须为正整数。端口、挂载、资源和供应链以 `manifest.yaml` 为准；容器保持非 root、只读根文件系统及默认拒绝网络。

验收除 manifest 自检外，还须在真实组合中验证签名恢复、交易广播、并发导入归属、错误输入和回收。纯函数检查不能证明链联动或浏览器业务已通过。
