# 本文件定义 Foundry/Anvil 运行时镜像,复用成熟官方 Foundry 镜像能力。
FROM ghcr.io/foundry-rs/foundry:v1.0.0@sha256:d12a373ec950de170d5461014ef9320ba0fb6e0db6f87835999d0fcf3820370e

LABEL org.opencontainers.image.title="runtime/evm-foundry"
LABEL org.opencontainers.image.description="Chaimir Foundry and Anvil EVM runtime"

USER root
ARG SOLC_0_8_20_SHA256=0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782
ADD https://github.com/ethereum/solidity/releases/download/v0.8.20/solc-static-linux /tmp/solc-0.8.20

# 复用上游 foundry:1000:1000,缓存写入 runtime-state 而不是学生 workspace。
# 固定内置 solc 0.8.20,避免实验运行时联网下载编译器。
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && apt-get -o Acquire::Retries=3 install -y --no-install-recommends jq ca-certificates \
    && echo "${SOLC_0_8_20_SHA256}  /tmp/solc-0.8.20" | sha256sum -c - \
    && install -m 0755 /tmp/solc-0.8.20 /usr/local/bin/solc-0.8.20 \
    && ln -sf /usr/local/bin/solc-0.8.20 /usr/local/bin/solc \
    && rm -f /tmp/solc-0.8.20 \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /workspace /runtime-state/foundry-home \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-foundry-entrypoint
COPY chaimir-workspace /usr/local/bin/chaimir-workspace
COPY chaimir-chain /usr/local/bin/chaimir-chain
RUN chmod +x /usr/local/bin/chaimir-foundry-entrypoint /usr/local/bin/chaimir-workspace /usr/local/bin/chaimir-chain

ENV HOME=/runtime-state/foundry-home
USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-foundry-entrypoint"]
