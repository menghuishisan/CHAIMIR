# 本文件构建钱包模拟器工具镜像。
FROM cgr.dev/chainguard/wolfi-base:latest

LABEL org.opencontainers.image.title="tool/wallet-sim"
LABEL org.opencontainers.image.description="Chaimir wallet simulator tool image"

RUN apk add --no-cache python-3.13 py3.13-pip \
    && python3.13 -m venv /opt/wallet-sim \
    && /opt/wallet-sim/bin/pip install --no-cache-dir "eth-account==0.13.7" \
    && addgroup -g 1000 chaimir \
    && adduser -D -u 1000 -G chaimir -h /app -s /sbin/nologin chaimir

WORKDIR /app
COPY wallet_sim.py /app/wallet_sim.py

USER 1000:1000
ENV PATH=/opt/wallet-sim/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
EXPOSE 8080
CMD ["python3.13", "/app/wallet_sim.py"]
