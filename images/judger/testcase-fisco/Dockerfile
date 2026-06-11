# 本文件定义 FISCO BCOS 合约测试判题镜像。
FROM fiscoorg/fiscobcos:v3.12.1

LABEL org.opencontainers.image.title="judger/testcase-fisco"
LABEL org.opencontainers.image.description="Chaimir FISCO BCOS testcase judger image"

USER root
RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /judge --create-home --shell /usr/sbin/nologin chaimir \
    && chown -R 1000:1000 /judge

COPY run_fisco_tests.sh /usr/local/bin/run-fisco-tests
COPY normalize_result.py /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-fisco-tests /usr/local/bin/normalize-result

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-fisco-tests"]
