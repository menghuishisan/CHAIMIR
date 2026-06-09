# runtime/evm-geth

go-ethereum 私链运行时镜像,用于真实节点形态的 EVM 教学实验。

本镜像复用官方 geth 镜像,仅增加平台启动脚本和 manifest 元数据。链状态目录由平台挂载,不得写入镜像层。
