# tool/db-viewer

本镜像提供状态数据库可视化工具,用于 Fabric CouchDB 或实验内 PostgreSQL 状态查看。

`manifest.yaml` 已声明受控 PostgreSQL 数据源、固定 pgweb 连接 URL、启动重试、Service 与最小 NetworkPolicy。学生只能通过 M2 平台代理访问 Web UI,不能手填连接串,也不能直连平台数据库、其他租户数据库或 Secret。后续 Fabric CouchDB 等数据源必须继续通过 WorkloadSpec 显式声明和后端注入,不得在前端开放任意连接入口。
