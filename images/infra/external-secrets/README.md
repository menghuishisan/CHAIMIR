# infra/external-secrets

External Secrets Operator 从固定 v2.10.0 源码归档重建，使用 Go 1.26.6、distroless static 运行层和已修复的 `x/net`、`x/text`、`x/crypto` 依赖；最终镜像必须以不可变 digest 通过项目 Trivy 0.72.0（HIGH=0、CRITICAL=0）、SBOM 和 Cosign 门禁。部署层通过官方 Helm Chart 安装 CRD、控制器、Webhook 和证书控制器，并在 Helm 后渲染阶段把三个工作负载镜像统一替换为本清单的不可变 digest。

应用不得直接访问密钥源；仅 ExternalSecret 专用身份拥有读取源 Secret 的权限，目标 Pod 继续只引用固定名称 Kubernetes Secret。
