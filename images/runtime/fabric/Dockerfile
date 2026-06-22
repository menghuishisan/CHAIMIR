# 本文件定义 Fabric 工具运行时镜像,复用 Hyperledger Fabric 官方 tools 镜像。
FROM hyperledger/fabric-tools:2.5.16

LABEL org.opencontainers.image.title="runtime/fabric"
LABEL org.opencontainers.image.description="Chaimir Hyperledger Fabric runtime tools image"

USER root

# 创建平台工作区用户;Fabric 官方工具仍由镜像提供,不在这里自研。
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-fabric-entrypoint
RUN chmod +x /usr/local/bin/chaimir-fabric-entrypoint

USER 1000:1000
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/chaimir-fabric-entrypoint"]
