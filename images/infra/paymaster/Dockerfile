# 本文件构建账户抽象 paymaster 服务镜像。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="infra/paymaster"
LABEL org.opencontainers.image.description="Chaimir account abstraction paymaster service"

WORKDIR /app
COPY package.json package-lock.json /app/
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund
COPY paymaster-service.mjs /app/paymaster-service.mjs
RUN chown -R 1000:1000 /app

ENV HOME=/app
USER 1000:1000
EXPOSE 8080
CMD ["node", "/app/paymaster-service.mjs"]
