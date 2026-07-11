# service/frontend

前端静态资源服务镜像,在构建阶段使用 `pnpm@9.1.0` 构建 `frontend/apps/web` 唯一 React SPA。学生、教师、学校管理员和平台管理员共享同一份静态产物,由应用内角色路由与权限守卫分流。

最终运行层只包含 Nginx 配置和静态产物,不复制源码、`node_modules`、测试文件或锁文件。应用路径包括:

- `/student/` 学生功能。
- `/teacher/` 服务教师端。
- `/school-admin/` 服务学校管理端。
- `/platform-admin/` 服务平台管理端。

前端使用 Browser History 路由。Nginx 对不存在的静态路径统一回退根 `index.html`,所有角色深链接刷新都由同一 SPA 接管。
