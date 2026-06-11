# 本文件构建钱包模拟器工具镜像。
FROM python:3.13-alpine

LABEL org.opencontainers.image.title="tool/wallet-sim"
LABEL org.opencontainers.image.description="Chaimir wallet simulator tool image"

RUN addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /app chaimir

WORKDIR /app
COPY wallet_sim.py /app/wallet_sim.py

USER 1000:1000
EXPOSE 8080
CMD ["python", "/app/wallet_sim.py"]
