# 本文件定义前端单 SPA 静态资源服务镜像,在构建阶段产出四角色共用的 React 应用。
# 构建与运行基座都由 images/build-images.ps1 注入不可变引用,禁止裸 tag 构建。
ARG NODE_BUILDER_IMAGE=invalid.invalid/base/node-builder:required
ARG NGINX_RUNTIME_IMAGE=invalid.invalid/runtime/nginx:required
ARG VITE_API_BASE_URL
ARG VITE_WS_BASE_URL
ARG VITE_API_TIMEOUT_MS
ARG VITE_RUNTIME_SELFTEST_POLL_INTERVAL_MS
ARG VITE_RUNTIME_SELFTEST_POLL_TIMEOUT_MS
ARG VITE_SIM_WORKER_COMMAND_TIMEOUT_MS
ARG VITE_SIM_STEP_INTERVAL_MS
ARG CHAIMIR_FRONTEND_SOURCE_DIGEST=unknown
FROM ${NODE_BUILDER_IMAGE} AS builder

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ARG VITE_API_BASE_URL
ARG VITE_WS_BASE_URL
ARG VITE_API_TIMEOUT_MS
ARG VITE_RUNTIME_SELFTEST_POLL_INTERVAL_MS
ARG VITE_RUNTIME_SELFTEST_POLL_TIMEOUT_MS
ARG VITE_SIM_WORKER_COMMAND_TIMEOUT_MS
ARG VITE_SIM_STEP_INTERVAL_MS
ARG CHAIMIR_FRONTEND_SOURCE_DIGEST=unknown
ARG TURBO_CONCURRENCY=1
ARG NODE_MAX_OLD_SPACE_SIZE=2048

USER root

WORKDIR /src/frontend

# 源码摘要必须参与复制层之前的命令,确保未提交改动不会复用旧的源码层。
RUN test -n "${CHAIMIR_FRONTEND_SOURCE_DIGEST}"

# 使用仓库声明的 pnpm 版本构建唯一 Web 应用,避免 CI 与本地工具链分叉。
RUN export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && corepack enable \
    && for attempt in 1 2 3 4 5; do \
        if corepack prepare pnpm@9.1.0 --activate; then break; fi; \
        if [ "$attempt" -eq 5 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done

COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml frontend/turbo.json frontend/tsconfig.base.json ./
COPY frontend/eslint.config.js frontend/.prettierrc.json ./
COPY frontend/packages/api-client/package.json ./packages/api-client/package.json
COPY frontend/packages/ide/package.json ./packages/ide/package.json
COPY frontend/packages/sim-sdk/package.json ./packages/sim-sdk/package.json
COPY frontend/packages/ui/package.json ./packages/ui/package.json
COPY frontend/apps/web/package.json ./apps/web/package.json
# Vite 与 Nginx 共同读取同一份生产安全头配置,构建阶段只复制该真相源供 Vite 解析。
COPY images/service/frontend/security-headers.conf /src/images/service/frontend/security-headers.conf

# Vite 只在构建期读取客户端变量;所有部署形态通过构建参数显式注入同一组值。
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL} \
    VITE_WS_BASE_URL=${VITE_WS_BASE_URL} \
    VITE_API_TIMEOUT_MS=${VITE_API_TIMEOUT_MS} \
    VITE_RUNTIME_SELFTEST_POLL_INTERVAL_MS=${VITE_RUNTIME_SELFTEST_POLL_INTERVAL_MS} \
    VITE_RUNTIME_SELFTEST_POLL_TIMEOUT_MS=${VITE_RUNTIME_SELFTEST_POLL_TIMEOUT_MS} \
    VITE_SIM_WORKER_COMMAND_TIMEOUT_MS=${VITE_SIM_WORKER_COMMAND_TIMEOUT_MS} \
    VITE_SIM_STEP_INTERVAL_MS=${VITE_SIM_STEP_INTERVAL_MS} \
    TURBO_CONCURRENCY=${TURBO_CONCURRENCY} \
    NODE_OPTIONS=--max-old-space-size=${NODE_MAX_OLD_SPACE_SIZE}

RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && pnpm config set fetch-retries 3 \
    && pnpm config set fetch-timeout 60000 \
    && pnpm config set network-concurrency 8 \
    && pnpm install --frozen-lockfile
COPY frontend/packages ./packages
COPY frontend/apps ./apps
RUN test -n "${CHAIMIR_FRONTEND_SOURCE_DIGEST}" && pnpm build && \
    mkdir -p /out && \
    cp -a apps/web/dist/. /out/

# 构建期预压缩:运行期根文件系统只读(manifest security.read_only_root_filesystem),
# Nginx 无处写压缩缓存,只能在此产出 .gz 供 gzip_static 直接发送。
# 用最高压缩比(-9)——构建一次、分发无数次,运行期零 CPU。
# 仅压缩文本类资源;woff2/woff 与 avif/webp/png 已是压缩格式,再压反而更大。
RUN find /out -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' -o -name '*.json' -o -name '*.svg' -o -name '*.map' -o -name '*.webmanifest' \) \
      ! -name 'runtime-config.js' \
      -exec gzip -9 -k {} +

FROM ${NGINX_RUNTIME_IMAGE}

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

LABEL org.opencontainers.image.title="service/frontend"
LABEL org.opencontainers.image.description="Chaimir frontend static asset service"

# Nginx 官方镜像默认监听 80;平台内部统一使用容器端口 8080,避免 root 绑定低端口。
RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && sed -i 's#https://dl-cdn.alpinelinux.org/alpine#https://mirrors.aliyun.com/alpine#g' /etc/apk/repositories \
    && rm -f /var/cache/apk/* \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk upgrade --cache-dir /var/cache/apk --update-cache && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && rm -rf /usr/share/nginx/html/* \
    && mkdir -p /var/cache/nginx/client_temp /var/cache/nginx/proxy_temp /var/cache/nginx/fastcgi_temp /var/cache/nginx/uwsgi_temp /var/cache/nginx/scgi_temp \
    && chown -R 101:101 /usr/share/nginx/html /var/cache/nginx /run /var/run /var/log/nginx

COPY images/service/frontend/nginx.conf /etc/nginx/nginx.conf
COPY images/service/frontend/security-headers.conf /etc/nginx/security-headers.conf
COPY --chmod=0555 images/service/frontend/runtime-config.sh /usr/local/bin/chaimir-runtime-config
COPY --from=builder --chown=101:101 /out/ /usr/share/nginx/html/

USER 101:101
EXPOSE 8080
ENTRYPOINT ["nginx", "-g", "daemon off;"]
