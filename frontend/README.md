# Chaimir Frontend

Chaimir 区块链「教学·实验·竞赛」三位一体平台前端 Monorepo。

当前 `apps/` 应用层已统一重构为单体前端应用 (`apps/web`)，基于 FSD (Feature-Sliced Design) 架构，按角色进行路由懒加载。`packages/` 是稳定共享能力层，重建应用时必须复用，不属于旧页面布局。

## 当前目录结构

```text
frontend/
├── apps/
│   └── web/                    # 唯一的单体前端 SPA，采用 FSD 架构
│       └── src/
│           ├── app/            # 驱动层: 全局 Provider, 路由实例化
│           ├── layouts/        # 布局层: AuthLayout, MainLayout, ImmersiveLayout
│           ├── pages/          # 路由层: 按四端角色懒加载隔离 (auth, student, teacher, etc.)
│           ├── features/       # 领域层: 1:1 映射后端模块, 包含业务组件与逻辑
│           ├── components/     # 应用组件层: 跨业务组装 UI
│           ├── hooks/          # 通用 Hooks
│           ├── store/          # 全局轻量状态
│           └── utils/          # 应用层工具
├── packages/
│   ├── api-client/             # 后端 HTTP/WS 契约 SDK
│   ├── ui/                     # 设计系统、tokens、组件、业务组件、图表组件
│   ├── sim-sdk/                # M4 仿真 SDK、运行时、渲染器和内置仿真包
│   └── ide/                    # Monaco/xterm 工作台能力
├── package.json
├── pnpm-workspace.yaml
├── tsconfig.base.json
└── turbo.json
```

## 目录职责

- `apps/web`: 统一的单体前端 SPA 入口。采用 **FSD (Feature-Sliced Design)** 架构，其内部 `src/` 目录的严格职责划分如下：
  - `src/app/`: **驱动层**。负责路由树初始化、全局 Provider 挂载、鉴权守卫 (RoleGuard) 与全局样式引入。
  - `src/layouts/`: **布局层**。维护三大外壳：`AuthLayout` (登录/空旷页)、`MainLayout` (侧栏+顶栏的标准工作台)、`ImmersiveLayout` (无侧栏的深色全屏工作台)。
  - `src/pages/`: **路由边界层 (极其重要)**。严格按 `auth/`、`student/`、`teacher/`、`school-admin/`、`platform-admin/` 和 `shared/` 划分。这里的组件**必须懒加载**，且禁止编写重度业务，只负责引用 `features/` 中的模块并下发权限。
  - `src/features/`: **业务领域层 (核心)**。**1:1 强映射后端 11 个业务模块**（如 `identity`, `teaching`, `experiment`, `contest`）。同一功能在不同端的展示（如“提交作业”与“批改作业”）收敛于此，杜绝代码重复。
  - `src/components/`: **应用级组件**。仅存放跨业务领域的 UI 拼装（如 `AppSidebar`, `NotificationBell`）。注意：基础和通用业务组件应沉淀至 `packages/ui`。
  - `src/hooks/`: 应用级通用 Hooks（如 `useAuth`, `useWebSocket`）。
  - `src/store/`: 极轻量的全局状态（如 `currentUser`, `currentTenant`）。
  - `src/utils/`: 应用级工具（如本地存储键名定义、日期格式化适配）。
- `packages/ui`: 令牌、基础组件、业务组件和可访问图表组件的唯一来源。新应用不得在页面内重复实现已有 Button、Input、Table、Pagination、Modal、Toast、PageScaffold、WorkbenchShell 等组件。
- `packages/ui/src/tokens`: 设计令牌层，维护颜色、间距、圆角、阴影、层级、字体、断点、动效、全局 reset、focus-visible、reduced-motion 等视觉基础变量。页面样式必须引用语义令牌，不写裸 hex。
- `packages/sim-sdk`: 仿真运行时、渲染器、authoring 工具和内置仿真包。仿真页面只做业务装配，不重写仿真引擎。
- `packages/ide`: 代码实验 IDE/终端工作台能力，不绑定具体实验业务。代码编辑器、终端和工作台布局优先复用这里。

普通业务页面、角色菜单、页面布局和路由装配不得新增到 `packages/`。只有可被多处复用、边界稳定、可独立理解和验证的能力才进入 `packages/`。

## 快速开始

```bash
pnpm install
```

启动开发服务器：
```bash
cd apps/web && pnpm dev
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

- 新 UI/UX 不能沿用已删除的旧应用壳、旧页面工厂或旧四端页面结构。
- 页面接后端必须通过 `@chaimir/api-client`，不得使用模拟数据替代已存在的后端功能。
- 页面和组件样式必须使用 `@chaimir/ui` tokens，优先复用 `@chaimir/ui` 已有组件。
- 仿真工作台必须复用 `@chaimir/sim-sdk`。
- 代码编辑器和终端必须复用 `@chaimir/ide`。
- 没有后端功能的页面不创建；已有后端功能必须按真实 DTO、字段和权限边界实现。

详见 [前端设计规范](../docs/总-前端设计规范.md)、[工程目录设计](../docs/总-工程目录设计.md) 和 [前端后端功能对齐清单](../docs/前端后端功能对齐清单.md)。
