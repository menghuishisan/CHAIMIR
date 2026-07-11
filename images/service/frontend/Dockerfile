# 本文件定义前端单 SPA 静态资源服务镜像,在构建阶段产出四角色共用的 React 应用。
ARG NODE_BUILDER_IMAGE=node:20-alpine
FROM ${NODE_BUILDER_IMAGE} AS builder

USER root

WORKDIR /src/frontend

# 使用仓库声明的 pnpm 版本构建唯一 Web 应用,避免 CI 与本地工具链分叉。
RUN corepack enable && corepack prepare pnpm@9.1.0 --activate

COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml frontend/turbo.json frontend/tsconfig.base.json ./
COPY frontend/.eslintrc.js frontend/.prettierrc.json ./
COPY frontend/packages ./packages
COPY frontend/apps ./apps

RUN pnpm install --frozen-lockfile
RUN pnpm build && \
    mkdir -p /out && \
    cp -a apps/web/dist/. /out/

FROM nginx:1.27-alpine

LABEL org.opencontainers.image.title="service/frontend"
LABEL org.opencontainers.image.description="Chaimir frontend static asset service"

# Nginx 官方镜像默认监听 80;平台内部统一使用容器端口 8080,避免 root 绑定低端口。
RUN apk upgrade --no-cache \
    && rm -rf /usr/share/nginx/html/* \
    && mkdir -p /var/cache/nginx/client_temp /var/cache/nginx/proxy_temp /var/cache/nginx/fastcgi_temp /var/cache/nginx/uwsgi_temp /var/cache/nginx/scgi_temp \
    && chown -R 101:101 /usr/share/nginx/html /var/cache/nginx /run /var/run /var/log/nginx

COPY images/service/frontend/nginx.conf /etc/nginx/nginx.conf
COPY --from=builder --chown=101:101 /out/ /usr/share/nginx/html/

USER 101:101
EXPOSE 8080
ENTRYPOINT ["nginx", "-g", "daemon off;"]
