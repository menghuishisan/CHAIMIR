# service/frontend

前端静态资源服务镜像,在构建阶段使用 `pnpm@9.1.0` 构建学生端、教师端、学校管理端和平台管理端四个 React 应用。

最终运行层只包含 Nginx 配置和静态产物,不复制源码、`node_modules`、测试文件或锁文件。路径分发规则:

- `/` 服务学生端。
- `/teacher/` 服务教师端。
- `/school-admin/` 服务学校管理端。
- `/platform-admin/` 服务平台管理端。

各端均使用 hash 路由和 Nginx SPA fallback,刷新页面不会落到 404。
