# 本文件定义 Foundry/Anvil 运行时镜像,复用成熟官方 Foundry 镜像能力。
FROM ghcr.io/foundry-rs/foundry:v1.7.1@sha256:bdc584033cefab8be282c7f57c263cda42f04700cd1ec88d64bebe2f645aba72

LABEL org.opencontainers.image.title="runtime/evm-foundry"
LABEL org.opencontainers.image.description="Chaimir Foundry and Anvil EVM runtime"

USER root
ARG SOLC_0_8_20_SHA256=0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782
ARG SOLC_0_8_24_SHA256=fb03a29a517452b9f12bcf459ef37d0a543765bb3bbc911e70a87d6a37c30d5f
ARG SOLC_0_8_26_SHA256=d5f23436f443edb85d8e76906d12f0a86ce0490e7663a9e608efeb7a93f149ef
ADD --checksum=sha256:0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782 https://github.com/ethereum/solidity/releases/download/v0.8.20/solc-static-linux /tmp/solc-0.8.20
ADD --checksum=sha256:fb03a29a517452b9f12bcf459ef37d0a543765bb3bbc911e70a87d6a37c30d5f https://github.com/ethereum/solidity/releases/download/v0.8.24/solc-static-linux /tmp/solc-0.8.24
ADD --checksum=sha256:d5f23436f443edb85d8e76906d12f0a86ce0490e7663a9e608efeb7a93f149ef https://github.com/ethereum/solidity/releases/download/v0.8.26/solc-static-linux /tmp/solc-0.8.26

# 复用上游 foundry:1000:1000,缓存写入 runtime-state 而不是学生 workspace。
# 内置多个经校验的 Solidity 编译器,实验按精确版本路径选择且不允许运行时联网下载。
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 install -y --no-install-recommends jq ca-certificates \
    && echo "${SOLC_0_8_20_SHA256}  /tmp/solc-0.8.20" | sha256sum -c - \
    && echo "${SOLC_0_8_24_SHA256}  /tmp/solc-0.8.24" | sha256sum -c - \
    && echo "${SOLC_0_8_26_SHA256}  /tmp/solc-0.8.26" | sha256sum -c - \
    && install -m 0755 /tmp/solc-0.8.20 /usr/local/bin/solc-0.8.20 \
    && install -m 0755 /tmp/solc-0.8.24 /usr/local/bin/solc-0.8.24 \
    && install -m 0755 /tmp/solc-0.8.26 /usr/local/bin/solc-0.8.26 \
    && rm -f /tmp/solc-0.8.20 /tmp/solc-0.8.24 /tmp/solc-0.8.26 \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /workspace /runtime-state/foundry-home \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-foundry-entrypoint
COPY chaimir-foundry-adapter /usr/local/bin/chaimir-foundry-adapter
COPY genesis-state.json /usr/local/share/chaimir/foundry/genesis-state.json
RUN chmod +x /usr/local/bin/chaimir-foundry-entrypoint /usr/local/bin/chaimir-foundry-adapter

ENV HOME=/runtime-state/foundry-home
USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-foundry-entrypoint"]
