# 本文件定义长安链合约测试判题镜像。
FROM chainmakerofficial/chainmaker:v2.3.5

LABEL org.opencontainers.image.title="judger/testcase-chainmaker"
LABEL org.opencontainers.image.description="Chaimir ChainMaker testcase judger image"

USER root
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /judge --create-home chaimir \
    && chown -R chaimir:chaimir /judge

COPY run_chainmaker_tests.sh /usr/local/bin/run-chainmaker-tests
COPY normalize_result.py /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-chainmaker-tests /usr/local/bin/normalize-result

USER chaimir
WORKDIR /judge
ENTRYPOINT ["run-chainmaker-tests"]
