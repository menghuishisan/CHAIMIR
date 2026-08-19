# API 接口总览

> 汇总 11 模块 API 的 Base 路径、错误码段与全局规范。详细接口见各模块"接口设计"文档。
> 最后更新:2026-08-06

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
- HTTP 状态码在业务面恒为 200,交互一律按 `code` 判定;不存在"按状态码分支"的第二套判断。
- 信封只有两类显式例外:统一文件服务 `GET /download` 流式返回附件,以及运维探针 `/-/healthz`、`/-/readyz`、`/api/healthz`(消费者是 kubelet/网关,判定信号只有状态码,详见 `docs/总-部署架构设计.md` §三 健康探针契约)。除此之外任何接口不得绕过信封。

#### 2.1 资源 ID 传输契约

- 数据库雪花 ID 在所有公开 HTTP/WS JSON 中统一编码为十进制字符串，例如 `"910000000000000001"`；禁止输出或接收 JSON number。
- path/query 中的 ID 按十进制字符串解析，必须为大于 0 的 `int64` 范围整数；空白、符号、小数、指数写法和前导零均非法。
- 后端 DTO 使用既有 `internal/platform/ids` 中的统一边界类型，模块 service/sqlc model 内部仍使用 `int64`；不得新建平行 ID 工具包或把传输类型扩散到数据库层。
- 前端 API Client 统一使用 `SnowflakeID = string`，不得对资源 ID 调用 `Number`、`parseInt`、算术运算或隐式数值比较。
- 含 ID 的适配器声明、资源约束和 WebSocket payload 必须使用有类型 DTO，不允许把 ID 藏在无类型 JSON 中绕过该规则。

这是一次不兼容的统一契约，不接受旧 number ID 双读或降级分支。

### 3. 鉴权
- JWT 会话(M1):登录响应只返回短期 `access_token`；长期 refresh token 由后端写入 `Secure; HttpOnly; SameSite=Strict` cookie，浏览器脚本和 Web Storage 均不可读取。客户端用空 JSON 请求 `/auth/refresh` 并携带 `X-Chaimir-Refresh: 1` 轮转 access，禁止在请求体或 URL 传 refresh token。
- 登录前定位租户:`X-Tenant-Code`(学校短码)。
- Access 15min,Refresh 7d 轮转;单端登录。
- 浏览器原生 WebSocket 不能设置 `Authorization` 头时,前端先通过 `POST /api/v1/auth/ws-ticket` 使用当前登录态为目标 WS path 换取短时路径绑定票据,再以 `?ticket=<ws_ticket>` 建连;后端校验票据路径和服务端会话,不得在普通 WS URL 中携带 access token。
- iframe/Web 工具入口不能设置 `Authorization` 头时,前端先通过 `POST /api/v1/auth/browser-ticket` 为工具代理路径前缀换取短时票据,再以 `?ticket=<browser_ticket>` 首次进入;后端校验票据、服务端会话与路径前缀后重新签发 access token,写入路径受限 HttpOnly Cookie 并清除 query。access token 不得出现在 URL,票据和平台 Cookie 均不得透传给工具容器。

### 4. 接口分类
- **`[用户]`**:前端调用,JWT + 角色鉴权。
- **`[内部]`**:模块间(服务间)调用,服务间鉴权 + 强制带 `tenant_id`;不对前端开放。在模块化单体中表现为 `internal/contracts` 接口调用,对外暴露时走内网鉴权,并携带调用方模块、`source_ref` 与 trace 上下文。
- 内部 HTTP 服务鉴权统一使用 HMAC-SHA256 签名,签名输入绑定 method、path、tenant_id、source_ref、timestamp 与 trace_id;`timestamp` 必须落在 `SERVICE_AUTH_MAX_SKEW_SECONDS` 环境变量声明的时间窗口内,超出窗口直接拒绝,防止截获请求被重放。

