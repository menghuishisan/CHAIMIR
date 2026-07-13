# 本文件构建账户抽象 paymaster 服务镜像,最终运行层只保留 Node 运行服务所需文件。
FROM node:24-alpine3.23@sha256:595398b0081eacda8e1c4c5b97b76cd1020e4d58a8ebcb4843b9bca1e79e7436 AS deps

LABEL org.opencontainers.image.title="infra/paymaster"
LABEL org.opencontainers.image.description="Chaimir account abstraction paymaster service"

WORKDIR /app
COPY package.json package-lock.json /app/
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund
COPY paymaster-service.mjs /app/paymaster-service.mjs

FROM node:24-alpine3.23@sha256:595398b0081eacda8e1c4c5b97b76cd1020e4d58a8ebcb4843b9bca1e79e7436

LABEL org.opencontainers.image.title="infra/paymaster"
LABEL org.opencontainers.image.description="Chaimir account abstraction paymaster service"

WORKDIR /app
RUN apk upgrade --no-cache \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack
COPY --from=deps --chown=1000:1000 /app /app

ENV HOME=/app
USER 1000:1000
EXPOSE 8080
CMD ["node", "/app/paymaster-service.mjs"]
