# 本文件定义 Node 构建基座镜像,供前端和 Node 工具链镜像多阶段构建复用。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="base/node-builder"
LABEL org.opencontainers.image.description="Chaimir Node builder base image"

# 构建基座先应用发行版安全更新,并固定升级 npm 以修复 Node 官方镜像内置 npm 依赖漏洞。
RUN apt-get update \
    && apt-get upgrade -y --no-install-recommends \
    && npm install -g npm@11.17.0 \
    && npm install -g undici@6.27.0 \
    && rm -rf /usr/local/lib/node_modules/npm/node_modules/undici \
    && cp -a /usr/local/lib/node_modules/undici /usr/local/lib/node_modules/npm/node_modules/undici \
    && npm uninstall -g undici \
    && npm cache clean --force \
    && rm -rf /var/lib/apt/lists/*

# 创建非 root 构建用户,后续镜像需要写缓存时应显式使用 BuildKit cache mount。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /src --create-home chaimir \
    && chown -R chaimir:chaimir /src

# 统一构建目录,后续镜像在此目录内执行 npm/pnpm 构建。
WORKDIR /src
USER chaimir

# 构建镜像默认输出 Node 版本,用于镜像自检。
CMD ["node", "--version"]
