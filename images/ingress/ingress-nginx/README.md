# ingress/ingress-nginx

ingress-nginx 使用官方上游固定控制器镜像,不重打包。TLS、域名、WAF 和路由策略由部署层 Kustomize/Ingress 资源声明,镜像目录只治理来源、版本、digest、端口、安全和离线导入。

生产不得启用 hostNetwork 或固定 NodePort 绕过平台入口策略。
