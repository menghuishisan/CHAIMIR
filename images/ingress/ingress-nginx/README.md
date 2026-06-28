# ingress/ingress-nginx

ingress-nginx 从官方源码固定版本重建控制器二进制,并复用官方控制器镜像的 Nginx/Lua 运行契约。TLS、域名、WAF 和路由策略由部署层 Kustomize/Ingress 资源声明,镜像目录只治理来源、版本、digest、端口、安全和离线导入。

生产不得启用 hostNetwork 或固定 NodePort 绕过平台入口策略。
