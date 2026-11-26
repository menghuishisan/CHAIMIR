// 本文件把组合编译器注入的数据库地址转换为 pgweb 的受控启动参数。
package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	address := requiredEnv("CHAIMIR_DATABASE_ADDRESS")
	user := requiredEnv("CHAIMIR_DATABASE_USER")
	database := requiredEnv("CHAIMIR_DATABASE_NAME")
	password := requiredEnv("POSTGRES_PASSWORD")
	if !safeToken(user) || !safeToken(database) {
		fail("database user/name contains unsupported characters")
	}
	databaseURL, err := url.Parse("postgresql://" + address)
	if err != nil || databaseURL.Hostname() == "" || databaseURL.User != nil || databaseURL.Path != "" || databaseURL.RawQuery != "" || databaseURL.ForceQuery || strings.Contains(address, "#") {
		fail("CHAIMIR_DATABASE_ADDRESS is invalid")
	}
	port, err := strconv.Atoi(databaseURL.Port())
	if err != nil || port < 1 || port > 65535 {
		fail("CHAIMIR_DATABASE_ADDRESS is invalid")
	}
	databaseURL.User = url.UserPassword(user, password)
	databaseURL.Path = "/" + database
	databaseURL.RawQuery = "sslmode=require"
	args := []string{
		"pgweb",
		"--bind=0.0.0.0",
		"--listen=8081",
		"--ssl=require",
		"--url=" + databaseURL.String(),
		"--open-retry=60",
		"--open-retry-delay=2",
	}
	if err := syscall.Exec("/usr/local/bin/pgweb", args, os.Environ()); err != nil {
		fail("exec pgweb: " + err.Error())
	}
}

func requiredEnv(name string) string {
	value := os.Getenv(name)
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
	if _, err := fmt.Fprintln(os.Stderr, message); err != nil {
		os.Exit(2)
	}
	os.Exit(2)
}
