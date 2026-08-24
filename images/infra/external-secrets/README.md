# infra/external-secrets

External Secrets Operator 使用官方固定版本和多架构 digest。部署层通过官方 Helm Chart 安装 CRD、控制器、Webhook 和证书控制器，并在 Helm 后渲染阶段把三个工作负载镜像统一替换为本清单的不可变 digest。

应用不得直接访问密钥源；仅 ExternalSecret 专用身份拥有读取源 Secret 的权限，目标 Pod 继续只引用固定名称 Kubernetes Secret。