### 5. 错误码分段
| 段 | 模块 |
| --- | --- |
| 1xxxx | 通用(11001 未登录 / 11002 权限不足 / 11003 越权 / 11013 审计写入失败 / 11503–11508 装配依赖缺失) |
| 116xx | 基础横切 transfer 导入导出任务与下载授权 |
| 117xx | 统一文件服务下载授权消费与文件读取 |
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
- `/auth/ws-ticket`:为浏览器 WebSocket 签发短时路径绑定连接票据。
- `/auth/browser-ticket`:为浏览器 Web 工具代理签发短时路径前缀绑定入口票据。
- `/platform/applications`、`/platform/tenants`:入驻审核、租户管理 `[平台管理员]`。
- `/tenant/config`、`/tenant/sso`:租户配置。`POST /tenant/logo` 上传校徽(multipart,**上传即生效**,同一请求内落 `logo_ref` 并返回新的配置视图),`DELETE /tenant/logo` 移除;`PATCH /tenant/config` 不接受 `logo_ref`。`GET /tenant/brand` 是**免鉴权且无参数**的品牌读取口,返回 `{ display_name, logo_image }`,仅 `deploy_mode=school` 下有内容 —— SaaS 登录页面对未确定租户,本无校徽可显示,而带 `tenant_code` 的公开端点会成为廉价的租户枚举通道。校徽以 data URI 内联下发而不签发投放授权:登录页没有会话,授权绑不到账号(详见 M1 接口设计 §4)。
- `/org/*`:院系/专业/班级。读对教师/学校管理员开放,写只对学校管理员开放;`GET /org/classes/{id}/students?page=&size=` 是教师侧挑选学生的唯一入口,只出编号/姓名/学号并返回 `{list,total,page,size}`。
- `/accounts/*`:账号导入(预览+提交)、增改停用、授予管理员。**只对学校管理员开放** —— 它是账号目录(带手机号掩码、状态、角色),业务模块与教师端取学生一律走 `/org/classes/{id}/students` 或 `contracts.IdentityService`。
- `/me/*`:个人中心。`GET /me`、`POST /me/password`、`GET /me/sessions` 对租户账号与平台管理员都开放(平台身份走独立分支,只返回姓名/状态/角色);`POST /me/phone` 只对租户账号开放,平台账号无手机号字段。
- 审计:M1 只持有并写入 `audit_log`,不注册查询路由;对外查询与导出统一在 M9 `/admin/audit`、`/admin/audit/export`。

### 基础横切 transfer `/api/v1/transfer`
- `GET /tasks`:查询当前账号导入/导出任务,支持 `channel`、`status`、分页过滤;平台管理员只访问 `tenant_id=0` 的平台任务,租户账号只访问本租户任务。
- `GET /tasks/{id}`:读取当前账号、学校管理员或平台管理员可见的任务快照。
- `POST /tasks/{id}/download-grant`:对已完成任务签发统一文件服务短时下载授权,响应只暴露 `{ token, task, expires_at }`,平台任务和租户任务都必须走统一 storage 对象前缀校验。

> transfer 只暴露通用任务状态和下载授权,不承载模块业务预览、业务结果或业务审批数据。模块导出接口应返回 transfer 任务快照,客户端下载文件需再走 download-grant,禁止模块接口直接返回对象存储直链或 base64 文件体。

> 任务快照的 `file_name` 是契约必填字段(创建任务时缺失即拒绝),任务成功后原样登记为 `artifact_file_name`。客户端保存文件时只取 `artifact_file_name`,不得再按多个字段依次取值或自造默认文件名。

### 统一文件服务 `/api/v1/storage`
- `GET /download?token=...`:登录账号消费与本人账号、租户和资源边界绑定的短时授权,成功响应直接流式返回文件,不使用 JSON 信封。

授权在签名内携带 `mode`,决定投放方式;`mode` 由签发模块决定,客户端无法篡改(改动会导致验签失败):

| mode | 消费次数 | 响应头 | Range | 用途 |
| --- | --- | --- | --- | --- |
| `download` | 一次(Redis 原子标记) | `Content-Disposition: attachment` | 不支持 | 导入导出产物、成绩单、题库附件、课时资料等取件场景 |
| `stream` | 授权有效期内可重复 | `Content-Disposition: inline` + `Accept-Ranges: bytes` | 支持(`206 Partial Content`) | 课时视频等需要边下边播与拖动进度的场景 |

`stream` 之所以不能沿用一次性消费:播放器为拖动进度会对同一资源发多个分段请求,首个请求就会烧掉一次性令牌。它以更短有效期(`STORAGE_STREAM_GRANT_TTL_SECONDS`)与同样的租户+账号+资源前缀绑定换取可重复性,不放宽任何一项边界校验。两种 mode 共用同一端点、同一 grant 构造器与签名器,不新增第二条投放路由。

