// 本文件实现 The Graph 入口,校验组合绑定并转换为 graph-node 原生环境变量。
package main

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var (
	networkPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	databasePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	userPattern     = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	passwordPattern = regexp.MustCompile(`^[A-Za-z0-9_.~-]+$`)
)

func main() {
	rpc := required("CHAIMIR_GRAPH_RPC_URL")
	databaseAddress := required("CHAIMIR_GRAPH_POSTGRES_ADDRESS")
	ipfs := required("CHAIMIR_GRAPH_IPFS_ADDRESS")
	password := required("POSTGRES_PASSWORD")
	poiAccessToken := required("GRAPH_POI_ACCESS_TOKEN")
	network := required("CHAIMIR_GRAPH_NETWORK")
	database := required("CHAIMIR_GRAPH_POSTGRES_DB")
	user := required("CHAIMIR_GRAPH_POSTGRES_USER")
	if !networkPattern.MatchString(network) || !databasePattern.MatchString(database) || !userPattern.MatchString(user) || !passwordPattern.MatchString(password) || strings.TrimSpace(poiAccessToken) != poiAccessToken || strings.ContainsAny(poiAccessToken, "\r\n") {
		fail("graph binding contains invalid characters")
	}
	requireHTTPURL("CHAIMIR_GRAPH_RPC_URL", rpc)
	databaseHostPort, ok := normalizeDatabaseAddress(databaseAddress)
	if !ok {
		fail("CHAIMIR_GRAPH_POSTGRES_ADDRESS is invalid")
	}
	requireHTTPURL("CHAIMIR_GRAPH_IPFS_ADDRESS", ipfs)
	env := append(os.Environ(),
		"POSTGRES_URL=postgresql://"+user+":"+password+"@"+databaseHostPort+"/"+database,
		"ETHEREUM_RPC="+network+":"+rpc,
		"IPFS="+ipfs,
		"GRAPH_POI_ACCESS_TOKEN="+poiAccessToken,
	)
	args := append([]string{"/usr/local/bin/graph-node"}, os.Args[1:]...)
	if err := syscall.Exec(args[0], args, env); err != nil {
		fail("exec graph-node: " + err.Error())
	}
}

// normalizeDatabaseAddress 接收 binding 生成的 TCP endpoint,
// 在构造 URL 前拒绝凭据、路径和其他 scheme。
func normalizeDatabaseAddress(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	if !strings.Contains(value, "://") {
		value = "postgresql://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "postgresql" || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", false
	}
	return parsed.Host, true
}

// requireHTTPURL 在 graph-node 解析前强制执行 HTTP binding 契约。
func requireHTTPURL(name, raw string) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || !hasExplicitPort(raw) {
		fail(name + " is invalid")
	}
	port := parsed.Port()
	if port == "" {
		port = explicitPort(raw)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		fail(name + " port must be in 1-65535")
	}
}

func hasExplicitPort(raw string) bool {
	return explicitPort(raw) != ""
}

func explicitPort(raw string) string {
	separator := strings.Index(raw, "://")
	if separator < 0 {
		return ""
	}
	authority := raw[separator+3:]
	if end := strings.IndexAny(authority, "/?#"); end >= 0 {
		authority = authority[:end]
	}
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		return ""
	}
	colon := strings.LastIndex(authority, ":")
	if colon < 0 || colon == len(authority)-1 {
		return ""
	}
	return authority[colon+1:]
}

func required(name string) string {
	value := os.Getenv(name)
	if value == "" {
		fail(name + " is required")
	}
	return value
}

func fail(message string) {
	if _, err := fmt.Fprintln(os.Stderr, message); err != nil {
		os.Exit(2)
	}
	os.Exit(2)
}
