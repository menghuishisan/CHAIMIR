# network/cilium-runtime

Cilium 运行基座从固定 Ubuntu、Cilium LLVM、bpftool、iptables 与源码构建的 gops/CNI loopback/iptables wrapper 组成。Ubuntu 基座的 `/usr/bin/pebble` 经源码与运行依赖审计确认不被 Cilium 使用，且命中高危 Go 漏洞，因此在最终层删除后重新扫描。
