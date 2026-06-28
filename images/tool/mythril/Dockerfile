# 本文件定义 Mythril 符号执行安全分析工具镜像。
FROM cgr.dev/chainguard/wolfi-base:latest AS build

RUN apk add --no-cache \
        ca-certificates \
        gcc \
        git \
        libstdc++ \
        make \
        python-3.10 \
        python-3.10-dev \
        py3.10-pip \
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

FROM cgr.dev/chainguard/wolfi-base:latest

LABEL org.opencontainers.image.title="tool/mythril"
LABEL org.opencontainers.image.description="Chaimir Mythril symbolic execution tool image"

RUN apk add --no-cache ca-certificates libstdc++ python-3.10
COPY --from=build /opt/mythril /opt/mythril
RUN addgroup -g 1000 chaimir \
    && adduser -D -u 1000 -G chaimir -h /workspace -s /sbin/nologin chaimir \
    && chown -R 1000:1000 /workspace \
    && /opt/mythril/bin/myth version

USER 1000:1000
WORKDIR /workspace
ENTRYPOINT ["/opt/mythril/bin/myth"]
