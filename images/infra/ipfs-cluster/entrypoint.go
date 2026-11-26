// 本文件把 IPFS API 绑定写入 IPFS Cluster 官方 service.json,再以 exec 方式启动 daemon。
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode"
)

func main() {
	apiURL := os.Getenv("CHAIMIR_IPFS_API_URL")
	if apiURL == "" {
		fail("CHAIMIR_IPFS_API_URL is required")
	}
	parsed, err := url.Parse(apiURL)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.ContainsAny(apiURL, "?#") {
		fail("CHAIMIR_IPFS_API_URL is invalid")
	}
	clusterSecret := os.Getenv("CHAIMIR_IPFS_CLUSTER_SECRET")
	if clusterSecret != strings.TrimSpace(clusterSecret) {
		fail("CHAIMIR_IPFS_CLUSTER_SECRET must not contain surrounding whitespace")
	}
	if len(clusterSecret) != 64 {
		fail("CHAIMIR_IPFS_CLUSTER_SECRET must be 32 bytes encoded as 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(clusterSecret); err != nil {
		fail("CHAIMIR_IPFS_CLUSTER_SECRET must be hexadecimal")
	}
	if err := os.Unsetenv("CHAIMIR_IPFS_CLUSTER_SECRET"); err != nil {
		fail("remove cluster secret from child environment: " + err.Error())
	}
	port := parsed.Port()
	if port == "" {
		fail("CHAIMIR_IPFS_API_URL port is required")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		fail("CHAIMIR_IPFS_API_URL port is invalid")
	}
	port = strconv.Itoa(portNumber)
	prefix := "dns4"
	if ip := net.ParseIP(parsed.Hostname()); ip != nil {
		if ip.To4() != nil {
			prefix = "ip4"
		} else {
			prefix = "ip6"
		}
	}
	multiaddress := fmt.Sprintf("/%s/%s/tcp/%s", prefix, parsed.Hostname(), port)
	configDir := "/runtime-state/ipfs-cluster"
	if err := os.MkdirAll(configDir, 0700); err != nil {
		fail("initialize IPFS Cluster config directory: " + err.Error())
	}
	// 官方 init 子命令只能通过 IPFS_CLUSTER_PATH 选择配置根目录；由唯一入口
	// 固定为 runtime-state，避免把工作目录或外部未声明环境键当作状态边界。
	if err := os.Setenv("IPFS_CLUSTER_PATH", configDir); err != nil {
		fail("set IPFS Cluster config directory: " + err.Error())
	}
	configPath := filepath.Join(configDir, "service.json")
	if err := ensureServiceConfig(configPath); err != nil {
		fail("initialize IPFS Cluster service.json: " + err.Error())
	}
	identityID, err := readIdentityID(filepath.Join(configDir, "identity.json"))
	if err != nil {
		fail("read IPFS Cluster identity: " + err.Error())
	}
	contents, err := os.ReadFile(configPath)
	if err != nil {
		fail("read IPFS Cluster service.json: " + err.Error())
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	var config map[string]any
	if err := decoder.Decode(&config); err != nil {
		fail("parse IPFS Cluster service.json: " + err.Error())
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		fail("IPFS Cluster service.json must contain one JSON document")
	}
	configSection(config, "cluster")["secret"] = clusterSecret
	trustedPeers, err := trustedPeerIDs(identityID, os.Getenv("CHAIMIR_IPFS_CLUSTER_TRUSTED_PEERS"))
	if err != nil {
		fail("IPFS Cluster trusted peers are invalid: " + err.Error())
	}
	configSection(config, "consensus", "crdt")["trusted_peers"] = trustedPeers
	configSection(config, "api", "ipfsproxy")["node_multiaddress"] = multiaddress
	// REST、proxy 和 pinning API 都是跨容器 provider endpoint，不能只绑定 loopback。
	configSection(config, "api", "restapi")["http_listen_multiaddress"] = "/ip4/0.0.0.0/tcp/9094"
	configSection(config, "api", "ipfsproxy")["listen_multiaddress"] = "/ip4/0.0.0.0/tcp/9095"
	configSection(config, "api", "pinsvcapi")["http_listen_multiaddress"] = "/ip4/0.0.0.0/tcp/9097"
	configSection(config, "ipfs_connector", "ipfshttp")["node_multiaddress"] = multiaddress
	if err := writeServiceConfig(configPath, config); err != nil {
		fail("write IPFS Cluster service.json: " + err.Error())
	}
	args := append([]string{"ipfs-cluster-service", "daemon"}, os.Args[1:]...)
	env := os.Environ()
	if err := syscall.Exec("/usr/local/bin/ipfs-cluster-service", args, env); err != nil {
		fail("exec IPFS Cluster: " + err.Error())
	}
}

// writeServiceConfig 只在完整写入并关闭成功后替换官方配置，避免中断启动损坏上一份状态。
func writeServiceConfig(configPath string, config map[string]any) (writeErr error) {
	directory := filepath.Dir(configPath)
	temporary, err := os.CreateTemp(directory, ".service-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if temporaryPath != "" {
			writeErr = errors.Join(writeErr, os.Remove(temporaryPath))
		}
	}()
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return errors.Join(err, temporary.Close())
	}
	if err := temporary.Sync(); err != nil {
		return errors.Join(err, temporary.Close())
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, configPath); err != nil {
		return err
	}
	temporaryPath = ""
	return nil
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

// readIdentityID 读取官方初始化生成的 peer ID，禁止以临时或通配身份启动集群。
func readIdentityID(identityPath string) (string, error) {
	contents, err := os.ReadFile(identityPath)
	if err != nil {
		return "", fmt.Errorf("read identity.json: %w", err)
	}
	var identity struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(contents, &identity); err != nil {
		return "", fmt.Errorf("parse identity.json: %w", err)
	}
	identityID := strings.TrimSpace(identity.ID)
	if identityID == "" || identityID == "*" {
		return "", fmt.Errorf("identity.json does not contain a valid peer ID")
	}
	return identityID, nil
}

// trustedPeerIDs 将组合注入的显式 peer 列表转换为官方 CRDT 配置；未声明时仅信任自身 identity。
func trustedPeerIDs(identityID, raw string) ([]string, error) {
	parts := []string{}
	for _, item := range strings.Split(raw, ",") {
		if strings.IndexFunc(item, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("peer IDs cannot contain control characters")
		}
		peerID := strings.TrimSpace(item)
		if peerID != "" {
			parts = append(parts, peerID)
		}
	}
	if len(parts) == 0 {
		parts = []string{identityID}
	}
	seen := make(map[string]struct{}, len(parts))
	for _, peerID := range parts {
		if peerID == "*" {
			return nil, fmt.Errorf("wildcard or control characters are not allowed")
		}
		if _, exists := seen[peerID]; exists {
			return nil, fmt.Errorf("duplicate peer ID %q", peerID)
		}
		seen[peerID] = struct{}{}
	}
	return parts, nil
}

// configSection 定位官方配置分组，结构不符时拒绝覆盖已有状态。
func configSection(root map[string]any, path ...string) map[string]any {
	for index, key := range path {
		section, ok := root[key].(map[string]any)
		if !ok || section == nil {
			fail("IPFS Cluster service.json missing " + strings.Join(path[:index+1], "."))
		}
		root = section
	}
	return root
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
