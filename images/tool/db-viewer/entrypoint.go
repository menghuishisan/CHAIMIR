// 本文件把组合编译器注入的数据库地址转换为 pgweb 的受控启动参数。
package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"syscall"
)

func main() {
	address := requiredEnv("CHAIMIR_DATABASE_ADDRESS")
	user := os.Getenv("CHAIMIR_DATABASE_USER")
	if user == "" {
		user = "postgres"
	}
	database := os.Getenv("CHAIMIR_DATABASE_NAME")
	if database == "" {
		database = "postgres"
	}
	password := requiredEnv("POSTGRES_PASSWORD")
	if !safeToken(user) || !safeToken(database) {
		fail("database user/name contains unsupported characters")
	}
	databaseURL := url.URL{Scheme: "postgresql", Host: address, Path: "/" + database, User: url.UserPassword(user, password)}
	if databaseURL.Hostname() == "" {
		fail("CHAIMIR_DATABASE_ADDRESS is invalid")
	}
	args := []string{
		"pgweb",
		"--bind=0.0.0.0",
		"--listen=8081",
		"--url=" + databaseURL.String(),
		"--open-retry=60",
		"--open-retry-delay=2",
	}
	if err := syscall.Exec("/usr/local/bin/pgweb", args, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "exec pgweb: %v\n", err)
		os.Exit(1)
	}
}

func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		fail(name + " is required")
	}
	return value
}

func safeToken(value string) bool {
	for _, char := range value {
		if !(char == '-' || char == '_' || char == '.' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
			return false
		}
	}
	return value != ""
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
