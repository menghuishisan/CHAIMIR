# 本文件定义 Fabric 工具运行时镜像,从官方 tools 镜像提取必要二进制并放入干净运行层。
FROM hyperledger/fabric-tools:2.5.16 AS fabric-tools

FROM ubuntu:22.04

LABEL org.opencontainers.image.title="runtime/fabric"
LABEL org.opencontainers.image.description="Chaimir Hyperledger Fabric runtime tools image"

USER root

# 安装最小运行依赖并创建平台工作区用户;默认 MSP 私钥不进入最终镜像层。
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && apt-get -o Acquire::Retries=3 -y install --no-install-recommends ca-certificates bash \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state \
    && chown -R 1000:1000 /workspace /runtime-state

COPY --from=fabric-tools /usr/local/bin/peer /usr/local/bin/peer
COPY --from=fabric-tools /usr/local/bin/configtxgen /usr/local/bin/configtxgen
COPY --from=fabric-tools /usr/local/bin/configtxlator /usr/local/bin/configtxlator
COPY --from=fabric-tools /usr/local/bin/cryptogen /usr/local/bin/cryptogen
COPY --from=fabric-tools /usr/local/bin/discover /usr/local/bin/discover
COPY --from=fabric-tools /usr/local/bin/ledgerutil /usr/local/bin/ledgerutil
COPY --from=fabric-tools /usr/local/bin/osnadmin /usr/local/bin/osnadmin
COPY --from=fabric-tools /etc/hyperledger/fabric/core.yaml /etc/hyperledger/fabric/core.yaml
COPY --from=fabric-tools /etc/hyperledger/fabric/configtx.yaml /etc/hyperledger/fabric/configtx.yaml
COPY --from=fabric-tools /etc/hyperledger/fabric/orderer.yaml /etc/hyperledger/fabric/orderer.yaml
COPY entrypoint.sh /usr/local/bin/chaimir-fabric-entrypoint
RUN chmod 0755 /usr/local/bin/peer /usr/local/bin/configtxgen /usr/local/bin/configtxlator /usr/local/bin/cryptogen /usr/local/bin/discover /usr/local/bin/ledgerutil /usr/local/bin/osnadmin /usr/local/bin/chaimir-fabric-entrypoint

ENV FABRIC_CFG_PATH=/etc/hyperledger/fabric
USER 1000:1000
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/chaimir-fabric-entrypoint"]