**鉴权载体**:`<video src>`、`<img src>` 这类由浏览器自身发起的请求发不出 `Authorization` 头,故本入口接受 **Bearer 头**(XHR 取件)**或路径受限 Cookie**(浏览器直连),沿用 M2 浏览器工具代理已有的 `chaimir_access` 机制。两个入口共用 Cookie 与会话校验底座,但首次建链方式不同:M2 工具代理只接受 `/auth/browser-ticket` 签发的短时路径票据,统一文件服务由签发 `mode=stream` 授权的业务接口直接写 Cookie。文件服务**不接受任何 query 形式的 access token** —— `token` 参数位只表示投放授权,同名两义会让验签路径产生歧义。Cookie 作用域限定 `storage.DownloadCookiePath`(`/api/v1/storage`)、`HttpOnly`,有效期与 access token 一致;授权仍绑定租户、账号与资源前缀,`authorizeDownloadGrant` 的会话比对不因载体变化而放宽。

> 所有模块签发的授权都只能由本入口消费。业务模块和前端页面不得拼接 MinIO 地址、复制 token 验签逻辑或新增模块私有下载路由。

### M2 沙箱引擎 `/api/v1/sandbox`
- `/runtimes`、`/tools`:运行时/工具管理 + 接入即测 `[平台管理员]`;镜像预拉取提供触发与状态查询,完成以全目标节点真实拉取成功为准。
- `/catalog`:编排目录 `[教师/学校管理员/平台管理员]` —— 可用运行时(含其可用镜像版本)与可用工具的最小字段集。业务模块编排环境只用它,不放开 `/runtimes`、`/tools`:那两条会连带下发容器编排清单、镜像 digest 地址、命令白名单与自检详情,属平台运维资产。
- `/sandboxes`:创建/查询/销毁/回收 `[内部]`;`WS /sandboxes/{id}/progress`、`/terminal`。
- `/sandboxes/{id}/files`、`/tools/{code}/*`、`/command-tools/{code}/run`:文件、Web 工具代理和受控命令工具 `[用户]`;Web 工具代理支持浏览器一次性 `token` 入口并换成路径受限 Cookie。
- `GET /sandboxes/{id}` 响应中的 `capabilities` 由运行时命令清单与服务端注册表计算,是前端文件、终端、命令工具和链操作入口的权威能力声明。
- `/sandboxes/{id}/chain/deploy|tx|query`:链上部署、交易和查询`[用户/内部]`;用户路径按沙箱 owner 校验,内部服务路径按签名 `source_ref` 校验。
- `/sandboxes/{id}/chain/reset`:链恢复创世就绪态`[内部]`。
- `/quota`:沙箱配额查看与调整。`GET /quota` 校管读本租户(忽略客户端传入的 `tenant_id`),平台管理员必须显式传 `tenant_id` 指定目标租户;`PATCH /quota` 平台管理员在请求体传 `tenant_id`,校管由服务端覆写为会话租户。

### M3 评测引擎 `/api/v1/judge`
- `/judgers`:判题器管理 `[平台管理员]`。
- `/catalog`:判题器目录 `[教师/学校管理员/平台管理员]` —— 可用判题方式的编码/名称/类型。教师配置检查点只用它,不放开 `/judgers`:`resource_spec` 里的判题镜像版本、受控命令与执行组件属判题私密面。
- `/tasks`:提交判题(sandbox_mode: fresh/reuse)`[内部]`;`WS /tasks/{id}/progress`;`GET /tasks/{id}`。
- `/tasks/{id}/rejudge`、`/rejudge/batch`:重判。
- `/tasks/{id}/manual-score`:人工评分。
- `/fingerprints/*`:查重能力 `[内部]`。

