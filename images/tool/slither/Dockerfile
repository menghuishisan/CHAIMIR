# 本文件定义 Slither 工具镜像,使用官方 Python 包和 Solidity 官方编译器构建命令工具。
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ARG SOLC_0_8_20_PATH=solc-linux-amd64-v0.8.20+commit.a1b79de6
ARG SOLC_0_8_20_SHA256=0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782
ARG SOLC_0_8_24_PATH=solc-linux-amd64-v0.8.24+commit.e11b9ed9
ARG SOLC_0_8_24_SHA256=fb03a29a517452b9f12bcf459ef37d0a543765bb3bbc911e70a87d6a37c30d5f
ARG SOLC_0_8_26_PATH=solc-linux-amd64-v0.8.26+commit.8a97fa7a
ARG SOLC_0_8_26_SHA256=d5f23436f443edb85d8e76906d12f0a86ce0490e7663a9e608efeb7a93f149ef
ARG PIP_INDEX_URL=https://pypi.org/simple

LABEL org.opencontainers.image.title="tool/slither"
LABEL org.opencontainers.image.description="Chaimir Slither security scanner image"

COPY chaimir_slither.py /usr/local/bin/chaimir-slither

USER root
RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && rm -f /var/cache/apk/* \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache bash ca-certificates gcompat git libstdc++ py3-pip python3 wget && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && for spec in \
        "0.8.20|${SOLC_0_8_20_PATH}|${SOLC_0_8_20_SHA256}" \
        "0.8.24|${SOLC_0_8_24_PATH}|${SOLC_0_8_24_SHA256}" \
        "0.8.26|${SOLC_0_8_26_PATH}|${SOLC_0_8_26_SHA256}"; do \
        version="${spec%%|*}"; rest="${spec#*|}"; path="${rest%%|*}"; sha="${rest#*|}"; \
        for attempt in 1 2 3 4 5 6 7 8; do \
            if wget -q --timeout=300 --tries=3 -O "/usr/local/bin/solc-${version}" "https://binaries.soliditylang.org/linux-amd64/${path}"; then break; fi; \
            if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
            rm -f "/usr/local/bin/solc-${version}"; \
            sleep $((attempt * 3)); \
        done; \
        echo "${sha}  /usr/local/bin/solc-${version}" | sha256sum -c -; \
        chmod 0755 "/usr/local/bin/solc-${version}"; \
    done \
    && python3 -m venv /opt/slither \
    && /opt/slither/bin/pip install --no-cache-dir --index-url "${PIP_INDEX_URL}" \
        "pip==25.3" \
        "setuptools==80.0.0" \
        "wheel==0.46.0" \
        "slither-analyzer==0.11.3" \
        "aiohttp==3.14.3" \
    && rm -rf /opt/slither/lib/python3.12/site-packages/pip /opt/slither/lib/python3.12/site-packages/pip-*.dist-info \
    && apk del py3-pip \
    && /opt/slither/bin/slither --version >/dev/null \
    && for version in 0.8.20 0.8.24 0.8.26; do \
        "/usr/local/bin/solc-${version}" --version >/dev/null; \
    done \
    && chmod 0755 /usr/local/bin/chaimir-slither \
    && addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /workspace -s /sbin/nologin chaimir \
    && mkdir -p /workspace /tmp/slither \
    && chown -R 1000:1000 /workspace /tmp/slither

USER 1000:1000
ENV HOME=/workspace
ENV TMPDIR=/tmp/slither
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/slither/bin
WORKDIR /workspace
CMD ["chaimir-slither"]
