# 本文件定义 Redis 官方镜像的安全薄封装,只升级系统包并固定非 root 运行。
FROM redis:8-alpine

LABEL org.opencontainers.image.title="middleware/redis"
LABEL org.opencontainers.image.description="Chaimir Redis middleware thin wrapper"

USER root
RUN apk upgrade --no-cache \
    && chown -R redis:redis /data

USER redis
WORKDIR /data
EXPOSE 6379