### M4 仿真可视化引擎 `/api/v1/sim`
- `/packages/*`:仿真包查询/扩展接入。`GET /packages` 同一路由按 `mine` 分叉:缺省只回已上架包;`mine=true` 按服务端会话账号回本人提交的包(状态可选,缺省全部状态),作者边界不接受客户端传参。提交请求不含 `compute`/`backend_adapter` —— 执行位置按作者类型派生(内置=浏览器 Worker、教师/第三方=后端隔离容器),提交者没有可选项,故也没有运行能力查询接口。**无 bundle 下载授权接口**:内置包按 `code` 在 Worker 内装配,扩展包 bundle 只经受控 k8s exec 投入隔离容器,从不下发浏览器。
- `/reviews/*`:仿真包审核 `[平台管理员]`,列表携带隔离预览渲出的样例教学帧供人工判定。确定性双跑结论、渲帧结论与样例帧由 M4 进程内的隔离预览任务直接写入审核记录,**没有回写接口** —— 一个能把门禁置为通过的服务签名入口就是审核闸门的旁路。
- `/sessions`:创建/操作上报/回放/分享;`/sessions/recycle` 回收 `[内部]`;`WS /sessions/{id}/stream`(隔离容器执行:客户端只发 step/event/back/restart 四种受控命令,服务端首帧下发包自描述信息、其后逐帧推完整教学快照,前端只渲染)。
- `/sessions/{id}/checkpoints`:检查点上报 `[内部]`。

### M5 题库与模板中心 `/api/v1/content`
- `/items/*`:内容 CRUD/检索/题面(过滤答案)/full(内部)/发布/弃用。
- `/items/system-import`:系统/外部源建题 `[内部]`(M8 漏洞题固化入库)。
- `/items/{code}/versions`、`/clone`、`/share`、`/unshare`、`/shared`:版本/复用/共享。
- `/categories`、`/papers`:分类/组卷。
- `/items/{code}/{version}/full`、`/items/batch`:内部取用 `[内部]`。

### M6 教学 `/api/v1/teaching`
- `/courses/*`:课程 CRUD/发布/克隆/共享/邀请码。`POST /courses/cover` 上传封面(multipart,返回 `object_ref`,不绑课程 id —— 先落上传教师的暂存位,随创建/编辑课程提交后由服务端搬到该课程正式位);`POST /courses/{id}/cover/access` 换取封面投放授权(`mode=stream`)。封面只对能看到该课程的人可见,不设公开路径;课程列表缩略图用平台纸材质,不逐行换授权。
- `/chapters`、`/lessons`:章节课时**写入口**(课时关联 M7 实验/M4 仿真);只读一律走 `GET /courses/{id}/outline`,它一次返回课程 + 章节 + 课时 + 本人进度,不再另设章节/课时列表接口。`POST /lessons/{id}/material` 上传视频或附件、`POST /lessons/{id}/material/access` 换取投放授权(视频 `mode=stream` 可续播,附件 `mode=download` 一次性取件)。
- `/courses/join`、`/members`:选课成员。`POST /courses/{id}/members/batch` 的粒度是一个班级(请求体 `{ class_id }`),学生由 M6 经 `contracts.IdentityService.ListClassStudents` 解析;成员响应带 `student_name`/`student_no`,客户端不再另取账号目录。
- `/assignments/*`、`/submissions/*`:作业/提交/批改(判题调 M3)。`GET /courses/{id}/assignments?page=&size=` 与 `GET /assignments/{id}/submissions` 师生同路由按身份分视角:授课教师见含草稿全量与全班提交,课程内学生只见已发布作业与本人提交;列表统一返回 `{list,total,page,size}` —— 这两条是学生取得作业编号与提交编号的唯一入口。
- `/posts`、`/announcements`、`/review`:讨论/公告/评价。`GET /courses/{id}/announcements?page=&size=` 返回分页信封。
- `/courses/{id}/grades/*`:单课程成绩;`GET /courses/{id}/grades?page=&size=` 返回分页信封,`/grades` 只读契约供 M11 聚合;M6 改分后发布 `teaching.grade.updated` 事件。

### M7 实验 `/api/v1/experiment`
- `/experiments/*`:配置/校验/发布。
- `/student/experiments`、`/student/experiments/{id}`:学生可发现的已发布实验列表与单条读取,统一走学生投影(剔除环境初始化与判题内部配置)。
- `/experiments/{id}/instances`、`/instances/{id}`:实例创建(编排 M2/M4)/工作台/控制;`/instances/{id}/stages/{stage}/activate` 是阶段资源创建唯一写入口;`/instances/{id}/progress` 返回 M10 订阅元信息。**无手动回收接口**:引擎资源释放走 `finish` 内部回收、后台超时回收、以及订阅 `teaching.course.ended` 的课程结束级联三条自动路径。
- `/instances/{id}/checkpoints/{cp}/judge`:检查点判分(调 M3)。
- `/instances/{id}/report`、`/reports/{id}/grade`:报告。
- `/groups/*`:多人协作。`GET /experiments/{id}/groups` 是教师编组视角(全部分组 + 成员角色,不含实例),`GET /groups/{id}` 是按组读单组并附带共享实例。
- `/internal/instances/{id}/score`、`/internal/stats` `[内部]`(供上层聚合/M9;M7 不直接依赖同层 M6)。

