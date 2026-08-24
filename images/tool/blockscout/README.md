# tool/blockscout

本镜像提供 Blockscout EVM 区块链浏览器,用于交易、区块和合约事件观察。

`manifest.yaml` 只声明 Blockscout Web 组件以及对 `evm-json-rpc`、`postgres-compatible` 的命名绑定。数据库由组合编译器按能力解析并作为独立组件展开，不能由该工具固定或内嵌。M2 把数据库地址、用户、名称和 Secret 注入后,入口生成官方 `DATABASE_URL`;只有两条绑定、数据库 Secret 和 `SECRET_KEY_BASE` 均已注入并健康时才允许启动。学生只通过 M2 平台代理访问 Web UI,不得直连数据库、RPC 或容器服务。
