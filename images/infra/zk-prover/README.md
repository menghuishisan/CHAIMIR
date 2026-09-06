# infra/zk-prover

本镜像提供 npm 锁定的 `circom2`/`snarkjs` ZK 证明工具链,可执行文件固定在 `/opt/chaimir/zk-prover/node_modules/.bin/circom2` 和 `/opt/chaimir/zk-prover/node_modules/.bin/snarkjs`,用于在组合编排的受控命令中编译电路、生成 witness、生成 proof 和执行验证。它不内置电路、不提供常驻 HTTP 服务,也不承载生产密钥管理；命令、输入和输出目录必须由 M2 WorkloadSpec 显式注入。
