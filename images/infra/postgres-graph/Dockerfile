# 本文件定义 The Graph 专用 PostgreSQL 镜像,复用官方 PostgreSQL。
FROM postgres:16-alpine

LABEL org.opencontainers.image.title="infra/postgres-graph"
LABEL org.opencontainers.image.description="Chaimir PostgreSQL instance for The Graph indexing"

# The Graph 索引库运行在沙箱协同容器中,需满足 M2 统一 1000:1000 身份契约。
RUN mkdir -p /var/lib/postgresql/data /var/run/postgresql /workspace \
    && chown -R 1000:1000 /var/lib/postgresql /var/run/postgresql /workspace

USER 1000:1000
EXPOSE 5432
