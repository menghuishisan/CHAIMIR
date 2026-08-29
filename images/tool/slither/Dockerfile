# 本文件定义 Slither 工具镜像,使用官方 Python 包和 Solidity 官方编译器构建命令工具。
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ARG SOLC_VERSION=0.8.26
ARG SOLC_PATH=solc-linux-amd64-v0.8.26+commit.8a97fa7a
ARG SOLC_SHA256=d5f23436f443edb85d8e76906d12f0a86ce0490e7663a9e608efeb7a93f149ef
ARG PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple

LABEL org.opencontainers.image.title="tool/slither"
LABEL org.opencontainers.image.description="Chaimir Slither security scanner image"

USER root
RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && sed -i 's#https://dl-cdn.alpinelinux.org/alpine#https://mirrors.aliyun.com/alpine#g' /etc/apk/repositories \
    && rm -f /var/cache/apk/* \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk upgrade --cache-dir /var/cache/apk --update-cache && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache bash ca-certificates gcompat git libstdc++ py3-pip python3 wget && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        if wget -q --timeout=300 --tries=3 -O "/usr/local/bin/solc-${SOLC_VERSION}" "https://binaries.soliditylang.org/linux-amd64/${SOLC_PATH}"; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        rm -f "/usr/local/bin/solc-${SOLC_VERSION}"; \
        sleep $((attempt * 3)); \
    done \
    && echo "${SOLC_SHA256}  /usr/local/bin/solc-${SOLC_VERSION}" | sha256sum -c - \
    && chmod 0755 /usr/local/bin/solc-${SOLC_VERSION} \
    && ln -s /usr/local/bin/solc-${SOLC_VERSION} /usr/local/bin/solc \
    && python3 -m venv /opt/slither \
    && /opt/slither/bin/pip install --no-cache-dir --upgrade --index-url "${PIP_INDEX_URL}" \
        "pip>=25.3" \
        "setuptools>=80.0.0" \
        "wheel>=0.46.0" \
        "slither-analyzer==0.11.3" \
        "aiohttp>=3.14.3" \
    && rm -rf /opt/slither/lib/python3.12/site-packages/pip /opt/slither/lib/python3.12/site-packages/pip-*.dist-info \
    && apk del py3-pip \
    && ln -s /opt/slither/bin/slither /usr/local/bin/slither \
    && slither --version >/dev/null \
    && solc --version >/dev/null \
    && addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /workspace -s /sbin/nologin chaimir \
    && mkdir -p /workspace /tmp/slither \
    && chown -R 1000:1000 /workspace /tmp/slither

USER 1000:1000
ENV HOME=/workspace
ENV TMPDIR=/tmp/slither
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/slither/bin
WORKDIR /workspace
CMD ["slither", "--version"]
