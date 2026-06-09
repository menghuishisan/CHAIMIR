# 本文件定义前端静态资源服务镜像,要求构建上下文中已有 frontend 静态产物。
FROM nginx:1.27-alpine

LABEL org.opencontainers.image.title="service/frontend"
LABEL org.opencontainers.image.description="Chaimir frontend static asset service"

# Nginx 官方镜像默认监听 80;平台内部统一使用容器端口 8080,避免 root 绑定低端口。
RUN addgroup -S chaimir \
    && adduser -S -G chaimir chaimir \
    && sed -i 's/listen       80;/listen       8080;/' /etc/nginx/conf.d/default.conf \
    && chown -R chaimir:chaimir /usr/share/nginx/html /var/cache/nginx /var/run /var/log/nginx

# 这里只复制已有静态产物,不伪造 React 构建流程。
COPY frontend/ /usr/share/nginx/html/

USER chaimir
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
