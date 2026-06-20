# tool/fabric-explorer

本镜像提供 Fabric 专用浏览器,用于通道、区块、链码和交易观察。镜像自身只包含 Hyperledger Explorer 前后端应用,不内置固定 Fabric 网络、PostgreSQL 数据库或示例证书。

`manifest.yaml` 声明 `tool.runtime_config_required=true`:只有当运行时/实验 WorkloadSpec 已真实提供 PostgreSQL 服务、Fabric connection profile 和最小权限证书挂载后,才能把该工具标记为可调度。学生只通过 M2 平台代理访问 Web UI,不得直连容器服务。
