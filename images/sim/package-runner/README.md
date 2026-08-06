# sim/package-runner

M4 扩展仿真包通用运行器镜像。教师(`teacher_<id>__`)与第三方(`org_<id>__`)提交的仿真包全部在本镜像内执行。

## 它是运行器,不是某个仿真的镜像

镜像内只有平台自己的装配器与 `stdio-json` 协议实现,**不含任何仿真算法**。新增一个教师扩展包不新增镜像 —— 包正文由后端按会话投递。这与 `sim/<package-code>` 恰好相反(后者把算法固化在镜像里),两者职责不可混用。

## 为什么扩展包在容器里跑而不在浏览器里跑

装配外部 bundle 必须先解封浏览器 Worker 的动态 import 封锁,那等于让第三方代码在**未封锁的 Worker、平台自己的 origin 内**执行一次:它能拿路径受限 Cookie 打平台接口、能 `importScripts` 同源资产,`connect-src 'self'` 挡不住同源行为。而静态扫描拦不住 `globalThis['fe'+'tch']` 这类拼接 —— 动态语言下的能力访问无法靠模式匹配穷尽。

容器给的是**结构性**保证,不依赖看懂代码:独立命名空间、deny-all 网络、non-root、只读根文件系统、丢弃全部 capability、禁用 ServiceAccount Token、seccomp、资源硬限、执行超时。容器里没有 origin、没有凭据、没有网络出口。

前端 CSP 因此保持 `script-src 'self'` 不变。详见 `docs/04-仿真可视化引擎/07-安全设计.md` §2。

## bundle 经 exec stdin 投递

计算 Pod 根文件系统只读且网络 deny-all,容器既写不下也取不到对象存储正文;放开网络就等于拆掉隔离边界。故字节由后端从统一文件服务取出后直接推入容器标准输入,容器内先校验 sha256 与 `sim_package.bundle_hash` 逐字节一致再按 `meta.entry` 装配 —— 投递通道可信不等于内容可信,哈希是"跑的就是审核过的那份代码"的唯一凭据。**容器全程不持有任何凭据**。归档落在唯一可写的 `emptyDir`(`/tmp/sim-bundle`),随会话命名空间删除一并消失。

## 协议

一次 exec = 从标准输入读一行 JSON 命令 → 向标准输出写一行 JSON 响应 → 进程退出。三条命令:

| 命令 | 用途 | 响应 |
| --- | --- | --- |
| `init` | 装配包并产出 tick=0 首帧 | `{ok,descriptor,snapshot}` |
| `apply` | 从 seed + 已有事件重放到当前位置,再执行下一条事件 | `{ok,descriptor,snapshot}` |
| `verify` | 上架前隔离预览:同 seed 双跑逐帧比对 + 回传样例帧 | `{ok,determinism,frames}` |

**容器不常驻会话状态**:状态由后端在每条命令里带回,容器每次从 `seed + events` 重放。代价是重放开销(受包声明 `max_events` 约束),换来的是容器崩溃、Pod 重建、后端重启都不影响过程可复现,容器侧也无需维护会话生命周期。

快照是**完整教学帧**(`tick`/`state`/`view`/`current_step`/`interaction_availability`/`checkpoint_results`),不只是 state:扩展包的 `render` 同样是外部代码,不能在浏览器执行。浏览器只把帧交给平台自己的封闭模式渲染器绘制。

响应体是不可信输入,后端在转发前会再按 TeachingFrame 协议校验一次,不合协议返回 `42011` 并终止会话。

## 运行时代码从哪来

来自 `frontend/packages/sim-sdk/src/runtime/`:容器宿主(`containerHost.ts`)与浏览器 Worker 宿主(`sim.worker.ts`)**共用同一个 `SimEngine`**。引擎有两份实现就必然漂移,而漂移的表现是"同一个 seed 在两个宿主跑出两条过程",回放、分享与判分随之失效。故本目录只有 Dockerfile、manifest 与本文,不放任何脚本;镜像构建时执行 `pnpm --filter @chaimir/sim-sdk build:container` 编译容器入口。

入口模块只接受 `.mjs`:归档内没有 `package.json` 时 Node 按 CJS 解析 `.js`,而扩展包必须默认导出 `SimPackage`(ESM 语义),允许 `.js` 会让"能不能装配"取决于归档里有没有 `type:module`。

## 能力目录登记

能力编号 `sim-package-runner`,digest、命令、受控环境变量、资源与 I/O 上限统一登记在 `SIM_BACKEND_STDIO_ADAPTERS_JSON`;节点规模与执行步数使用仿真包已审核的 `scale_limit`。服务端按 `author_type` 自动为教师/第三方包绑定该能力,教师提交表单不含"运行方式"字段。单租户并发受 `SIM_BACKEND_MAX_CONCURRENT_SESSIONS_PER_TENANT` 约束,超限返回 `42005`。
