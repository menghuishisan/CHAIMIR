# 本文件定义长安链合约测试判题镜像。
FROM chainmakerofficial/chainmaker:v2.3.5

LABEL org.opencontainers.image.title="judger/testcase-chainmaker"
LABEL org.opencontainers.image.description="Chaimir ChainMaker testcase judger image"

USER root
RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /judge --create-home --shell /usr/sbin/nologin chaimir \
    && chown -R 1000:1000 /judge

COPY run_chainmaker_tests.sh /usr/local/bin/run-chainmaker-tests
COPY normalize_result.py /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-chainmaker-tests /usr/local/bin/normalize-result

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-chainmaker-tests"]
