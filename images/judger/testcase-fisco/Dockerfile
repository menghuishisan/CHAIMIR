# 本文件定义 FISCO BCOS 合约测试判题镜像。
ARG JUDGE_MIN_IMAGE=invalid.invalid/base/judge-min:required

FROM fiscoorg/fiscobcos:v3.12.1@sha256:93db5277a5c896fde402f2165d133cc3d346effef4165ec417ef098eb2ede74b AS fisco-upstream

FROM ${JUDGE_MIN_IMAGE}

LABEL org.opencontainers.image.title="judger/testcase-fisco"
LABEL org.opencontainers.image.description="Chaimir FISCO BCOS testcase judger image"

USER root
RUN mkdir -p /judge \
    && chown -R 1000:1000 /judge

COPY run_fisco_tests.sh /usr/local/bin/run-fisco-tests
COPY --from=fisco-upstream /usr/local/bin/fisco-bcos /usr/local/bin/fisco-bcos
RUN chmod +x /usr/local/bin/run-fisco-tests /usr/local/bin/fisco-bcos

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-fisco-tests"]
