# observability/prometheus

Prometheus 从官方源码固定版本构建,用于升级存在高危漏洞的 Go 依赖并保持官方运行参数。监控入口配置统一通过 `deploy/config/chaimir.env` 的环境变量键声明,具体抓取配置由部署层 ConfigMap/Operator values 管理。

学生不得直接访问 Prometheus。
