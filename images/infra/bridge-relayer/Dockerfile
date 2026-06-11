# 本文件定义跨链桥中继器镜像,提供 Wormhole/IBC 等成熟中继工具链。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="infra/bridge-relayer"
LABEL org.opencontainers.image.description="Chaimir bridge relayer toolchain image"

WORKDIR /opt/chaimir/bridge-relayer
COPY package.json package-lock.json /opt/chaimir/bridge-relayer/
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && mkdir -p /workspace \
    && chown -R 1000:1000 /workspace /opt/chaimir/bridge-relayer

COPY entrypoint.sh /usr/local/bin/chaimir-bridge-relayer-entrypoint
RUN chmod +x /usr/local/bin/chaimir-bridge-relayer-entrypoint

ENV HOME=/workspace
USER 1000:1000
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/chaimir-bridge-relayer-entrypoint"]
