# 本文件构建钱包模拟器工具镜像。
FROM python:3.13-alpine

LABEL org.opencontainers.image.title="tool/wallet-sim"
LABEL org.opencontainers.image.description="Chaimir wallet simulator tool image"

RUN addgroup -S chaimir \
    && adduser -S -G chaimir -h /app chaimir

WORKDIR /app
COPY wallet_sim.py /app/wallet_sim.py

USER chaimir
EXPOSE 8080
CMD ["python", "/app/wallet_sim.py"]
