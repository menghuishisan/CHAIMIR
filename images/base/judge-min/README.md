# base/judge-min

判题执行器基础镜像,用于后续 `judger/*` 镜像复用安全基线。

本镜像只提供最小 shell、证书和非 root 用户,不包含具体判题逻辑。具体 EVM/Fabric/FISCO 判题能力必须由对应 `judger/` 镜像明确实现。
