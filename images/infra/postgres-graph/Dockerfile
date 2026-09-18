# 本文件定义 The Graph 专用 PostgreSQL 镜像,复用官方 PostgreSQL。
FROM postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777

LABEL org.opencontainers.image.title="infra/postgres-graph"
LABEL org.opencontainers.image.description="Chaimir PostgreSQL instance for The Graph indexing"

# 固定安装 Trivy 已确认的 Alpine 修复包,避免把上游基础层已公开的 HIGH 漏洞带入候选镜像。
RUN apk add --no-cache \
    libcrypto3=3.5.8-r0 \
    libssl3=3.5.8-r0 \
    libuuid=2.42.3-r1

# The Graph 索引库运行在沙箱协同容器中,需满足平台统一的 1000:1000 身份契约。
RUN mkdir -p /var/lib/postgresql/data /var/run/postgresql /runtime-state/postgres /workspace \
    && chown -R 1000:1000 /var/lib/postgresql /var/run/postgresql /workspace \
    && chown -R 1000:1000 /runtime-state/postgres \
    && rm -f /usr/local/bin/gosu

USER 1000:1000
EXPOSE 5432
