# network/cilium-envoy

该镜像是通过统一漏洞门禁的上游 Cilium Envoy 薄封装。保留上游 `cilium-envoy-starter`，由 Chart 在启动后按最小 capability 集合执行权限收敛；不得替换为自制入口脚本。
