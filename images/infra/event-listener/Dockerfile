# 本文件构建链上事件监听服务镜像。
FROM node:24-alpine3.23@sha256:595398b0081eacda8e1c4c5b97b76cd1020e4d58a8ebcb4843b9bca1e79e7436

LABEL org.opencontainers.image.title="infra/event-listener"
LABEL org.opencontainers.image.description="Chaimir blockchain event listener service"

WORKDIR /app
COPY package.json package-lock.json /app/
RUN apk upgrade --no-cache \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack /root/.npm
COPY event-listener.mjs /app/event-listener.mjs
RUN chown -R 1000:1000 /app

ENV HOME=/app
USER 1000:1000
EXPOSE 8080
CMD ["node", "/app/event-listener.mjs"]
