# 本文件定义 Fabric 链码测试判题镜像。
FROM hyperledger/fabric-tools:2.5

LABEL org.opencontainers.image.title="judger/testcase-fabric"
LABEL org.opencontainers.image.description="Chaimir Fabric chaincode testcase judger image"

USER root
RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /judge --create-home --shell /usr/sbin/nologin chaimir \
    && chown -R 1000:1000 /judge

COPY run_fabric_tests.sh /usr/local/bin/run-fabric-tests
COPY normalize_result.py /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-fabric-tests /usr/local/bin/normalize-result

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-fabric-tests"]
