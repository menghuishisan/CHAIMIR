// 本文件生成 Kubo 首次初始化使用的受控配置,阻止初始化阶段访问公网 AutoConf。
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/coreiface/options"
)

// main 生成一次性初始化配置;配置包含实例身份,只写入运行时状态并由入口在初始化后删除。
func main() {
	if len(os.Args) != 2 {
		fail("usage: chaimir-ipfs-config-init <output-path>")
	}

	identity, err := config.CreateIdentity(io.Discard, []options.KeyGenerateOption{
		options.Key.Type(options.Ed25519Key),
	})
	if err != nil {
		fail("create node identity: %v", err)
	}

	conf, err := config.InitWithIdentity(identity)
	if err != nil {
		fail("initialize Kubo config: %v", err)
	}

	// Kubo 的默认配置包含 auto 占位符;它们会在官方 init 的 keyspace 初始化阶段
	// 触发公网 AutoConf。平台镜像只允许由显式 CompositionSpec 提供网络联动。
	conf.AutoConf.Enabled = config.False
	conf.Bootstrap = []string{}
	conf.Ipns.DelegatedPublishers = []string{}
	conf.Routing.DelegatedRouters = []string{}
	conf.DNS.Resolvers = map[string]string{}
	conf.Discovery.MDNS.Enabled = false

	data, err := config.Marshal(conf)
	if err != nil {
		fail("marshal Kubo config: %v", err)
	}

	file, err := os.OpenFile(os.Args[1], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		fail("create temporary Kubo config: %v", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		fail("write temporary Kubo config: %v", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		fail("sync temporary Kubo config: %v", err)
	}
	if err := file.Close(); err != nil {
		fail("close temporary Kubo config: %v", err)
	}
}

// fail 将初始化错误写到 stderr 并以非零状态退出,不使用默认配置或降级路径。
func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "chaimir-ipfs-config-init: "+format+"\n", args...)
	os.Exit(1)
}
