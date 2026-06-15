# backend/api 目录说明

`backend/api` 是 Chaimir 后端对外接口契约目录,用于维护 OpenAPI、proto 等接口描述文件。

## 职责边界

- 本目录只放接口契约和契约维护说明,不放 Gin 路由、service、repo、sqlc、业务实现或装配代码。
- HTTP/WS 运行时入口在 `backend/internal/modules/<module>/api*.go`。
- 模块装配入口在 `backend/cmd/server/<module>.go`。
- 模块间接口、DTO 和事件类型在 `backend/internal/contracts`。
- 基础横切 HTTP 入口如 transfer 属于 `backend/internal/platform/*`,但其对外路径仍要在本目录契约中登记。

## 维护规则

- `openapi.yaml` 以 `docs/总-API接口总览.md`、各模块接口设计文档和当前后端路由实现共同为依据。
- 文档和实现不一致时,先同步 `docs/` 的设计口径,再同步实现与本目录契约。
- 文档未细化请求/响应字段但实现已存在路由时,契约必须先登记路径、方法、鉴权类别和统一响应结构,业务字段用通用对象承载,避免凭空编造字段。
- `[内部]` 接口在模块化单体内优先通过 `internal/contracts` 调用;如以 HTTP 暴露,必须使用服务间鉴权并携带租户、来源和 trace 上下文。
- 错误响应只暴露 `{ code, message, trace_id }`;技术原因只进入日志,不得写入对外 schema。

## 文件组织

- `openapi.yaml`: OpenAPI 根文件,只放全局信息、tags、公共 components,以及对各模块 path 文件的 `$ref`。
- `paths/health.yaml`: 健康探针。
- `paths/identity.yaml`: M1 身份与租户。
- `paths/transfer.yaml`: 基础横切统一导入导出中心。
- `paths/sandbox.yaml` ~ `paths/grade.yaml`: M2-M11 各模块路径。

## 拆分规则

- 新增或修改接口时,优先改对应 `paths/<module>.yaml`,不要把模块路径直接写进根 `openapi.yaml`。
- 根 `openapi.yaml` 只登记 path 到模块文件的引用,例如:
  ```yaml
  /api/v1/sandbox/runtimes:
    $ref: './paths/sandbox.yaml#/api/v1/sandbox/runtimes'
  ```
- 公共响应体、鉴权、通用参数和通用请求体只放根文件 `components`。
- 模块私有 schema 只有在字段已经由文档或 DTO 明确时才允许补充;文档未明确的请求体继续引用公共 `ObjectBody`,避免伪造字段。
