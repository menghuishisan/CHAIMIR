# 本文件定义 FISCO BCOS 合约测试判题镜像。
FROM chaimir/base/judge-min:dev AS judge-base
FROM fiscoorg/fiscobcos:v3.12.1 AS fisco-upstream

FROM debian:12-slim

LABEL org.opencontainers.image.title="judger/testcase-fisco"
LABEL org.opencontainers.image.description="Chaimir FISCO BCOS testcase judger image"

USER root
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /judge --create-home --shell /usr/sbin/nologin chaimir \
    && chown -R 1000:1000 /judge

COPY run_fisco_tests.sh /usr/local/bin/run-fisco-tests
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
COPY --from=fisco-upstream /usr/local/bin/fisco-bcos /usr/local/bin/fisco-bcos
RUN chmod +x /usr/local/bin/run-fisco-tests /usr/local/bin/normalize-result

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-fisco-tests"]
