# runtime/chainmaker

本镜像封装长安链节点运行时。组网、证书和节点配置由 M2 沙箱控制面注入,镜像自身只提供受控节点进程和容器内 RPC 端口契约。

当前官方 v2.3 稳定镜像的二进制仍未满足 HIGH/CRITICAL 供应链准入,manifest 暂标记为不可部署;取得可校验源码后应使用受控 Go builder 重建二进制。
