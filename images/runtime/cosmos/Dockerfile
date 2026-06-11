# 本文件定义 Cosmos SDK/Tendermint 单链运行时镜像。
FROM ghcr.io/cosmos/gaia:v21.0.1

LABEL org.opencontainers.image.title="runtime/cosmos"
LABEL org.opencontainers.image.description="Chaimir Cosmos SDK local chain runtime"

USER root

# 创建固定 UID/GID 的沙箱用户,运行时仍由只读 rootfs 和 runtime-state 卷约束写入面。
RUN addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /workspace chaimir \
    && mkdir -p /runtime-state/cosmos \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-cosmos-entrypoint
RUN chmod +x /usr/local/bin/chaimir-cosmos-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 26657 1317
ENTRYPOINT ["/usr/local/bin/chaimir-cosmos-entrypoint"]
