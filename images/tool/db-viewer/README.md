# tool/db-viewer

本镜像提供状态数据库可视化工具,用于 Fabric CouchDB 或实验内 PostgreSQL 状态查看。

`manifest.yaml` 只声明 pgweb 进程及一个 `postgres-compatible` 数据绑定。数据库由组合编译器按能力解析并独立展开，Service 地址通过 `CHAIMIR_DATABASE_ADDRESS` 注入,数据库用户、名称和密码由组合参数与 Secret 注入,入口再生成 pgweb 所需连接 URL；镜像不保存固定 Service 名。学生只能通过 M2 平台代理访问 Web UI,不能手填连接串,也不能直连平台数据库、其他租户数据库或 Secret。新增可用数据库组件必须提供同一能力和 `postgres` 端点,不得在前端开放任意连接入口。
