# Chaimir Frontend

Chaimir 区块链「教学·实验·竞赛」三位一体平台前端 Monorepo。

当前 `apps/` 应用层只有单体前端应用 (`apps/web`)，基于 FSD (Feature-Sliced Design) 架构，按角色进行路由懒加载。`packages/` 是稳定共享能力层，应用页面必须复用。

## 当前目录结构

```text
frontend/
├── apps/
│   └── web/                    # 唯一的单体前端 SPA，采用 FSD 架构
│       └── src/
│           ├── app/            # 驱动层: 全局装配、运行时配置、HTTP 客户端
│           ├── routes/         # 路由边界层: 懒加载角色区、路由清单和权限装配
│           ├── layouts/        # 布局层: AuthLayout, MainLayout, ImmersiveLayout
│           ├── features/       # 领域层: 1:1 映射后端模块, 包含业务组件与逻辑
│           ├── components/     # 应用组件层: 跨业务组装 UI 与状态屏
│           ├── hooks/          # 通用 Hooks
│           ├── styles/         # 全局样式接线
│           └── utils/          # 应用层工具
├── packages/
│   ├── api-client/             # 后端 HTTP/WS 契约 SDK
│   ├── ui/                     # 设计系统、tokens、组件、业务组件、图表组件
│   ├── sim-sdk/                # M4 仿真协议、Worker 运行时、authoring 与内置仿真包
│   └── ide/                    # Monaco/xterm 编辑器与终端装配能力
├── package.json
├── pnpm-workspace.yaml
├── tsconfig.base.json
└── turbo.json
```

## 目录职责

- `apps/web`: 统一的单体前端 SPA 入口。采用 **FSD (Feature-Sliced Design)** 架构，其内部 `src/` 目录的严格职责划分如下：
  - `src/app/`: **驱动层**。负责应用根装配 (`App.tsx`)、运行时配置读取 (`config.ts`)、HTTP 客户端装配 (`api.ts`) 与资源失效协议 (`resourceInvalidation.ts`)。
  - `src/routes/`: **路由边界层 (极其重要)**。`authRoutes.tsx` 维护公共认证路由；`roleRoutes.tsx` 只登记四个角色区前缀并懒加载对应 Section；每个角色区的路径清单与菜单数据收敛在 `routes/sections/`（`*Section.tsx` + `*Navigation.ts`），使非本角色的路由结构与菜单树不进入口包。这里可以引用 `features/*/pages` 页面实现，但不得编写业务逻辑、状态机或页面样式。
  - `src/layouts/`: **布局层**。只维护三大外壳：`AuthLayout` (底层全裸露的认证页壳)、`MainLayout` (四角色共用的侧栏+顶栏光面壳)、`ImmersiveLayout` (无侧栏的全屏沉浸壳)。不得按角色复制布局。
  - `src/features/`: **业务领域层 (核心)**。**1:1 强映射后端 11 个业务模块**（如 `identity`, `teaching`, `experiment`, `contest`）。页面实现统一放在各模块的 `pages/` 子目录，同一功能在不同端的展示（如“提交作业”与“批改作业”）收敛于此，杜绝代码重复。
  - `src/components/`: **应用级组件**。仅存放跨业务领域的 UI 拼装与壳层状态屏（`AppStatusScreen`、`RoleGuard`、`RouteErrorBoundary`、`StatusPages`）。注意：基础和通用业务组件应沉淀至 `packages/ui`。
  - `src/hooks/`: 应用级通用 Hooks（`useAsyncResource`、`useMediaQuery`、`useOnlineStatus`）。
  - `src/utils/`: 应用级工具（会话读写、角色路径契约、用户向错误映射、日期与枚举文案）。角色路径只在 `roleRouting.ts` 定义；侧栏菜单由 `layouts/main/navigation.ts` 的类型化契约描述、由 `routes/sections/*Navigation.ts` 按角色分别提供数据。
- `packages/ui`: 令牌、基础组件、业务组件和可访问图表组件的唯一来源。新应用不得在页面内重复实现已有 Button、Input、Table、Pagination、Modal、Toast、PageScaffold、WorkbenchShell 等组件。
- `packages/ui/src/tokens`: 设计令牌层，维护颜色、间距、圆角、阴影、层级、字体、断点、动效、全局 reset、focus-visible、reduced-motion 等视觉基础变量。页面样式必须引用语义令牌，不写裸 hex。
- `packages/sim-sdk`: 仿真协议类型、校验、确定性 Worker 运行时、authoring 工具和内置仿真包，不含视图层。仿真页面只做业务装配，不重写仿真引擎。
- `packages/ide`: Monaco 编辑器与 xterm 终端的装配能力，不绑定具体实验业务，不含视图布局。

普通业务页面、角色菜单、页面布局和路由装配不得新增到 `packages/`。只有可被多处复用、边界稳定、可独立理解和验证的能力才进入 `packages/`。

## 快速开始

```bash
pnpm install
```

启动开发服务器：
```bash
pnpm run dev
```

## 常用命令

```bash
pnpm type-check
pnpm lint
pnpm build
pnpm clean:artifacts
```

- `clean:artifacts`: 清理 `.turbo` 和 `dist`，不删除依赖。
- `clean:deps`: 清理根、app 和 package 下的 `node_modules`。
- `clean`: 同时清理构建缓存和依赖。

## 重建约束

- UI/UX 只允许使用当前单应用壳、FSD 页面目录和共享包,不得创建角色专属应用壳、页面工厂或独立角色应用。
- 页面接后端必须通过 `@chaimir/api-client`，不得使用模拟数据替代已存在的后端功能。
- 页面和组件样式必须使用 `@chaimir/ui` tokens，优先复用 `@chaimir/ui` 已有组件。
- 仿真工作台的协议、运行时与内置仿真包必须复用 `@chaimir/sim-sdk`，视图层用 `@chaimir/ui` 组件装配。
- 代码编辑器和终端必须复用 `@chaimir/ide`，工作台外壳用 `@chaimir/ui` 的 `WorkbenchShell`。
- 没有后端功能的页面不创建；已有后端功能必须按真实 DTO、字段和权限边界实现。

详见 [前端设计规范](../docs/总-前端设计规范.md)、[工程目录设计](../docs/总-工程目录设计.md) 和 [前端后端功能对齐清单](../docs/前端后端功能对齐清单.md)。
