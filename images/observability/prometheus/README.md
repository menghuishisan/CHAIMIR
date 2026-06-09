# observability/prometheus

Prometheus 使用官方上游固定镜像,不重打包。监控入口配置统一通过 `deploy/config/chaimir.env` 的环境变量键声明,具体抓取配置由部署层 ConfigMap/Operator values 管理。

学生不得直接访问 Prometheus。
