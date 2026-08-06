# 本文件构建 M4 扩展仿真包通用运行器镜像。
#
# 它是一个**通用运行器,不是某个仿真的镜像**:镜像内只有平台自己的装配器与 stdio-json 协议实现,
# 不含任何仿真算法。新增教师扩展包不新增镜像 —— 包正文由后端按会话经 k8s exec stdin 投递
# (见 docs/总-镜像与容器设计.md §六之一)。
#
# 运行时代码来自 frontend/packages/sim-sdk:容器宿主与浏览器 Worker 宿主共用同一个 SimEngine,
# 引擎有两份实现就必然漂移,而漂移的表现是"同一个 seed 在两个宿主跑出两条过程"。
# 因此这里不复制脚本,而是从 sim-sdk 源码编译容器入口(pnpm build:container)。
ARG NODE_BUILDER_IMAGE=invalid.invalid/base/node-builder:required
FROM ${NODE_BUILDER_IMAGE} AS builder

USER root
WORKDIR /src/frontend

# 使用仓库声明的 pnpm 版本,避免 CI 与本地工具链分叉。
RUN corepack enable && corepack prepare pnpm@9.1.0 --activate

COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml frontend/tsconfig.base.json ./
COPY frontend/packages/sim-sdk ./packages/sim-sdk

# 只装 sim-sdk 一个工作区包的开发依赖(仅 typescript):
# runner 承载不可信代码,依赖越少可审计面越小,运行期更是零第三方依赖。
RUN pnpm install --frozen-lockfile --filter @chaimir/sim-sdk
RUN pnpm --filter @chaimir/sim-sdk build:container

FROM ${NODE_BUILDER_IMAGE}

LABEL org.opencontainers.image.title="sim/package-runner"
LABEL org.opencontainers.image.description="Chaimir M4 extension simulation package runner"

USER root
RUN apk upgrade --no-cache \
    && mkdir -p /opt/sim-runner /tmp/sim-bundle \
    && chown -R chaimir:chaimir /opt/sim-runner /tmp/sim-bundle

# 产物是 CommonJS(tsconfig.container.json 用 NodeNext + .js 后缀),
# 显式声明 type 以免继承到别处的 ESM 判定。
COPY --from=builder --chown=chaimir:chaimir /src/frontend/packages/sim-sdk/dist/ /opt/sim-runner/
RUN printf '{\n  "name": "chaimir-sim-runner",\n  "private": true,\n  "type": "commonjs"\n}\n' > /opt/sim-runner/package.json \
    && chown chaimir:chaimir /opt/sim-runner/package.json

USER chaimir
WORKDIR /opt/sim-runner

# 容器由后端经 k8s exec 逐命令驱动:每次 exec 起一个短命进程读一行命令、写一行响应后退出。
# Pod 常驻进程只用于保持就绪(idle_command 由能力目录给出),故此处 ENTRYPOINT 即命令入口。
ENTRYPOINT ["node", "/opt/sim-runner/runtime/containerMain.js"]
