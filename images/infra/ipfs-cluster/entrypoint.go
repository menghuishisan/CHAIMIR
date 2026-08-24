// 本文件把 IPFS API 绑定写入 IPFS Cluster 官方 service.json,再以 exec 方式启动 daemon。
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
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
	configPath := filepath.Join(configDir, "service.json")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		fail("read IPFS Cluster service.json: " + err.Error())
	}
	var config map[string]any
	if err := json.Unmarshal(contents, &config); err != nil {
		fail("parse IPFS Cluster service.json: " + err.Error())
	}
	setString(config, "api", "ipfsproxy", "node_multiaddress", multiaddress)
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
