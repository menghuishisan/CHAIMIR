# 本文件定义跨链桥中继器镜像,提供 Wormhole/IBC 等成熟中继工具链。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="infra/bridge-relayer"
LABEL org.opencontainers.image.description="Chaimir bridge relayer toolchain image"

RUN useradd --system --create-home --home-dir /workspace chaimir \
    && npm install -g @certusone/wormhole-sdk@0.10.18 \
    && chown -R chaimir:chaimir /workspace

USER chaimir
WORKDIR /workspace
CMD ["node", "-e", "import('@certusone/wormhole-sdk').then(() => console.log('wormhole-sdk ready'))"]
