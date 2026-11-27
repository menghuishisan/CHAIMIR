# Chaimir

区块链教学 · 实验 · 竞赛三位一体平台

> 多租户 SaaS + 学校私有化双形态，Go 模块化单体（11 模块强边界）+ React 四端前端 + K8s 沙箱。

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19+-61DAFB?logo=react)](https://react.dev/)

---

## 简介

Chaimir 是一个面向高等教育的区块链教学平台，集成了**课程学习、实验仿真、竞赛对抗**三大核心场景。

平台采用「业务薄、引擎厚」的架构设计，链环境、判题器和仿真包按各自契约扩展；竞赛采用已实现的解题赛与对抗赛，新赛制须同时闭合执行、计分与回放流程。同时提供完整的多租户隔离机制与 K8s 沙箱执行环境，支持 SaaS 与学校私有化两种部署形态。

项目强调工程化与可维护性，拥有严格的模块边界、事件驱动架构、审计与通知统一处理机制，以及覆盖前后端的详细设计规范。

---

## 核心特性

- 四端统一体验：学生、教师、学校管理员、平台管理员共享同一套设计系统与组件库
- 沉浸式工作台：支持代码实验 IDE、PBFT 共识仿真、对抗赛回放、解题赛答题等教学场景
- 强隔离执行环境：学生代码在独立 K8s 沙箱中运行，支持确定性回放与检查点判定
- 多租户与数据隔离：基于 PostgreSQL RLS 实现租户级隔离，支持跨校聚合与权限控制
- 可视化与仿真引擎：提供图、网络、时序、矩阵、流水线等多种可视化模式
- 完整文档体系：从架构、接口、数据库到前端设计，均有正式规范文档

---

## 部署与镜像

Chaimir 支持两种部署形态：

- **SaaS 形态**：多租户统一部署，适合平台运营商或区域教学中心
- **私有化部署**：学校独立部署，数据与计算资源完全隔离

平台使用 Kubernetes 作为沙箱执行底座，所有核心镜像均通过规范流程构建、分发与安全校验。完整镜像清单、容器编排配置、私有化部署指南以及镜像安全治理流程请参考：

- [总-镜像与容器设计](docs/总-镜像与容器设计.md)
- [总-部署架构设计](docs/总-部署架构设计.md)

---

## 技术架构

- **后端**：Go 模块化单体（11 模块强边界），通过 `internal/contracts` 进行接口化交互
- **前端**：React + TypeScript + pnpm monorepo，四端共享 `@chaimir/ui`、`@chaimir/shared`、`@chaimir/sim-sdk`、`@chaimir/charts` 等包
- **沙箱与执行**：Kubernetes + 容器隔离 + NetworkPolicy + 资源硬限
- **数据与多租户**：PostgreSQL + RLS + sqlc
- **事件与通知**：NATS 事件总线 + 统一通知模块

详细技术选型与模块划分请参考 [总-技术选型](docs/总-技术选型.md) 与 [总-工程目录设计](docs/总-工程目录设计.md)。

---

## 快速开始

### 环境要求

- Go：使用 [go.mod](backend/go.mod) 声明的语言与 toolchain 版本。
- Node.js 与 pnpm：遵循 [package.json](frontend/package.json) 的 engines 和 packageManager；安装依赖使用锁文件。
- 完整运行环境：Docker、Kubernetes 和固定版本的中间件，准备步骤见 [部署指南](deploy/README.md)。

### 后端

```bash
cd backend
go mod download
go build ./cmd/server ./cmd/migrate
go test ./...
```

### 前端

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm build
pnpm lint
pnpm type-check
```

完整环境准备、数据库迁移、镜像构建与部署流程请参考文档。

### 本地启动

```bash
cd deploy
make dev-up
```

前后端、依赖服务、迁移任务和 HTTPS Ingress 由同一条入口统一启动，浏览器访问 `https://www.chaimir.io`。前端的 `pnpm lint`、`pnpm type-check` 和 `pnpm build` 仅用于工作区校验，不是第二套平台启动入口。

---

## CI/CD

后端、前端与通用镜像流水线按真实输入分别构建，完成扫描、SBOM、签名和验证后，由 `image-metadata-promotion` 通过 PR 晋升不可变 digest。权威锁合入 `main` 后部署 staging；`v*` 发布提交读取同一权威锁，经 GitHub Environment 审批部署 prod-saas。入口、凭据和网络前置见 [部署指南](deploy/README.md#cicd)。

---

## 文档

本项目采用**文档驱动开发**，已确认的设计、业务语义与工程约束由 `docs/` 正式文档维护；公开类型、OpenAPI、迁移和镜像 manifest 按所属契约维护机器声明，文档引用其来源并解释业务约束，不复制第二套机器清单。

- [开发规范](docs/总-开发规范与要求.md)
- [文档治理](docs/文档治理规范.md)
- [平台总体蓝图](docs/00-平台总体蓝图.md)
- [总-技术选型](docs/总-技术选型.md)
- [总-工程目录设计](docs/总-工程目录设计.md)
- [总-前端设计规范](docs/总-前端设计规范.md)
- [总-API接口总览](docs/总-API接口总览.md)
- [总-数据库表总览](docs/总-数据库表总览.md)
- [总-镜像与容器设计](docs/总-镜像与容器设计.md)
- [总-部署架构设计](docs/总-部署架构设计.md)

---

## 贡献指南

我们欢迎社区贡献！以下是当前可以参与的具体方向：

### 当前可贡献的方向

- 前端 UI/UX 优化与响应式适配（尤其是沉浸式工作台窄屏体验）
- 仿真可视化模式扩展（新增 PoW、Merkle 树等教学场景）
- 文档补充与案例完善
- 测试用例与自动化验证
- 部署脚本与镜像安全加固

### 贡献流程

1. 阅读[开发规范](docs/总-开发规范与要求.md)及对应模块契约。
2. 改动涉及设计时，先在对应模块的 `docs/` 目录下补充或修改设计文档。
3. 提交 Pull Request 前确保文档与实现一致。
4. 按[评审与验收](docs/总-开发规范与要求.md#评审与验收)提供证据；重构需验证原有行为。

详细架构边界、接口契约、数据库规范与部署流程请参考 `docs/` 下的正式文档。

---

## 常见问题

**Q: Chaimir 支持私有化部署吗？**  
A: 支持。平台设计之初就考虑了 SaaS 与私有化双形态，完整部署文档见 [总-部署架构设计](docs/总-部署架构设计.md)。

**Q: 学生代码如何保证安全？**  
A: 所有学生代码均在独立 Kubernetes 沙箱中执行，配合 NetworkPolicy、资源硬限与用后即毁策略。

**Q: 我可以自己扩展新的仿真场景吗？**  
A: 可以。仿真引擎支持插件化扩展，欢迎参考现有仿真包实现新的教学场景。

---

## 项目状态

Chaimir 处于开发和联调阶段。功能范围以设计文档为准，交付状态以对应版本的实际验收结果为准。

---

## 许可证

本项目采用 [MIT](LICENSE) 许可证。
