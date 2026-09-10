# 本文件定义 Redis 官方镜像的安全薄封装,只升级系统包并固定非 root 运行。
FROM redis:8-alpine@sha256:9d317178eceac8454a2284a9e6df2466b93c745529947f0cd42a0fa9609d7005

LABEL org.opencontainers.image.title="middleware/redis"
LABEL org.opencontainers.image.description="Chaimir Redis middleware thin wrapper"

USER root
RUN chown -R redis:redis /data

COPY redis-entrypoint.sh /usr/local/bin/chaimir-redis-entrypoint
COPY redis-probe.sh /usr/local/bin/chaimir-redis-probe
RUN chmod 0555 /usr/local/bin/chaimir-redis-entrypoint /usr/local/bin/chaimir-redis-probe

USER redis
WORKDIR /data
EXPOSE 6379
ENTRYPOINT ["/usr/local/bin/chaimir-redis-entrypoint"]
