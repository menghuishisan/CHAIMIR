# 本文件构建账户抽象 paymaster 服务镜像。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="infra/paymaster"
LABEL org.opencontainers.image.description="Chaimir account abstraction paymaster service"

RUN useradd --system --create-home --home-dir /app chaimir

WORKDIR /app
RUN npm install --omit=dev --no-package-lock fastify@4.29.0 ethers@6.13.5
COPY paymaster-service.mjs /app/paymaster-service.mjs
RUN chown -R chaimir:chaimir /app

USER chaimir
EXPOSE 8080
CMD ["node", "/app/paymaster-service.mjs"]
