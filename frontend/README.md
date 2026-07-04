# Chaimir Frontend

Chaimir 区块链「教学·实验·竞赛」三位一体平台前端 Monorepo。

## 包结构

```
frontend/
├── packages/          # 共享包
│   ├── ui/           # 设计系统：tokens + 组件库 + 业务组件
│   ├── api-client/   # 后端 API 客户端
│   ├── shared/       # 跨端工具函数
│   ├── ws-client/    # WebSocket 客户端
│   ├── auth/         # 认证授权
│   └── ide/          # Monaco + xterm 封装
└── apps/             # 四端应用（开发中）
    ├── student/
    ├── teacher/
    ├── school-admin/
    └── platform-admin/
```

## 快速开始

### 安装依赖

```bash
pnpm install
```

### 开发

```bash
# 启动所有包的开发模式
pnpm dev

# 启动特定包
pnpm --filter @chaimir/ui dev
```

### 构建

```bash
# 构建所有包
pnpm build

# 构建特定包
pnpm --filter @chaimir/ui build
```

### 代码检查

```bash
pnpm lint
pnpm type-check
```

## 开发规范

### 前端铁律（10条）

| # | 铁律 | 说明 |
|---|------|------|
| FE-1 | **全令牌化** | 组件零裸 hex，一律引用语义令牌 `var(--color-*)` |
| FE-2 | **无障碍达标** | 对比度 ≥ WCAG AA，键盘可达，焦点可见 |
| FE-3 | **禁用 emoji** | 统一使用 Lucide 图标库 |
| FE-4 | **文案面向用户** | 不暴露开发术语 |
| FE-5 | **无独立工作台落地页** | 登录直达角色第一个功能页 |
| FE-6 | **双模态** | 日常侧栏导航 + 沉浸式全屏 |
| FE-7 | **多步骤服务端持久化** | 向导/长表单中间态以服务端为权威 |
| FE-8 | **错误分层暴露** | 前端只展示友好文案 + trace_id |
| FE-9 | **窗口自适应必做** | 响应式布局，任意视口无横向滚动 |
| FE-10 | **质感开发时打磨** | 组件精度必须以真实产品手感为准 |

详见 `docs/总-前端设计规范.md`

## 设计系统

- **配色**：青电(cyan) × 琥珀(amber) 双色
- **字体**：Inter + JetBrains Mono
- **图标**：Lucide
- **令牌**：原始色阶 → 语义令牌

## 文档

- [前端设计规范](../docs/总-前端设计规范.md)
- [工程目录设计](../docs/总-工程目录设计.md)
- [API 接口总览](../docs/总-API接口总览.md)
