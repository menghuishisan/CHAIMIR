// 本文件把组合注入的 OP Stack 连接和 JWT Secret 转换为 op-node 官方启动环境。
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	for _, name := range []string{"OP_NODE_L1_ETH_RPC", "OP_NODE_L2_ENGINE_RPC", "OP_NODE_L1_BEACON", "OP_NODE_ROLLUP_CONFIG", "OP_NODE_ROLLUP_L1_CHAIN_CONFIG", "OP_NODE_RPC_ADDR", "OP_NODE_RPC_PORT", "OP_GETH_JWT_SECRET"} {
		requiredEnv(name)
	}
	validateURL("OP_NODE_L1_ETH_RPC", os.Getenv("OP_NODE_L1_ETH_RPC"), "http", "https")
	validateURL("OP_NODE_L2_ENGINE_RPC", os.Getenv("OP_NODE_L2_ENGINE_RPC"), "http", "https")
	validateURL("OP_NODE_L1_BEACON", os.Getenv("OP_NODE_L1_BEACON"), "http", "https")
	validatePort("OP_NODE_RPC_PORT", os.Getenv("OP_NODE_RPC_PORT"))
	rollupConfig := os.Getenv("OP_NODE_ROLLUP_CONFIG")
	info, err := os.Stat(rollupConfig)
	if err != nil || info.IsDir() {
		fail("OP_NODE_ROLLUP_CONFIG must point to a mounted rollup config file")
	}
	validateRollupConfig(rollupConfig)
	l1ChainConfig := os.Getenv("OP_NODE_ROLLUP_L1_CHAIN_CONFIG")
	validateL1ChainConfig(l1ChainConfig)
	jwt := strings.TrimSpace(os.Getenv("OP_GETH_JWT_SECRET"))
	if len(jwt) != 64 {
		fail("OP_GETH_JWT_SECRET must contain 32 bytes as 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(jwt); err != nil {
		fail("OP_GETH_JWT_SECRET is not valid hexadecimal")
	}
	secretPath := "/runtime-state/op-node/jwt.hex"
	if err := os.MkdirAll(filepath.Dir(secretPath), 0700); err != nil {
		fail("create OP Node runtime state: " + err.Error())
	}
	if err := os.WriteFile(secretPath, []byte(jwt+"\n"), 0600); err != nil {
		fail("write OP Node JWT file: " + err.Error())
	}
	if err := os.Setenv("OP_NODE_L2_ENGINE_AUTH", secretPath); err != nil {
		fail("set OP_NODE_L2_ENGINE_AUTH: " + err.Error())
	}
	if err := syscall.Exec("/usr/local/bin/op-node", append([]string{"op-node"}, os.Args[1:]...), os.Environ()); err != nil {
		fail("exec op-node: " + err.Error())
	}
}

// validateL1ChainConfig 校验由组合初始化资产注入的 L1 创世配置,确保 op-node 使用同一条受控 L1。
func validateL1ChainConfig(path string) {
	contents, err := os.ReadFile(path)
	if err != nil {
		fail("read OP_NODE_ROLLUP_L1_CHAIN_CONFIG: " + err.Error())
	}
	var envelope struct {
		Config struct {
			ChainID uint64 `json:"chainId"`
		} `json:"config"`
	}
	if err := json.Unmarshal(contents, &envelope); err != nil || envelope.Config.ChainID == 0 {
		fail("OP_NODE_ROLLUP_L1_CHAIN_CONFIG must contain a valid execution genesis config")
	}
}

func validatePort(name, raw string) {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || port < 1 || port > 65535 {
		fail(name + " must be a valid TCP port")
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
	if err != nil || parsed.Hostname() == "" || parsed.Port() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		fail(name + " is invalid")
	}
	port, portErr := strconv.Atoi(parsed.Port())
	if portErr != nil || port < 1 || port > 65535 {
		fail(name + " must use a TCP port in 1-65535")
	}
	for _, scheme := range schemes {
		if parsed.Scheme == scheme {
			return
		}
	}
	fail(fmt.Sprintf("%s uses an unsupported protocol", name))
}

// validateRollupConfig 在 op-node 进入上游配置构造器前拒绝不完整输入,
// 避免缺少 L1 chain ID 时触发上游 panic。
func validateRollupConfig(path string) {
	contents, err := os.ReadFile(path)
	if err != nil {
		fail("read OP_NODE_ROLLUP_CONFIG: " + err.Error())
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(contents, &config); err != nil {
		fail("OP_NODE_ROLLUP_CONFIG must contain valid JSON")
	}
	for _, field := range []string{"l1_chain_id", "l2_chain_id", "genesis"} {
		value, ok := config[field]
		if !ok || len(value) == 0 || string(value) == "null" || string(value) == "{}" || string(value) == "\"\"" {
			fail("OP_NODE_ROLLUP_CONFIG is missing required field: " + field)
		}
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
