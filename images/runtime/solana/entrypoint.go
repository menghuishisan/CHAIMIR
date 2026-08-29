// 本文件负责在无 shell 的最小 glibc 运行层中启动 Solana test-validator。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// envOrDefault 返回非空环境变量,否则使用运行时默认值。
func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// main 校验持久化账本并把信号直接交给 Solana 验证器进程。
func main() {
	ledgerDir := envOrDefault("CHAIMIR_SOLANA_LEDGER_DIR", "/runtime-state/solana/ledger")
	if info, err := os.Stat(ledgerDir); err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("path is not a directory")
		}
		fmt.Fprintf(os.Stderr, "Solana ledger directory is unavailable: %v\n", err)
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
	if os.Getenv("TMPDIR") == "" {
		if err := os.Setenv("TMPDIR", tmpDir); err != nil {
			fmt.Fprintf(os.Stderr, "设置 Solana 临时目录失败: %v\n", err)
			os.Exit(1)
		}
	}
	if os.Getenv("HOME") == "" {
		if err := os.Setenv("HOME", homeDir); err != nil {
			fmt.Fprintf(os.Stderr, "设置 Solana HOME 失败: %v\n", err)
			os.Exit(1)
		}
	}

	args := []string{
		"solana-test-validator",
		"--ledger", ledgerDir,
		"--rpc-port", envOrDefault("CHAIMIR_RUNTIME_RPC_PORT", "8899"),
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
