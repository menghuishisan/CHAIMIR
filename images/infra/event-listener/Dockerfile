# 本文件构建链上事件监听服务镜像。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="infra/event-listener"
LABEL org.opencontainers.image.description="Chaimir blockchain event listener service"

RUN useradd --system --create-home --home-dir /app chaimir

WORKDIR /app
RUN npm install --omit=dev --no-package-lock fastify@4.29.0 ethers@6.13.5
COPY event-listener.mjs /app/event-listener.mjs
RUN chown -R chaimir:chaimir /app

USER chaimir
EXPOSE 8080
CMD ["node", "/app/event-listener.mjs"]
