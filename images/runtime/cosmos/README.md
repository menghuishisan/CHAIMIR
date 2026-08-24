# runtime/cosmos

本镜像封装 Cosmos SDK/Gaia 单链运行时。首次挂载空的 `runtime-state` 时,入口会幂等生成单节点创世、验证者和本地链配置,并显式设置最低 gas price;已有运行态不会被覆盖。运行目录由 M2 以独立安全域挂载,学生不能直接读取。
