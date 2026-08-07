# service/backend

后端模块化单体服务镜像,同时构建 `backend/cmd/server` API 服务和 `backend/cmd/provisioner` 动态命名空间 RBAC 预配器。

本镜像使用 Go 多阶段构建,最终阶段采用 distroless nonroot 运行时。镜像不包含数据库密码、JWT 密钥、对象存储密钥或任何租户配置,运行期配置必须由 K8s ConfigMap/Secret 注入。
