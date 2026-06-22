# API 接口总览

> 汇总 11 模块 API 的 Base 路径、错误码段与全局规范。详细接口见各模块"接口设计"文档。
> 最后更新:2026-05-29

---

## 一、API 规范

### 1. 基础
- 基础路径:`/api/v1`,各模块在其下有子前缀(见第二节)。
- 风格:RESTful;资源名词,非 CRUD 用动词子路径;路径嵌套 ≤2 层。
- 协议:HTTPS;实时用 WebSocket。

### 2. 统一响应体
```json
{ "code": 0, "message": "ok", "data": {} }
```
- `code`:0 成功,非 0 业务错误码。
- `message`:用户向友好文案;技术原因只进入日志,不进入响应体。
- 分页:请求 `?page=1&size=20`;响应 `data: { list, total, page, size }`(默认 size=20,上限 100)。

### 3. 鉴权
- JWT 双 Token(M1):`Authorization: Bearer <access_token>`。
- 登录前定位租户:`X-Tenant-Code`(学校短码)。
- Access 15min,Refresh 7d 轮转;单端登录。
- 浏览器原生 WebSocket/iframe 工具入口不能设置 `Authorization` 头时,仅 sandbox 交互入口允许一次性 `?token=<access_token>` 进入;后端校验后写路径受限 HttpOnly Cookie 并清除 query,不得把 token 透传给工具容器。

### 4. 接口分类
- **`[用户]`**:前端调用,JWT + 角色鉴权。
- **`[内部]`**:模块间(服务间)调用,服务间鉴权 + 强制带 `tenant_id`;不对前端开放。在模块化单体中表现为 `internal/contracts` 接口调用,对外暴露时走内网鉴权,并携带调用方模块、`source_ref` 与 trace 上下文。
- 内部 HTTP 服务鉴权统一使用 HMAC-SHA256 签名,签名输入绑定 method、path、tenant_id、source_ref、timestamp 与 trace_id;`timestamp` 必须落在 `SERVICE_AUTH_MAX_SKEW_SECONDS` 环境变量声明的时间窗口内,超出窗口直接拒绝,防止截获请求被重放。

### 5. 错误码分段
| 段 | 模块 |
| --- | --- |
| 1xxxx | 通用(11001 未登录 / 11002 权限不足 / 11003 越权 / 11013 审计写入失败 / 11503–11508 装配依赖缺失) |
| 116xx | 基础横切 transfer 导入导出任务与下载授权 |
| 12xxx–14xxx | M1 账号认证/租户组织/身份装配/导入 |
| 2xxxx | M2 沙箱(21 运行时/22 沙箱/23 工具/24 配额) |
| 3xxxx | M3 评测(31 判题器/32 任务/33 查重) |
| 4xxxx | M4 仿真(41 仿真包/42 会话/43 审核) |
| 5xxxx | M5 题库(51 内容/52 版本/53 共享/54 组卷) |
| 6xxxx | M6 教学(61 课程/62 作业/63 进度/64 成绩) |
| 7xxxx | M7 实验(71 定义/72 实例/73 协作/74 结果) |
| 8xxxx | M8 竞赛(81 赛事/82 报名/83 解题/84 对抗/85 漏洞源) |
| 9xxxx | M9 管理(91 看板/92 审计/93 配置/94 告警) |
| A0xxx | M10 通知 |
| B0xxx | M11 成绩 |

补充:生产代码不得用同一错误码动态替换多种用户文案;新增场景应在对应段落补稳定错误码。`11503–11508` 只用于服务端装配依赖缺失,详细技术原因只进入日志。

### 6. 跨模块调用约定
- 业务实时数据经 M10 `POST /notify/push` 或模块授权后的进度 WS 入口推送;topic 必须带租户前缀 `tenant:{tenant_id}:...`;引擎内部字节流(终端/仿真 stream)走各模块自有 WS。
- 资源回收:M7/M8 调 M2 `/sandboxes/recycle`、M4 `/sessions/recycle`(按 source_ref)。
- source_ref 格式:`<来源>:<年份>:<资源类型>:<id>`(全称,见总纲约定;M7 实例统一为 `experiment:<年份>:instance:<id>`)。

---

## 二、各模块 Base 路径与关键接口

