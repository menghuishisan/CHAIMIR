# ingress/ingress-nginx

ingress-nginx 使用 Wolfi 固定版本包构建,控制器来源固定到 Chainguard 维护分支的明确提交,并保持上游 UID 101/GID 82、Nginx/Lua 目录和 admission webhook 运行契约。TLS、域名、WAF 和路由策略由部署层 Helm values 与 Kustomize/Ingress 资源声明,镜像目录只治理来源、版本、digest、端口、安全和离线导入。

生产不得启用 hostNetwork 或固定 NodePort 绕过平台入口策略。
