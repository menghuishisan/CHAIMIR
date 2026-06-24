# 本文件构建账户抽象 paymaster 服务镜像,最终运行层只保留 Node 运行服务所需文件。
FROM node:24-alpine3.23 AS deps

LABEL org.opencontainers.image.title="infra/paymaster"
LABEL org.opencontainers.image.description="Chaimir account abstraction paymaster service"

WORKDIR /app
COPY package.json package-lock.json /app/
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund
COPY paymaster-service.mjs /app/paymaster-service.mjs

FROM node:24-alpine3.23

LABEL org.opencontainers.image.title="infra/paymaster"
LABEL org.opencontainers.image.description="Chaimir account abstraction paymaster service"

WORKDIR /app
RUN apk upgrade --no-cache \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack
COPY --from=deps --chown=node:node /app /app

ENV HOME=/app
USER node:node
EXPOSE 8080
CMD ["node", "/app/paymaster-service.mjs"]
