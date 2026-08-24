// 本文件把组合注入的 OP Stack 连接和 JWT Secret 转换为 op-node 官方启动环境。
package main

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {
	for _, name := range []string{"OP_NODE_L1_ETH_RPC", "OP_NODE_L2_ENGINE_RPC", "OP_NODE_L1_BEACON", "OP_NODE_ROLLUP_CONFIG", "CHAIMIR_OP_NODE_L2_ENGINE_JWT"} {
		requiredEnv(name)
	}
	validateURL("OP_NODE_L1_ETH_RPC", os.Getenv("OP_NODE_L1_ETH_RPC"), "http", "https", "ws", "wss")
	validateURL("OP_NODE_L2_ENGINE_RPC", os.Getenv("OP_NODE_L2_ENGINE_RPC"), "http", "https", "ws", "wss")
	validateURL("OP_NODE_L1_BEACON", os.Getenv("OP_NODE_L1_BEACON"), "http", "https")
	rollupConfig := os.Getenv("OP_NODE_ROLLUP_CONFIG")
	info, err := os.Stat(rollupConfig)
	if err != nil || info.IsDir() {
		fail("OP_NODE_ROLLUP_CONFIG must point to a mounted rollup config file")
	}
	jwt := strings.TrimSpace(os.Getenv("CHAIMIR_OP_NODE_L2_ENGINE_JWT"))
	if len(jwt) != 64 {
		fail("CHAIMIR_OP_NODE_L2_ENGINE_JWT must contain 32 bytes as 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(jwt); err != nil {
		fail("CHAIMIR_OP_NODE_L2_ENGINE_JWT is not valid hexadecimal")
	}
	secretPath := "/runtime-state/op-node/jwt.hex"
	if err := os.MkdirAll(filepath.Dir(secretPath), 0700); err != nil {
		fail("create OP Node runtime state: " + err.Error())
	}
	if err := os.WriteFile(secretPath, []byte(jwt+"\n"), 0600); err != nil {
		fail("write OP Node JWT file: " + err.Error())
	}
	if os.Getenv("OP_NODE_RPC_ADDR") == "" {
		if err := os.Setenv("OP_NODE_RPC_ADDR", "0.0.0.0"); err != nil {
			fail("set OP_NODE_RPC_ADDR: " + err.Error())
		}
	}
	if os.Getenv("OP_NODE_RPC_PORT") == "" {
		if err := os.Setenv("OP_NODE_RPC_PORT", "8545"); err != nil {
			fail("set OP_NODE_RPC_PORT: " + err.Error())
		}
	}
	if err := os.Setenv("OP_NODE_L2_ENGINE_AUTH", secretPath); err != nil {
		fail("set OP_NODE_L2_ENGINE_AUTH: " + err.Error())
	}
	if err := syscall.Exec("/usr/local/bin/op-node", append([]string{"op-node"}, os.Args[1:]...), os.Environ()); err != nil {
		fail("exec op-node: " + err.Error())
	}
}

func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		fail(name + " is required")
	}
	return value
}

func validateURL(name, raw string, schemes ...string) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		fail(name + " is invalid")
	}
	for _, scheme := range schemes {
		if parsed.Scheme == scheme {
			return
		}
	}
	fail(fmt.Sprintf("%s uses an unsupported protocol", name))
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
