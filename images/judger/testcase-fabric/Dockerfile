# 本文件定义 Fabric 链码测试判题镜像。
ARG JUDGE_MIN_IMAGE=invalid.invalid/base/judge-min:required
ARG FABRIC_TOOLS_IMAGE=invalid.invalid/base/fabric-tools:required

FROM ${JUDGE_MIN_IMAGE} AS judge-base

FROM ${FABRIC_TOOLS_IMAGE} AS fabric-tools

FROM ubuntu:22.04@sha256:0e0a0fc6d18feda9db1590da249ac93e8d5abfea8f4c3c0c849ce512b5ef8982

LABEL org.opencontainers.image.title="judger/testcase-fabric"
LABEL org.opencontainers.image.description="Chaimir Fabric chaincode testcase judger image"

USER root
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && apt-get -o Acquire::Retries=3 -y install --no-install-recommends ca-certificates bash \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /judge --create-home --shell /usr/sbin/nologin chaimir \
    && chown -R 1000:1000 /judge

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
COPY run_fabric_tests.sh /usr/local/bin/run-fabric-tests
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
RUN chmod 0755 /usr/local/bin/peer /usr/local/bin/configtxgen /usr/local/bin/configtxlator /usr/local/bin/cryptogen /usr/local/bin/discover /usr/local/bin/ledgerutil /usr/local/bin/osnadmin /usr/local/bin/run-fabric-tests /usr/local/bin/normalize-result

ENV FABRIC_CFG_PATH=/etc/hyperledger/fabric
USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-fabric-tests"]
