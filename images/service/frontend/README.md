# service/frontend

前端静态资源服务镜像。

当前仓库的 `frontend/` 目录尚未形成可发布的 React 应用产物,因此本镜像不伪造前端构建步骤。它只接受构建上下文中的 `frontend/dist/` 静态产物目录;缺少该目录时必须构建失败,不得把源码、`node_modules`、测试文件或锁文件复制进 Nginx 发布镜像。
