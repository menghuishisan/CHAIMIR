# 本文件定义 PostgreSQL 官方镜像的安全薄封装,移除不再需要的 gosu 并固定非 root 运行。
FROM postgres:18-alpine@sha256:9a8afca54e7861fd90fab5fdf4c42477a6b1cb7d293595148e674e0a3181de15

LABEL org.opencontainers.image.title="middleware/postgres"
LABEL org.opencontainers.image.description="Chaimir PostgreSQL middleware thin wrapper"

USER root
RUN apk upgrade --no-cache \
    && rm -f /usr/local/bin/gosu \
    && mkdir -p /var/lib/postgresql/data /var/run/postgresql \
    && chown -R postgres:postgres /var/lib/postgresql /var/run/postgresql

USER postgres
WORKDIR /var/lib/postgresql
EXPOSE 5432
