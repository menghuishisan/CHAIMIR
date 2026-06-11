# 本文件构建合约交互面板工具镜像。
FROM python:3.13-alpine

LABEL org.opencontainers.image.title="tool/contract-ui"
LABEL org.opencontainers.image.description="Chaimir contract interaction panel image"

RUN addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /app chaimir

WORKDIR /app
COPY contract_ui.py /app/contract_ui.py

USER 1000:1000
EXPOSE 8080
CMD ["python", "/app/contract_ui.py"]
