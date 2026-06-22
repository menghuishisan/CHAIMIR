# 本文件构建账户抽象 paymaster 服务镜像,最终运行层只保留 Node 运行服务所需文件。
FROM node:22-bookworm-slim AS deps

LABEL org.opencontainers.image.title="infra/paymaster"
LABEL org.opencontainers.image.description="Chaimir account abstraction paymaster service"

WORKDIR /app
COPY package.json package-lock.json /app/
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund
COPY paymaster-service.mjs /app/paymaster-service.mjs

FROM node:22-bookworm-slim

LABEL org.opencontainers.image.title="infra/paymaster"
LABEL org.opencontainers.image.description="Chaimir account abstraction paymaster service"

WORKDIR /app
RUN apt-get update \
    && apt-get -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack
COPY --from=deps --chown=node:node /app /app

ENV HOME=/app
USER node:node
EXPOSE 8080
CMD ["/app/paymaster-service.mjs"]
