# sim/package-runner

M4 扩展仿真包通用运行器镜像。教师(`teacher_<id>__`)与第三方(`org_<id>__`)提交的仿真包全部在本镜像内执行。

## 镜像职责

镜像内只有平台自己的装配器与 `stdio-json` 协议实现,**不含任何仿真算法**。新增一个教师扩展包不新增镜像 —— 包正文由后端按会话投递。扩展包统一复用该运行器，不按包分别维护镜像。

## 执行边界

教师及第三方代码在后端隔离容器执行，浏览器仅渲染平台封闭协议的纯数据教学帧。执行与网络边界见[M4 安全设计](../../../docs/04-仿真可视化引擎/06-安全设计.md)。

## bundle 经 exec stdin 投递

后端从统一文件服务读取 bundle，经 exec 标准输入投递；容器不访问对象存储或持有凭据。容器先核对 bundle SHA-256 与 `sim_package.bundle_hash`，再按 `meta.entry` 装配。根文件系统只读，归档只写入临时 `emptyDir`（`/tmp/sim-bundle`），随会话命名空间回收。

## 协议

一次 exec = 从标准输入读一行 JSON 命令 → 向标准输出写一行 JSON 响应 → 进程退出。四条命令:

| 命令 | 用途 | 响应 |
| --- | --- | --- |
| `init` | 装配包并产出 tick=0 首帧 | `{ok,descriptor,snapshot}` |
| `apply` | 从 seed + 已有事件重放到当前位置,再执行下一条事件 | `{ok,descriptor,snapshot}` |
| `restore` | 重放指定事件序列 | `{ok,descriptor,snapshot}` |
| `verify` | 同一 seed 双跑，比对包描述和样例帧 | `{ok,descriptor,determinism,frames}` |

**容器不常驻会话状态**:状态由后端在每条命令里带回,容器每次从 `seed + events` 重放。代价是重放开销(受包声明 `max_events` 约束),换来的是容器崩溃、Pod 重建、后端重启都不影响过程可复现,容器侧也无需维护会话生命周期。

快照是**完整教学帧**(`tick`/`state`/`view`/`current_step`/`interaction_availability`/`checkpoint_results`),不只是 state:扩展包的 `render` 同样是外部代码,不能在浏览器执行。浏览器只把帧交给平台自己的封闭模式渲染器绘制。

响应体是不可信输入,后端在转发前会再按 TeachingFrame 协议校验一次,不合协议返回 `42011` 并终止会话。

## 运行时代码从哪来

容器宿主 `containerHost.ts` 与浏览器 Worker 宿主 `sim.worker.ts` 共用 `frontend/packages/sim-sdk/src/runtime/` 中的 `SimEngine`，保持种子、回放与判分语义一致。本目录只维护镜像装配，构建时执行 `pnpm --filter @chaimir/sim-sdk build:container` 编译容器入口，不复制 SDK 运行时实现。

入口模块只接受 `.mjs`:归档内没有 `package.json` 时 Node 按 CJS 解析 `.js`,而扩展包必须默认导出 `SimPackage`(ESM 语义),允许 `.js` 会让"能不能装配"取决于归档里有没有 `type:module`。

## 能力边界与能力目录登记

能力编号 `sim-package-runner`,digest、命令、受控环境变量、资源与 I/O 上限统一登记在 `SIM_BACKEND_STDIO_ADAPTERS_JSON`;节点规模与执行步数使用仿真包已审核的 `scale_limit`。服务端按 `author_type` 自动为教师/第三方包绑定该能力,教师提交表单不含"运行方式"字段。单租户并发受 `SIM_BACKEND_MAX_CONCURRENT_SESSIONS_PER_TENANT` 约束,超限返回 `42005`。

## 验收与清理

验证 `init`、`apply`、`restore`、`verify` 的真实 stdio-json 流程，包括 bundle 摘要、完整教学帧、确定性、并发上限及非法响应拒绝。会话结束回收执行资源、临时卷与投递材料；不删除平台长期对象。通用构建、供应链及隔离要求见[镜像规范](../../../docs/总-镜像与容器设计.md)。
