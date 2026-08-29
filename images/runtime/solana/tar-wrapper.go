// 本文件适配 Solana 1.18 固定的 GNU tar sparse 参数与 BusyBox tar 的参数集合。
package main

import (
	"os"
	"strings"
	"syscall"
)

// isTarOptionBundle 判断不带连字符的短选项串,避免误改归档路径或参数值。
func isTarOptionBundle(arg string) bool {
	if arg == "" {
		return false
	}
	for _, r := range arg {
		switch r {
		case 'c', 'x', 't', 'f', 'C', 'v', 'j', 'J', 'z', 'a', 'h', 'm', 'o', 'k', 'O', 'S', 'T', 'X':
		default:
			return false
		}
	}
	return true
}

// normalizeTarArg 移除 BusyBox 不支持但不影响归档内容的 sparse 选项。
func normalizeTarArg(arg string) string {
	if arg == "--sparse" {
		return ""
	}
	if strings.HasPrefix(arg, "-") {
		arg = strings.ReplaceAll(arg, "S", "")
		if arg == "-" {
			return ""
		}
	} else if isTarOptionBundle(arg) {
		// GNU tar 可以把 jcfhS 作为不带连字符的选项串传入。
		arg = strings.ReplaceAll(arg, "S", "")
	}
	return arg
}

// main 以 tar applet 身份执行固定 BusyBox,保持 Solana 的归档/解档调用契约。
func main() {
	args := []string{"tar"}
	for _, arg := range os.Args[1:] {
		if normalized := normalizeTarArg(arg); normalized != "" {
			args = append(args, normalized)
		}
	}
	if err := syscall.Exec("/usr/bin/busybox", args, os.Environ()); err != nil {
		os.Exit(127)
	}
}
