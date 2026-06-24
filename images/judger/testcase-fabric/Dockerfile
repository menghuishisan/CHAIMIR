# 本文件定义 Fabric 链码测试判题镜像。
ARG JUDGE_MIN_IMAGE

FROM ${JUDGE_MIN_IMAGE} AS judge-base

FROM hyperledger/fabric-tools:2.5.16

LABEL org.opencontainers.image.title="judger/testcase-fabric"
LABEL org.opencontainers.image.description="Chaimir Fabric chaincode testcase judger image"

USER root
RUN if command -v apt-get >/dev/null 2>&1; then apt-get -o Acquire::Retries=3 update && apt-get -o Acquire::Retries=3 -y upgrade && rm -rf /var/lib/apt/lists/*; fi \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /judge --create-home --shell /usr/sbin/nologin chaimir \
    && chown -R 1000:1000 /judge

COPY run_fabric_tests.sh /usr/local/bin/run-fabric-tests
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-fabric-tests /usr/local/bin/normalize-result

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-fabric-tests"]
