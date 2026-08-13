// 生产镜像只携带运行时配置契约占位;Kustomize overlay 必须以同名 ConfigMap 挂载真实值。
// 缺少挂载时保持未配置,前端会闭锁启动而不会意外暴露 SaaS 平台入口。
window.__CHAIMIR_RUNTIME_CONFIG__ = undefined;
