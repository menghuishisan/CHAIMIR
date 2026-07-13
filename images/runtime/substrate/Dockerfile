# 本文件定义 Substrate/Polkadot 开发链运行时镜像,复用 Polkadot SDK 官方节点镜像。
FROM parity/polkadot:stable2603-3@sha256:0bbcce4a417ad8c2c88af08b96bd27113f1fd33427ae4d02b6c64b8c126f1997

LABEL org.opencontainers.image.title="runtime/substrate"
LABEL org.opencontainers.image.description="Chaimir Polkadot SDK development chain runtime"

USER root

# 官方镜像已提供 parity:1000:1000;只升级系统安全包并创建平台声明的写入目录。
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /workspace \
    && mkdir -p /runtime-state/substrate \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-substrate-entrypoint
RUN chmod +x /usr/local/bin/chaimir-substrate-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 9944
ENTRYPOINT ["/usr/local/bin/chaimir-substrate-entrypoint"]
