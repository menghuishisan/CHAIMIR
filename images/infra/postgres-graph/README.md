# infra/postgres-graph

本镜像为 The Graph 索引场景提供隔离 PostgreSQL 实例。`POSTGRES_DB`、`POSTGRES_USER` 和 Secret 注入的 `POSTGRES_PASSWORD` 由组合配置提供,不写入镜像。生产部署可按学校资源策略复用其他 `postgres-compatible` 提供者,但镜像层保留独立配套镜像用于沙箱实验。
