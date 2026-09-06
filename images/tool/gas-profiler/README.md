# tool/gas-profiler

本镜像提供 gas 消耗分析 CLI,从标准输入读取交易 receipt JSON 并输出数量、总量、最小值、最大值和均值。平台命令策略只允许无参数 `gas-profiler`,调用方通过受控 stdin 传入工作区中的 receipt 内容；它不访问平台控制面。
