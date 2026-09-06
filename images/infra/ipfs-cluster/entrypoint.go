// 本文件把 IPFS API 绑定写入 IPFS Cluster 官方 service.json,再以 exec 方式启动 daemon。
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func main() {
	apiURL := os.Getenv("CHAIMIR_IPFS_API_URL")
	if apiURL == "" {
		fail("CHAIMIR_IPFS_API_URL is required")
	}
	parsed, err := url.Parse(apiURL)
	if err != nil || parsed.Hostname() == "" {
		fail("CHAIMIR_IPFS_API_URL is invalid")
	}
	port := parsed.Port()
	if port == "" {
		port = "5001"
	}
	if _, err := strconv.Atoi(port); err != nil {
		fail("CHAIMIR_IPFS_API_URL port is invalid")
	}
	prefix := "dns4"
	if net.ParseIP(parsed.Hostname()) != nil {
		prefix = "ip4"
	}
	multiaddress := fmt.Sprintf("/%s/%s/tcp/%s", prefix, parsed.Hostname(), port)
	configDir := os.Getenv("IPFS_CLUSTER_PATH")
	if configDir == "" {
		configDir = "/runtime-state/ipfs-cluster"
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		fail("initialize IPFS Cluster config directory: " + err.Error())
	}
	configPath := filepath.Join(configDir, "service.json")
	if err := ensureServiceConfig(configPath); err != nil {
		fail("initialize IPFS Cluster service.json: " + err.Error())
	}
	contents, err := os.ReadFile(configPath)
	if err != nil {
		fail("read IPFS Cluster service.json: " + err.Error())
	}
	var config map[string]any
	if err := json.Unmarshal(contents, &config); err != nil {
		fail("parse IPFS Cluster service.json: " + err.Error())
	}
	setString(config, "api", "ipfsproxy", "node_multiaddress", multiaddress)
	// REST、proxy 和 pinning API 都是跨容器 provider endpoint，不能只绑定 loopback。
	setString(config, "api", "restapi", "http_listen_multiaddress", "/ip4/0.0.0.0/tcp/9094")
	setString(config, "api", "ipfsproxy", "listen_multiaddress", "/ip4/0.0.0.0/tcp/9095")
	setString(config, "api", "pinsvcapi", "http_listen_multiaddress", "/ip4/0.0.0.0/tcp/9097")
	setString(config, "ipfs_connector", "ipfshttp", "node_multiaddress", multiaddress)
	updated, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fail("encode IPFS Cluster service.json: " + err.Error())
	}
	if err := os.WriteFile(configPath, append(updated, '\n'), 0600); err != nil {
		fail("write IPFS Cluster service.json: " + err.Error())
	}
	args := append([]string{"ipfs-cluster-service", "daemon"}, os.Args[1:]...)
	if err := syscall.Exec("/usr/local/bin/ipfs-cluster-service", args, os.Environ()); err != nil {
		fail("exec IPFS Cluster: " + err.Error())
	}
}

// ensureServiceConfig 用官方初始化命令在空 runtime-state 卷生成完整 service.json。
func ensureServiceConfig(configPath string) error {
	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat service.json: %w", err)
	}
	command := exec.Command("/usr/local/bin/ipfs-cluster-service", "init")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = os.Environ()
	if err := command.Run(); err != nil {
		return fmt.Errorf("ipfs-cluster-service init: %w", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("initialized service.json is unavailable: %w", err)
	}
	return nil
}

func setString(root map[string]any, first, second, key, value string) {
	level, ok := root[first].(map[string]any)
	if !ok {
		fail("IPFS Cluster service.json missing " + first + "." + second)
	}
	section, ok := level[second].(map[string]any)
	if !ok {
		fail("IPFS Cluster service.json missing " + first + "." + second)
	}
	section[key] = value
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
