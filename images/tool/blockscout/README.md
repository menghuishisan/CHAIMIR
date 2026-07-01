# tool/blockscout

本镜像提供 Blockscout EVM 区块链浏览器,用于交易、区块和合约事件观察。

`manifest.yaml` 已把 Blockscout Web 容器、专用 PostgreSQL 组件、Service 与最小 NetworkPolicy 写入同一工具 WorkloadSpec。M2 只在 EVM 运行时提供已声明 RPC 端口时允许调度该工具;学生只通过 M2 平台代理访问 Web UI,不得直连数据库、RPC 或容器服务。