### M1 身份与租户 `/api/v1`
- `/auth/*`:登录(手机号/学号/短信)、刷新、登出、找回、SSO。
- `/platform/applications`、`/platform/tenants`:入驻审核、租户管理 `[平台管理员]`。
- `/tenant/config`、`/tenant/sso`:租户配置。
- `/org/*`:院系/专业/班级。
- `/accounts/*`:账号导入(预览+提交)、增改停用、授予管理员。
- `/me/*`:个人中心。
- `/audit`:审计查询(M9 复用)。

### 基础横切 transfer `/api/v1/transfer`
- `GET /tasks`:查询当前账号导入/导出任务,支持 `channel`、`status`、分页过滤;平台管理员只访问 `tenant_id=0` 的平台任务,租户账号只访问本租户任务。
- `GET /tasks/{id}`:读取当前账号、学校管理员或平台管理员可见的任务快照。
- `POST /tasks/{id}/download-grant`:对已完成任务签发统一文件服务短时下载授权,响应只暴露 `{ token, task, expires_at }`,平台任务和租户任务都必须走统一 storage 对象前缀校验。

> transfer 只暴露通用任务状态和下载授权,不承载模块业务预览、业务结果或业务审批数据。模块导出接口应返回 transfer 任务快照,客户端下载文件需再走 download-grant,禁止模块接口直接返回对象存储直链或 base64 文件体。

### M2 沙箱引擎 `/api/v1/sandbox`
- `/runtimes`、`/tools`:运行时/工具管理 + 接入即测 `[平台管理员]`;镜像预拉取提供触发与状态查询,完成以全目标节点真实拉取成功为准。
- `/sandboxes`:创建/查询/销毁/回收 `[内部]`;`WS /sandboxes/{id}/progress`、`/terminal`。
- `/sandboxes/{id}/files`、`/tools/{code}/*`、`/command-tools/{code}/run`:文件、Web 工具代理和受控命令工具 `[用户]`;Web 工具代理支持浏览器一次性 `token` 入口并换成路径受限 Cookie。
- `/sandboxes/{id}/chain/deploy|tx|query`:链上部署、交易和查询`[用户/内部]`;用户路径按沙箱 owner 校验,内部服务路径按签名 `source_ref` 校验。
- `/sandboxes/{id}/chain/reset`:链恢复创世就绪态`[内部]`。
- `/quota`:配额。

### M3 评测引擎 `/api/v1/judge`
- `/judgers`:判题器管理。
- `/tasks`:提交判题(sandbox_mode: fresh/reuse)`[内部]`;`WS /tasks/{id}/progress`;`GET /tasks/{id}`。
- `/tasks/{id}/rejudge`、`/rejudge/batch`:重判。
- `/tasks/{id}/manual-score`:人工评分。
- `/fingerprints/*`:查重能力 `[内部]`。

### M4 仿真可视化引擎 `/api/v1/sim`
- `/packages/*`:仿真包查询/获取 bundle/扩展接入。
- `/reviews/*`:仿真包审核 `[平台管理员]`。
- `/sessions`:创建/操作上报/回放/分享;`/sessions/recycle` 回收 `[内部]`;`WS /sessions/{id}/stream`(后端计算)。
- `/sessions/{id}/checkpoints`:检查点上报 `[内部]`。

### M5 题库与模板中心 `/api/v1/content`
- `/items/*`:内容 CRUD/检索/题面(过滤答案)/full(内部)/发布/弃用。
- `/items/system-import`:系统/外部源建题 `[内部]`(M8 漏洞题固化入库)。
- `/items/{code}/versions`、`/clone`、`/share`、`/unshare`、`/shared`:版本/复用/共享。
- `/categories`、`/papers`:分类/组卷。
- `/items/{code}/{version}/full`、`/items/batch`:内部取用 `[内部]`。

### M6 教学 `/api/v1/teaching`
- `/courses/*`:课程 CRUD/发布/克隆/共享/邀请码。
- `/chapters`、`/lessons`:章节课时(课时关联 M7 实验/M4 仿真)。
- `/courses/join`、`/members`:选课成员。
- `/assignments/*`、`/submissions/*`:作业/提交/批改(判题调 M3)。
- `/posts`、`/announcements`、`/review`:讨论/公告/评价。
- `/courses/{id}/grades/*`:单课程成绩;`/grades` 只读契约供 M11 聚合;M6 改分后发布 `teaching.grade.updated` 事件。

