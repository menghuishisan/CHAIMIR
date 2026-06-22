# judger/testcase-chainmaker

本镜像执行长安链合约测试判题。链配置、证书和隐藏断言通过判题私有卷注入,镜像不向学生暴露这些材料。

当前官方 v2.3 稳定镜像的二进制仍未满足 HIGH/CRITICAL 供应链准入,manifest 暂标记为不可部署;取得可校验源码后应使用受控 Go builder 重建二进制。
