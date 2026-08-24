# runtime/near

本镜像封装 NEAR 本地节点运行时。首次挂载空的 `runtime-state` 时,入口会生成单节点 localnet 配置;已有创世和账户材料不会被覆盖。运行态只在沙箱安全域内使用。
