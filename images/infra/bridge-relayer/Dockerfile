# 本文件定义跨链桥中继器镜像,提供 Wormhole/IBC 等成熟中继工具链。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="infra/bridge-relayer"
LABEL org.opencontainers.image.description="Chaimir bridge relayer toolchain image"

WORKDIR /opt/chaimir/bridge-relayer
COPY package.json package-lock.json /opt/chaimir/bridge-relayer/
RUN useradd --system --create-home --home-dir /workspace chaimir \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && chown -R chaimir:chaimir /workspace

COPY entrypoint.sh /usr/local/bin/chaimir-bridge-relayer-entrypoint
RUN chmod +x /usr/local/bin/chaimir-bridge-relayer-entrypoint

USER chaimir
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/chaimir-bridge-relayer-entrypoint"]
