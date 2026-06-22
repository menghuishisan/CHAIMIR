# tool/blockscout

本镜像提供 Blockscout EVM 区块链浏览器,用于交易、区块和合约事件观察。

`manifest.yaml` 声明 `tool.runtime_config_required=true`:只有当运行时/实验 WorkloadSpec 已真实提供 EVM RPC、Blockscout PostgreSQL 数据库、最小 NetworkPolicy 和代理路由后,才能把该工具标记为可调度。学生只通过 M2 平台代理访问 Web UI,不得直连数据库、RPC 或容器服务。
