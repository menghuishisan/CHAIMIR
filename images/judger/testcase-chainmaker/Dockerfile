# 本文件定义长安链合约测试判题镜像,从可校验源码归档重建 ChainMaker 工具。
ARG GO_BUILDER_IMAGE=chaimir/base/go-builder:dev
ARG RUNTIME_IMAGE=debian:12-slim
ARG CHAINMAKER_SOURCE_ARCHIVE=images/vendor/chainmaker/chainmaker-go-v2.3.8.tar.gz
ARG CHAINMAKER_SOURCE_SHA256=
ARG CHAINMAKER_VERSION=v2.3.8
ARG CHAINMAKER_GIT_BRANCH=source-archive
ARG CHAINMAKER_GIT_COMMIT=source-archive

FROM ${GO_BUILDER_IMAGE} AS chainmaker-build
FROM chaimir/base/judge-min:dev AS judge-base

ARG CHAINMAKER_SOURCE_ARCHIVE
ARG CHAINMAKER_SOURCE_SHA256
ARG CHAINMAKER_VERSION
ARG CHAINMAKER_GIT_BRANCH
ARG CHAINMAKER_GIT_COMMIT

USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /src/chainmaker-go /out \
    && chown -R 1000:1000 /src/chainmaker-go /out
USER 1000:1000
COPY --chown=1000:1000 ${CHAINMAKER_SOURCE_ARCHIVE} /tmp/chainmaker-go-source.tar.gz
RUN test -n "${CHAINMAKER_SOURCE_SHA256}" \
    && echo "${CHAINMAKER_SOURCE_SHA256}  /tmp/chainmaker-go-source.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/chainmaker-go-source.tar.gz -C /src/chainmaker-go --strip-components=1 \
    && cd /src/chainmaker-go/main \
    && go mod download \
    && go build -trimpath -ldflags="-s -w -X chainmaker.org/chainmaker-go/module/blockchain.CurrentVersion=${CHAINMAKER_VERSION} -X chainmaker.org/chainmaker-go/module/blockchain.BuildDateTime=source-archive -X chainmaker.org/chainmaker-go/module/blockchain.GitBranch=${CHAINMAKER_GIT_BRANCH} -X chainmaker.org/chainmaker-go/module/blockchain.GitCommit=${CHAINMAKER_GIT_COMMIT}" -o /out/chainmaker \
    && cp /src/chainmaker-go/main/libwasmer_runtime_c_api.so /out/libwasmer.so \
    && cp /src/chainmaker-go/main/prebuilt/linux/wxdec /out/wxdec

FROM ${RUNTIME_IMAGE}

LABEL org.opencontainers.image.title="judger/testcase-chainmaker"
LABEL org.opencontainers.image.description="Chaimir ChainMaker testcase judger image"

USER root
COPY --from=chainmaker-build /out/chainmaker /chainmaker-go/bin/chainmaker
COPY --from=chainmaker-build /out/libwasmer.so /lib/libwasmer.so
COPY --from=chainmaker-build /out/wxdec /usr/bin/wxdec
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
RUN apt-get update \
    && apt-get upgrade -y --no-install-recommends \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /judge --create-home --shell /usr/sbin/nologin chaimir \
    && chmod 0755 /chainmaker-go/bin/chainmaker /lib/libwasmer.so /usr/bin/wxdec \
    && chown -R 1000:1000 /judge

COPY images/judger/testcase-chainmaker/run_chainmaker_tests.sh /usr/local/bin/run-chainmaker-tests
RUN chmod +x /usr/local/bin/run-chainmaker-tests /usr/local/bin/normalize-result

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-chainmaker-tests"]
