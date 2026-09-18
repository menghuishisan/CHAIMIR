// 本文件负责在无 shell 的最小 glibc 运行层中启动 Solana test-validator。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// main 校验持久化账本并把信号直接交给 Solana 验证器进程。
func main() {
	ledgerDir := required("CHAIMIR_SOLANA_LEDGER_DIR")
	rpcPort := requiredPort("CHAIMIR_RUNTIME_RPC_PORT")
	// 首次启动时 runtime-state 是空 PVC;由运行时负责创建账本目录,
	// 因此平台自检不需要主机侧 seed。
	if err := os.MkdirAll(ledgerDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "创建 Solana ledger 目录失败: %v\n", err)
		os.Exit(1)
	}

	// 临时文件和 HOME 放在账本同级目录,避免 validator --reset 清理 ledger 时误删。
	stateDir := filepath.Dir(ledgerDir)
	tmpDir := filepath.Join(stateDir, "tmp")
	homeDir := filepath.Join(stateDir, "home")
	for _, dir := range []string{tmpDir, homeDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "创建 Solana 运行目录失败: %v\n", err)
			os.Exit(1)
		}
	}
	if err := os.Setenv("TMPDIR", tmpDir); err != nil {
		fmt.Fprintf(os.Stderr, "设置 Solana 临时目录失败: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv("HOME", homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "设置 Solana HOME 失败: %v\n", err)
		os.Exit(1)
	}

	args := []string{
		"solana-test-validator",
		"--ledger", ledgerDir,
		"--rpc-port", strconv.Itoa(rpcPort),
		"--bind-address", "0.0.0.0",
	}
	if _, err := os.Stat(filepath.Join(ledgerDir, "genesis.bin")); os.IsNotExist(err) {
		args = append(args, "--reset")
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "读取 Solana 创世状态失败: %v\n", err)
		os.Exit(1)
	}

	if err := syscall.Exec("/usr/bin/solana-test-validator", args, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "启动 Solana 验证器失败: %v\n", err)
		os.Exit(1)
	}
}

func required(key string) string {
	value := os.Getenv(key)
	if value == "" {
		fmt.Fprintf(os.Stderr, "%s is required\n", key)
		os.Exit(2)
	}
	return value
}

// requiredPort 在调用 validator 前校验运行时 RPC 端口。
func requiredPort(key string) int {
	value := required(key)
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		fmt.Fprintf(os.Stderr, "%s must be a valid TCP port\n", key)
		os.Exit(2)
	}
	return port
}