### M8 竞赛 `/api/v1/contest`
- `/contests/*`:赛事管理/题目编排/发布开始结束。
- `/student/contests`、`/student/contests/{id}`:学生可发现的非草稿赛事列表与单条读取(门槛与列表一致)。
- `/signup`、`/contests/{id}/join-team`、`/teams/*`:报名组队。加入队伍只按邀请码(队伍编号对学生是内部标识,不进请求)。
- `/problems/{pid}/env`、`/submit`:解题赛(环境调 M2、判题调 M3)。
- `/battle/entry`、`/battle/matches`、`/battle/replay-window`、`/matches/{id}/replay`、`/ladder`:对抗赛/回放/天梯。`GET /contests/{id}/battle/entries?page=&size=` 与 `GET /contests/{id}/battle/matches` 都返回分页信封;matches 师生同路由按身份分视角:赛事组织者与学校管理员见本赛事全部对局(实时监控),其余账号见本队对局;学生回放使用 `GET /contests/{id}/battle/replay-window?page=&size=`，由服务端提供已完成总量、处理中数量、窗口前检查点和窗口内有序事件，不能用通用列表页切片重算全场状态;回放取件仍限参赛队伍成员。
- `/my/contest-records`、`/result-snapshot`:个人战绩。
- `/cheat-*`:防作弊。
- `/vuln-sources/*`、`/vuln-problems/*`:租户漏洞源与漏洞题转化 `[出题教师/学校管理员]`(finalize 调 M5 system-import)。
- `/platform/vuln-sources`:全局漏洞源目录(`tenant_id=0`)`[平台管理员]`,只查看与新增/更新,不代租户同步、不生成租户草稿。
- `/internal/stats`、`/students/{id}/contest-achievements` `[内部]`。

### M9 管理后台 `/api/v1/admin`
- `/platform/dashboard`、`/platform/statistics`:平台看板与统计聚合 `[平台管理员]`。租户列表与入驻申请审核不在 M9,统一走 M1 `/platform/tenants`、`/platform/applications`。
- `/school/dashboard`、`/school/statistics`:学校看板。
- `/audit`、`/audit/export`:统一审计查询中心(查 M1 audit_log)。
- `/configs/*`、`/alert-rules`、`/alert-events/*`:配置/告警。
- `/platform/monitoring/panels`:外接监控嵌入入口。
- `/platform/backups`:备份记录。

### M10 通知与实时推送 `/api/v1/notify`
- `/send` `[内部]`:统一通知发送。
- `/push` `[内部]`:实时推送到带租户前缀的 topic。
- `WS /api/ws`:统一实时通道(订阅 topic)`[租户用户]`;topic 以 `tenant:{tenant_id}` 为根,平台管理员不订阅。
- `/inbox/*`、`/preferences`:站内信/偏好 `[租户用户]`;`notification`、`notification_preference` 表 `tenant_id NOT NULL`,平台管理员无收件箱。`GET /preferences` 返回 `notification_template` 全量类型与本人设置的合并结果(`type/enabled/force`),不返回模板正文。
- `POST /announcements`:发布公告 `[平台管理员/学校管理员]`;`GET /announcements`:公告列表 `[租户用户/平台管理员]`;`POST /announcements/{id}/read`:标记已读 `[租户用户]`。

### M11 成绩中心 `/api/v1/grade-center`
- `/level-configs`、`/semesters`:等级映射/学期。`GET /students/{id}/grades?semester=&page=&size=` 返回包含当前范围 GPA 摘要和 `{list,total,page,size}` 的分页课程成绩明细。
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
| 扩展仿真接入 | 教师提交 bundle → M4 算 sha256 + 静态扫描 + manifest 协议校验 → 隔离容器预览(同 seed 双跑比对 + 渲样例帧)→ 内部签名回写报告 → 平台管理员按样例帧判定 → 上架 |
| 通知 | 模块发事件/调 M10 send → 渲染模板 → 站内信 + M10 push 红点 |
