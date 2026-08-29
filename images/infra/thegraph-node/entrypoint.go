// 本文件实现 The Graph 入口,校验组合绑定并转换为 graph-node 原生环境变量。
package main

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"syscall"
)

var (
	networkPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	databasePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	userPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	passwordPattern = regexp.MustCompile(`^[A-Za-z0-9_.~-]+$`)
)

func main() {
	rpc := required("CHAIMIR_GRAPH_RPC_URL")
	databaseAddress := required("CHAIMIR_GRAPH_POSTGRES_ADDRESS")
	ipfs := required("CHAIMIR_GRAPH_IPFS_ADDRESS")
	password := required("POSTGRES_PASSWORD")
	network := envOr("CHAIMIR_GRAPH_NETWORK", "local")
	database := envOr("CHAIMIR_GRAPH_POSTGRES_DB", "graph")
	user := envOr("CHAIMIR_GRAPH_POSTGRES_USER", "postgres")
	if !networkPattern.MatchString(network) || !databasePattern.MatchString(database) || !userPattern.MatchString(user) || !passwordPattern.MatchString(password) {
		fail("graph binding contains invalid characters")
	}
	if parsed, err := url.Parse(rpc); err != nil || parsed.Host == "" {
		fail("CHAIMIR_GRAPH_RPC_URL is invalid")
	}
	if parsed, err := url.Parse(databaseAddress); err != nil || parsed.Host == "" {
		fail("CHAIMIR_GRAPH_POSTGRES_ADDRESS is invalid")
	}
	if parsed, err := url.Parse(ipfs); err != nil || parsed.Host == "" {
		fail("CHAIMIR_GRAPH_IPFS_ADDRESS is invalid")
	}
	env := append(os.Environ(),
		"POSTGRES_URL=postgresql://"+user+":"+password+"@"+databaseAddress+"/"+database,
		"ETHEREUM_RPC="+network+":"+rpc,
		"IPFS="+ipfs,
	)
	args := append([]string{"/usr/local/bin/graph-node"}, os.Args[1:]...)
	if err := syscall.Exec(args[0], args, env); err != nil {
		fail("exec graph-node: " + err.Error())
	}
}

func required(name string) string {
	value := os.Getenv(name)
	if value == "" {
		fail(name + " is required")
	}
	return value
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func fail(message string) {
	if _, err := fmt.Fprintln(os.Stderr, message); err != nil {
		os.Exit(2)
	}
	os.Exit(2)
}
