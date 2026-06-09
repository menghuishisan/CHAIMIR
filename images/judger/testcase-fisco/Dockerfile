# 本文件定义 FISCO BCOS 合约测试判题镜像。
FROM fiscoorg/fiscobcos:v3.11.0

LABEL org.opencontainers.image.title="judger/testcase-fisco"
LABEL org.opencontainers.image.description="Chaimir FISCO BCOS testcase judger image"

USER root
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /judge --create-home chaimir \
    && chown -R chaimir:chaimir /judge

COPY run_fisco_tests.sh /usr/local/bin/run-fisco-tests
COPY normalize_result.py /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-fisco-tests /usr/local/bin/normalize-result

USER chaimir
WORKDIR /judge
ENTRYPOINT ["run-fisco-tests"]
