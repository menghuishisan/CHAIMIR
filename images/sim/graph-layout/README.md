# sim/graph-layout

后端图布局重计算仿真镜像,用于少数 `compute=backend` 的 M4 仿真包。

本镜像从标准输入读取图数据 JSON,输出带坐标的 JSON。它不访问网络、不读取平台数据、不向学生开放 shell。

运行时由 M4 共享 `stdio-json` K8s 适配器调用,能力编号 `graph-layout-stdio`、digest、命令、资源和 I/O 上限统一登记在 `SIM_BACKEND_STDIO_ADAPTERS_JSON`;节点和执行步数使用仿真包已审核的 `scale_limit`。新增其他同协议算法不得复制 Go 适配器。
