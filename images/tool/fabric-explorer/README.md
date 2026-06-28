# tool/fabric-explorer

本镜像提供 Fabric 专用浏览器,用于通道、区块、链码和交易观察。镜像自身只包含 Hyperledger Explorer 前后端应用,不内置固定 Fabric 网络、PostgreSQL 数据库或示例证书。

`manifest.yaml` 声明 `tool.runtime_config_required=true`:只有当运行时/实验 WorkloadSpec 已真实提供 PostgreSQL 服务、Fabric connection profile 和最小权限证书挂载后,才能把该工具标记为可调度。学生只通过 M2 平台代理访问 Web UI,不得直连容器服务。

当前 Hyperledger Explorer 上游 1.1.8/2.0.0 均不能通过平台 HIGH/CRITICAL 阻断门禁,本镜像单元保留用于治理追踪和后续替换,但 `supply_chain.deployable=false`。已排查的替代方向:

- FabEx:功能模型接近标准 Fabric 浏览器,但引入 MongoDB 工作负载,且源码依赖扫描仍存在 HIGH/CRITICAL,不能直接替换。
- Fabric-X Block Explorer:实现更新,但依赖 Fabric-X sidecar,不是标准 Fabric connection profile/证书模型的等价替换;当前源码依赖扫描也仍未过 HIGH/CRITICAL 门禁。

恢复准入只能来自官方安全版本、可验证源码重建或功能等价且已完成 WorkloadSpec 契约适配的替代组件,不得用静态页面或简化浏览器冒充完整区块浏览器能力。
