# 本文件定义跨链桥中继器镜像,薄封装 Hyperlane 官方 relayer agent。
FROM ghcr.io/hyperlane-xyz/hyperlane-agent@sha256:ed2a0dcc7ebe7accd2bec88f82ce196bb455f4cf9a4d84d7a9192052b1911551

LABEL org.opencontainers.image.title="infra/bridge-relayer"
LABEL org.opencontainers.image.description="Chaimir Hyperlane bridge relayer image"

USER root
RUN apt-get update \
    && apt-get upgrade -y --no-install-recommends \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /workspace /runtime-state /hyperlane-config \
    && chown -R 1000:1000 /workspace /runtime-state /hyperlane-config \
    && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /usr/local/bin/chaimir-bridge-relayer-entrypoint
RUN chmod +x /usr/local/bin/chaimir-bridge-relayer-entrypoint

ENV HOME=/workspace
WORKDIR /workspace
USER 1000:1000
ENTRYPOINT ["/usr/local/bin/chaimir-bridge-relayer-entrypoint"]
