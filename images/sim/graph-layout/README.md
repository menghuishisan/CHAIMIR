# sim/graph-layout

后端图布局重计算仿真镜像,用于少数 `compute=backend` 的 M4 仿真包。

本镜像从标准输入读取图数据 JSON,输出带坐标的 JSON。它不访问网络、不读取平台数据、不向学生开放 shell。
