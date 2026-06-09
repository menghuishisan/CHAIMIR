# 本文件定义 The Graph 专用 PostgreSQL 镜像,复用官方 PostgreSQL。
FROM postgres:16-alpine

LABEL org.opencontainers.image.title="infra/postgres-graph"
LABEL org.opencontainers.image.description="Chaimir PostgreSQL instance for The Graph indexing"

# 官方 postgres 镜像已使用非 root postgres 用户运行数据库进程。
USER postgres
EXPOSE 5432
