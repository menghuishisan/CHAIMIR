// 本文件实现 IPFS 镜像的最小启动入口,用于首次初始化 repo 并写入安全默认配置。
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const ipfsBin = "/usr/local/bin/ipfs"
const configInitBin = "/usr/local/bin/chaimir-ipfs-config-init"

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
func prepareRepo() (prepareErr error) {
	if err := os.Setenv("IPFS_PATH", "/runtime-state/ipfs"); err != nil {
		return fmt.Errorf("set IPFS_PATH: %w", err)
	}
	if err := os.Setenv("IPFS_TELEMETRY", "off"); err != nil {
		return fmt.Errorf("set IPFS_TELEMETRY: %w", err)
	}

	if _, err := os.Stat(os.Getenv("IPFS_PATH") + "/config"); os.IsNotExist(err) {
		directory, err := os.MkdirTemp("/runtime-state", "ipfs-init-*")
		if err != nil {
			return fmt.Errorf("create temporary init directory: %w", err)
		}
		defer func() {
			if err := os.RemoveAll(directory); err != nil {
				prepareErr = errors.Join(prepareErr, fmt.Errorf("remove temporary init directory: %w", err))
			}
		}()
		initConfigPath := filepath.Join(directory, "config.json")
		if err := runCommand(configInitBin, initConfigPath); err != nil {
			return err
		}
		if err := runCommand(ipfsBin, "init", "--profile=server", initConfigPath); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat repo config: %w", err)
	}

	settings := [][]string{
		{"config", "Addresses.API", "/ip4/0.0.0.0/tcp/5001"},
		{"config", "Addresses.Gateway", "/ip4/0.0.0.0/tcp/8080"},
		{"config", "--json", "AutoConf.Enabled", "false"},
		{"config", "--json", "Bootstrap", "[]"},
		{"config", "--json", "Routing.DelegatedRouters", "[]"},
		{"config", "--json", "Ipns.DelegatedPublishers", "[]"},
		{"config", "--json", "DNS.Resolvers", "{}"},
		{"config", "Plugins.Plugins.telemetry.Config.Mode", "off"},
	}
	for _, setting := range settings {
		if err := runCommand(ipfsBin, setting...); err != nil {
			return err
		}
	}
	return nil
}

// runCommand 执行镜像内受控工具;工具缺失或退出失败都直接返回,不使用兼容入口。
func runCommand(binary string, args ...string) error {
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s %v: %w", binary, args, err)
	}
	return nil
}
