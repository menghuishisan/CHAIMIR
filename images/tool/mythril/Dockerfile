# 本文件定义 Mythril 符号执行安全分析工具镜像。
FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795 AS build

ARG SOLC_VERSION=0.8.26
ARG SOLC_PATH=solc-linux-amd64-v0.8.26+commit.8a97fa7a
ARG SOLC_SHA256=d5f23436f443edb85d8e76906d12f0a86ce0490e7663a9e608efeb7a93f149ef

RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    packages='ca-certificates gcc git libstdc++ make python-3.10 python-3.10-dev py3.10-pip' \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache ${packages} && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && python3.10 -m venv /opt/mythril \
    && /opt/mythril/bin/pip install --no-cache-dir --upgrade pip \
    && /opt/mythril/bin/pip install --no-cache-dir \
        "mythril==0.24.8" \
        "setuptools==80.9.0" \
        "urllib3>=2.6.0" \
        "cryptography>=46.0.0" \
        "requests>=2.32.5" \
    && rm -rf \
        /opt/mythril/lib/python3.10/site-packages/setuptools/_vendor/wheel \
        /opt/mythril/lib/python3.10/site-packages/setuptools/_vendor/wheel-*.dist-info \
        /opt/mythril/lib/python3.10/site-packages/setuptools/_vendor/jaraco.context-*.dist-info \
    && /opt/mythril/bin/myth version

FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795

ARG SOLC_VERSION=0.8.26
ARG SOLC_PATH=solc-linux-amd64-v0.8.26+commit.8a97fa7a
ARG SOLC_SHA256=d5f23436f443edb85d8e76906d12f0a86ce0490e7663a9e608efeb7a93f149ef

LABEL org.opencontainers.image.title="tool/mythril"
LABEL org.opencontainers.image.description="Chaimir Mythril symbolic execution tool image"

ADD https://binaries.soliditylang.org/linux-amd64/${SOLC_PATH} /usr/local/bin/solc-${SOLC_VERSION}
RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    packages='ca-certificates libstdc++ python-3.10' \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache ${packages} && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done
COPY --from=build /opt/mythril /opt/mythril
RUN addgroup -g 1000 chaimir \
    && adduser -D -u 1000 -G chaimir -h /workspace -s /sbin/nologin chaimir \
    && mkdir -p /usr/local/bin /opt/solcx \
    && echo "${SOLC_SHA256}  /usr/local/bin/solc-${SOLC_VERSION}" | sha256sum -c - \
    && chmod 0755 /usr/local/bin/solc-${SOLC_VERSION} \
    && ln -sf /usr/local/bin/solc-${SOLC_VERSION} /usr/local/bin/solc \
    && ln -sf /opt/mythril/bin/myth /usr/local/bin/myth \
    && SOLCX_BINARY_PATH=/opt/solcx /opt/mythril/bin/python -c "import solcx; solcx.import_installed_solc()" \
    && chown -R 1000:1000 /workspace \
    && chmod -R a+rX /opt/solcx \
    && solc --version >/dev/null \
    && myth version

USER 1000:1000
ENV HOME=/workspace
ENV SOLCX_BINARY_PATH=/opt/solcx
ENV MPLCONFIGDIR=/tmp/matplotlib
ENV XDG_CONFIG_HOME=/tmp
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/mythril/bin
WORKDIR /workspace
ENTRYPOINT ["/opt/mythril/bin/myth"]
