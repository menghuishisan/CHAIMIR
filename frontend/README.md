# Chaimir Frontend

Chaimir 区块链「教学·实验·竞赛」三位一体平台前端。当前前端是 **单入口 React 应用 + 少量稳定能力包**，四个角色端通过同一个 `apps/web` 按路径承载。

## 目录结构

```text
frontend/
├── apps/
│   └── web/                    # 唯一浏览器入口：登录页 + 四角色路径
│       ├── src/
│       │   ├── app/            # 应用壳、角色路由类型、hash 路由
│       │   ├── features/       # 登录前公共页与四端业务页面定义
│       │   ├── lib/            # 配置、存储、错误、格式化、实时连接等应用级工具
│       │   ├── route-kit/      # 资源页动作、表格列、结果装配
│       │   └── styles/         # 四端体验层样式
│       ├── package.json
│       └── tsconfig.json
├── packages/
│   ├── api-client/             # 后端 HTTP/WS 契约 SDK
│   ├── ui/                     # 设计系统、组件、业务组件、图表组件
│   ├── sim-sdk/                # M4 仿真 SDK 与内置仿真包
│   └── ide/                    # Monaco/xterm 工作台能力
├── package.json
├── pnpm-workspace.yaml
├── tsconfig.base.json
└── turbo.json
```

## 目录职责

- `apps/web/src/app`: 只放单入口应用壳、路由类型和 hash 路由，不放具体业务页面。
- `apps/web/src/features/auth`: 登录、找回、激活、入驻、SSO、平台登录等登录前公共页面。
- `apps/web/src/features/*`: 角色端业务页面定义和特定页面组件。
- `apps/web/src/lib`: 应用级工具按职责拆分，例如 `config`、`storage`、`errors`、`format`、`realtime`。
- `packages/api-client`: 后端 HTTP/WS 契约 SDK。`src/client.ts` 统一处理 `/api/v1`、Token、`trace_id`、响应信封、上传下载和 WS ticket；`src/modules/*` 按后端公开 API 模块封装请求；`src/types/*` 按后端公开 DTO 模块拆分。这里不承载页面状态、UI、角色菜单或后端 `/internal` 接口。
- `packages/ui`: 令牌、基础组件、业务组件和可访问图表组件的唯一来源。
- `packages/sim-sdk`: 仿真运行时、渲染器、authoring 工具和内置仿真包。
- `packages/ide`: 代码实验 IDE/终端工作台能力，不绑定具体实验业务。

普通业务代码不要再新增 workspace package。只有可被多处复用、边界稳定、可独立理解和验证的能力才进入 `packages/`。

## 快速开始

```bash
pnpm install
pnpm dev
```

`pnpm dev` 只启动 `@chaimir/app-web`，默认端口为 `5173`。四端路径固定为：

- `/student/`
- `/teacher/`
- `/school-admin/`
- `/platform-admin/`

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

## 开发规范

### 前端铁律

| # | 铁律 | 说明 |
|---|------|------|
| FE-1 | 全令牌化 | 组件零裸 hex，一律引用语义令牌 `var(--color-*)` |
| FE-2 | 无障碍达标 | 对比度 ≥ WCAG AA，键盘可达，焦点可见 |
| FE-3 | 禁用 emoji | 统一使用 Lucide 图标库 |
| FE-4 | 文案面向用户 | 不暴露开发术语 |
| FE-5 | 无独立工作台落地页 | 登录直达角色第一个功能页 |
| FE-6 | 双模态 | 日常侧栏导航 + 沉浸式全屏 |
| FE-7 | 多步骤服务端持久化 | 向导/长表单中间态以服务端为权威 |
| FE-8 | 错误分层暴露 | 前端只展示友好文案 + trace_id |
| FE-9 | 窗口自适应必做 | 响应式布局，任意视口无横向滚动 |
| FE-10 | 质感开发时打磨 | 组件精度必须以真实产品手感为准 |

详见 [前端设计规范](../docs/总-前端设计规范.md) 和 [工程目录设计](../docs/总-工程目录设计.md)。
