# 本文件构建 DApp 钱包后端服务镜像,最终运行层只保留 Node 服务所需文件。
FROM node:24-alpine3.23 AS deps

LABEL org.opencontainers.image.title="infra/wallet-backend"
LABEL org.opencontainers.image.description="Chaimir wallet backend service"

WORKDIR /app
COPY package.json package-lock.json /app/
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund
COPY wallet-backend.mjs /app/wallet-backend.mjs

FROM node:24-alpine3.23

LABEL org.opencontainers.image.title="infra/wallet-backend"
LABEL org.opencontainers.image.description="Chaimir wallet backend service"

WORKDIR /app
RUN apk upgrade --no-cache \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack
COPY --from=deps --chown=node:node /app /app

ENV HOME=/app
USER node:node
EXPOSE 8080
CMD ["node", "/app/wallet-backend.mjs"]
