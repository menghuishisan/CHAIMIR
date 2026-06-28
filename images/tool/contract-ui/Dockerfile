# 本文件构建合约交互面板工具镜像。
FROM cgr.dev/chainguard/wolfi-base:latest

LABEL org.opencontainers.image.title="tool/contract-ui"
LABEL org.opencontainers.image.description="Chaimir contract interaction panel image"

RUN apk add --no-cache python-3.13 py3.13-pip \
    && python3.13 -m venv /opt/contract-ui \
    && /opt/contract-ui/bin/pip install --no-cache-dir "eth-abi==5.2.0" "eth-utils==5.3.1" "eth-hash[pycryptodome]==0.8.0" \
    && addgroup -g 1000 chaimir \
    && adduser -D -u 1000 -G chaimir -h /app -s /sbin/nologin chaimir

WORKDIR /app
COPY contract_ui.py /app/contract_ui.py

USER 1000:1000
ENV PATH=/opt/contract-ui/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
EXPOSE 8080
CMD ["python3.13", "/app/contract_ui.py"]
