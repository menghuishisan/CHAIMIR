// 本文件实现 IPFS 镜像的最小启动入口,用于首次初始化 repo 并写入安全默认配置。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const ipfsBin = "/usr/local/bin/ipfs"

// main 在启动 daemon 前确保 Kubo repo 已初始化并关闭默认遥测。
func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"daemon", "--migrate=true"}
	}

	if args[0] == "daemon" {
		if err := prepareRepo(); err != nil {
			fmt.Fprintf(os.Stderr, "chaimir-ipfs-entrypoint: %v\n", err)
			os.Exit(1)
		}
	}

	if err := syscall.Exec(ipfsBin, append([]string{ipfsBin}, args...), os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "chaimir-ipfs-entrypoint: exec ipfs: %v\n", err)
		os.Exit(1)
	}
}

// prepareRepo 初始化 Kubo repo,并把 API/Gateway 监听地址与 telemetry 配置收敛到平台要求。
func prepareRepo() error {
	if _, ok := os.LookupEnv("IPFS_PATH"); !ok {
		if err := os.Setenv("IPFS_PATH", "/runtime-state/ipfs"); err != nil {
			return fmt.Errorf("set IPFS_PATH: %w", err)
		}
	}
	if err := os.Setenv("IPFS_TELEMETRY", "off"); err != nil {
		return fmt.Errorf("set IPFS_TELEMETRY: %w", err)
	}

	if _, err := os.Stat(os.Getenv("IPFS_PATH") + "/config"); os.IsNotExist(err) {
		if err := runIPFS("init", "--profile=server"); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat repo config: %w", err)
	}

	settings := [][]string{
		{"config", "Addresses.API", "/ip4/0.0.0.0/tcp/5001"},
		{"config", "Addresses.Gateway", "/ip4/0.0.0.0/tcp/8080"},
		{"config", "Plugins.Plugins.telemetry.Config.Mode", "off"},
	}
	for _, setting := range settings {
		if err := runIPFS(setting...); err != nil {
			return err
		}
	}
	return nil
}

// runIPFS 执行一次受控 ipfs 子命令,失败时保留原始错误链用于定位。
func runIPFS(args ...string) error {
	cmd := exec.Command(ipfsBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ipfs %v: %w", args, err)
	}
	return nil
}