### M7 实验 `/api/v1/experiment`
- `/experiments/*`:配置/校验/发布。
- `/experiments/{id}/instances`、`/instances/{id}`:实例创建(编排 M2/M4)/工作台/控制;`/instances/{id}/stages/{stage}/activate` 是阶段资源创建唯一写入口;`/instances/{id}/progress` 返回 M10 订阅元信息。
- `/instances/{id}/checkpoints/{cp}/judge`:检查点判分(调 M3)。
- `/instances/{id}/report`、`/reports/{id}/grade`:报告。
- `/groups/*`:多人协作。
- `/internal/instances/{id}/score`、`/internal/stats` `[内部]`(供上层聚合/M9;M7 不直接依赖同层 M6)。

### M8 竞赛 `/api/v1/contest`
- `/contests/*`:赛事管理/题目编排/发布开始结束。
- `/signup`、`/teams/*`:报名组队。
- `/problems/{pid}/env`、`/submit`:解题赛(环境调 M2、判题调 M3)。
- `/battle/entry`、`/battle/matches`、`/matches/{id}/replay`、`/ladder`:对抗赛/回放/天梯。
- `/my/contest-records`、`/result-snapshot`:个人战绩。
- `/cheat-*`:防作弊。
- `/vuln-sources/*`、`/vuln-problems/*`:真实漏洞源(finalize 调 M5 system-import)。
- `/internal/stats`、`/students/{id}/contest-achievements` `[内部]`。

### M9 管理后台 `/api/v1/admin`
- `/platform/dashboard`、`/platform/statistics`、`/platform/tenants`、`/platform/applications`:平台看板/审核状态聚合视图 `[平台管理员]`。
- `/school/dashboard`、`/school/statistics`:学校看板。
- `/audit`、`/audit/export`:统一审计查询中心(查 M1 audit_log)。
- `/configs/*`、`/alert-rules`、`/alert-events/*`:配置/告警。
- `/platform/monitoring/panels`:外接监控嵌入入口。
- `/platform/backups`:备份记录。

### M10 通知与实时推送 `/api/v1/notify`
- `/send` `[内部]`:统一通知发送。
- `/push` `[内部]`:实时推送到带租户前缀的 topic。
- `WS /api/ws`:统一实时通道(订阅 topic)。
- `/inbox/*`、`/preferences`:站内信/偏好 `[用户]`。
- `/announcements/*`:系统公告。

### M11 成绩中心 `/api/v1/grade-center`
- `/level-configs`、`/semesters`:等级映射/学期。
- `/reviews/*`:成绩审核(approve 锁定/unlock 解锁);审核流程在 M11,单课程写保护投影由 M6 自管。
- `/students/{id}/grades`、`/gpa`、`/recompute`:GPA 聚合(只读 M6)。
- `/appeals/*`:申诉(accept 走解锁→改 M6→重算→重锁)。
- `/warnings/*`:学业预警。
- `/transcripts/*`:成绩单 PDF。

---

## 三、典型跨模块调用链(速查)

| 链路 | 调用序列 |
| --- | --- |
| 做实验 | M7 创建实例 → M2 起沙箱 + M4 起仿真 → 学生操作 → M3 判检查点(取 M5)→ M7 保存得分并发布事件 → 完成回收 M2/M4 |
| 交作业 | M6 取 M5 题面 → 学生提交 → M3 判(取 M5 full)→ 回写 M6 → 计成绩 |
| 解题赛 | M8 起 M2 环境 → 提交 → M3 判 → 更新排行 → M10 推送 |
| 对抗赛 | M8 提交参战物 → 撮合起 M2 对局沙箱 → M3 判 → ELO + replay → M10 推天梯 |
| 成绩聚合 | M6 单课程成绩/锁定投影 → M11 只读聚合 GPA → 审核状态流转 → 申诉解锁改 M6 → M11 重算重锁 |
| 漏洞出题 | M8 外部源 → 分级 → 预验证 → M5 system-import 固化 → 竞赛引用 |
| 通知 | 模块发事件/调 M10 send → 渲染模板 → 站内信 + M10 push 红点 |
