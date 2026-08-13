# 本文件定义前端单 SPA 静态资源服务镜像,在构建阶段产出四角色共用的 React 应用。
# 构建与运行基座都由 images/build-images.ps1 注入不可变引用,禁止裸 tag 构建。
ARG NODE_BUILDER_IMAGE=invalid.invalid/base/node-builder:required
ARG NGINX_RUNTIME_IMAGE=invalid.invalid/runtime/nginx:required
ARG VITE_API_BASE_URL
ARG VITE_WS_BASE_URL
ARG VITE_API_TIMEOUT_MS
ARG VITE_SIM_WORKER_COMMAND_TIMEOUT_MS
ARG VITE_SIM_STEP_INTERVAL_MS
FROM ${NODE_BUILDER_IMAGE} AS builder

ARG VITE_API_BASE_URL
ARG VITE_WS_BASE_URL
ARG VITE_API_TIMEOUT_MS
ARG VITE_SIM_WORKER_COMMAND_TIMEOUT_MS
ARG VITE_SIM_STEP_INTERVAL_MS

USER root

WORKDIR /src/frontend

# 使用仓库声明的 pnpm 版本构建唯一 Web 应用,避免 CI 与本地工具链分叉。
RUN corepack enable && corepack prepare pnpm@9.1.0 --activate

COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml frontend/turbo.json frontend/tsconfig.base.json ./
COPY frontend/.eslintrc.js frontend/.prettierrc.json ./
COPY frontend/packages ./packages
COPY frontend/apps ./apps
# Vite 与 Nginx 共同读取同一份生产安全头配置,构建阶段只复制该真相源供 Vite 解析。
COPY images/service/frontend/security-headers.conf /src/images/service/frontend/security-headers.conf

# Vite 只在构建期读取客户端变量;所有部署形态通过构建参数显式注入同一组值。
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL} \
    VITE_WS_BASE_URL=${VITE_WS_BASE_URL} \
    VITE_API_TIMEOUT_MS=${VITE_API_TIMEOUT_MS} \
    VITE_SIM_WORKER_COMMAND_TIMEOUT_MS=${VITE_SIM_WORKER_COMMAND_TIMEOUT_MS} \
    VITE_SIM_STEP_INTERVAL_MS=${VITE_SIM_STEP_INTERVAL_MS}

RUN pnpm install --frozen-lockfile
RUN pnpm build && \
    mkdir -p /out && \
    cp -a apps/web/dist/. /out/

# 构建期预压缩:运行期根文件系统只读(manifest security.read_only_root_filesystem),
# Nginx 无处写压缩缓存,只能在此产出 .gz 供 gzip_static 直接发送。
# 用最高压缩比(-9)——构建一次、分发无数次,运行期零 CPU。
# 仅压缩文本类资源;woff2/woff 与 avif/webp/png 已是压缩格式,再压反而更大。
RUN find /out -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' -o -name '*.json' -o -name '*.svg' -o -name '*.map' -o -name '*.webmanifest' \) \
      -exec gzip -9 -k {} +

FROM ${NGINX_RUNTIME_IMAGE}

LABEL org.opencontainers.image.title="service/frontend"
LABEL org.opencontainers.image.description="Chaimir frontend static asset service"

# Nginx 官方镜像默认监听 80;平台内部统一使用容器端口 8080,避免 root 绑定低端口。
RUN apk upgrade --no-cache \
    && rm -rf /usr/share/nginx/html/* \
    && mkdir -p /var/cache/nginx/client_temp /var/cache/nginx/proxy_temp /var/cache/nginx/fastcgi_temp /var/cache/nginx/uwsgi_temp /var/cache/nginx/scgi_temp \
    && chown -R 101:101 /usr/share/nginx/html /var/cache/nginx /run /var/run /var/log/nginx

COPY images/service/frontend/nginx.conf /etc/nginx/nginx.conf
COPY images/service/frontend/security-headers.conf /etc/nginx/security-headers.conf
COPY --from=builder --chown=101:101 /out/ /usr/share/nginx/html/

USER 101:101
EXPOSE 8080
ENTRYPOINT ["nginx", "-g", "daemon off;"]
