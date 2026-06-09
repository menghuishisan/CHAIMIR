# 本文件构建 DApp 钱包后端服务镜像。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="infra/wallet-backend"
LABEL org.opencontainers.image.description="Chaimir wallet backend service"

RUN useradd --system --create-home --home-dir /app chaimir

WORKDIR /app
RUN npm install --omit=dev --no-package-lock fastify@4.29.0 ethers@6.13.5
COPY wallet-backend.mjs /app/wallet-backend.mjs
RUN chown -R chaimir:chaimir /app

USER chaimir
EXPOSE 8080
CMD ["node", "/app/wallet-backend.mjs"]
