# Chaimir

区块链教学 · 实验 · 竞赛三位一体平台

> 多租户 SaaS + 学校私有化双形态，Go 模块化单体（11 模块强边界）+ React 四端前端 + K8s 沙箱。

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![React](https://img.shields.io/badge/React-18+-61DAFB?logo=react)](https://react.dev/)

---

## 简介

Chaimir 是一个面向高等教育的区块链教学平台，集成了**课程学习、实验仿真、竞赛对抗**三大核心场景。

平台采用「业务薄、引擎厚」的架构设计，所有可扩展能力（链环境、判题器、仿真引擎、赛制）均支持插件化扩展。同时提供完整的多租户隔离机制与 K8s 沙箱执行环境，支持 SaaS 与学校私有化两种部署形态。

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

- Go 1.25+
- Node.js 20+ + pnpm 9+
- PostgreSQL 14+
- Docker & Kubernetes（推荐，用于完整沙箱环境）

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
pnpm install
pnpm build
pnpm lint
pnpm type-check
```

完整环境准备、数据库迁移、镜像构建与部署流程请参考文档。

### 本地开发

```bash
# 前端四端与共享包开发
cd frontend
pnpm dev
```

需要完整沙箱、数据库、对象存储、病毒扫描与镜像供应链时，请按 [deploy/README](deploy/README.md) 使用 `deploy/` 下的 Kustomize 清单与 Makefile。日常本地进程联调不需要先应用完整 Kubernetes overlay。

---

## CI/CD

GitHub Actions 采用路径触发：

- `backend.yml`：后端变更后执行静态检查、测试、镜像构建、Trivy 扫描、Cosign 签名与 Harbor 推送。
- `frontend.yml`：前端变更后使用 pnpm 执行 lint、type-check、build，并在 main 推送时构建前端镜像。
- `images.yml`：镜像目录变更后按 manifest 构建、扫描、签名并生成 digest 锁。
- `deploy.yml`：backend workflow 在 main 成功完成并产出同 SHA 后端镜像后，自动部署 staging；`v*` tag 经 GitHub Environment 审批后部署 prod-saas。

README、docs 或仅前端资源变更不会直接创建 staging Deployment。staging 只消费已经通过后端流水线构建、扫描、签名并推送到 Harbor 的后端镜像。

---

## 文档

本项目采用**文档驱动开发**，所有设计决策、接口契约、数据模型均以 `docs/` 下的正式文档为唯一真相源。

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

1. 阅读 [总-工程目录设计](docs/总-工程目录设计.md)、[总-前端设计规范](docs/总-前端设计规范.md) 与对应模块文档，了解项目铁律与开发规范。
2. 改动涉及设计时，先在对应模块的 `docs/` 目录下补充或修改设计文档。
3. 提交 Pull Request 前确保文档与实现一致。
4. 所有改动需通过代码审查与文档一致性检查。

详细架构边界、接口契约、数据库规范与部署流程请参考 `docs/` 下的正式文档。

---

## 常见问题

**Q: Chaimir 支持私有化部署吗？**  
A: 支持。平台设计之初就考虑了 SaaS 与私有化双形态，完整部署文档见 [总-部署架构设计](docs/总-部署架构设计.md)。

**Q: 学生代码如何保证安全？**  
A: 所有学生代码均在独立 Kubernetes 沙箱中执行，配合 NetworkPolicy、资源硬限与用后即毁策略。

**Q: 我可以自己扩展新的仿真场景吗？**  
A: 可以。仿真引擎支持插件化扩展，欢迎参考现有仿真包实现新的教学场景。

**Q: 项目目前处于什么阶段？**  
A: 核心模块与文档体系已基本完善，部分模块已通过内部验收，欢迎社区参与共建。

---

## 项目状态

Chaimir 目前处于活跃开发阶段，核心模块与文档体系已基本完善，部分模块已通过内部验收。欢迎感兴趣的开发者与高校参与共建。

---

## 许可证

本项目采用 [MIT](LICENSE) 许可证。
