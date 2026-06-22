# 本文件构建链上事件监听服务镜像。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="infra/event-listener"
LABEL org.opencontainers.image.description="Chaimir blockchain event listener service"

WORKDIR /app
COPY package.json package-lock.json /app/
RUN apt-get update \
    && apt-get upgrade -y --no-install-recommends \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/bin/npm /usr/local/bin/npx /root/.npm /var/lib/apt/lists/*
COPY event-listener.mjs /app/event-listener.mjs
RUN chown -R 1000:1000 /app

ENV HOME=/app
USER 1000:1000
EXPOSE 8080
CMD ["node", "/app/event-listener.mjs"]
