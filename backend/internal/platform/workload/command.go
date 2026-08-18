// workload command 文件提供声明式工作负载共用的 argv 安全校验。
package workload

import (
	"path"
	"strings"
)

var shellExecutables = map[string]struct{}{
	"sh": {}, "bash": {}, "dash": {}, "ash": {}, "zsh": {}, "ksh": {}, "csh": {},
	"cmd": {}, "cmd.exe": {}, "powershell": {}, "powershell.exe": {}, "pwsh": {}, "pwsh.exe": {},
}

// ValidCommand 校验声明式命令为非空 argv 数组且不含流控制字符。
func ValidCommand(command []string) bool {
	if len(command) == 0 {
		return false
	}
	for _, argument := range command {
		if strings.TrimSpace(argument) == "" || strings.ContainsAny(argument, "\x00\r\n") {
			return false
		}
	}
	return true
}

// ValidNonShellCommand 在合法 argv 基础上拒绝 shell 解释器入口。
func ValidNonShellCommand(command []string) bool {
	if !ValidCommand(command) {
		return false
	}
	executable := strings.ToLower(path.Base(strings.TrimSpace(command[0])))
	_, blocked := shellExecutables[executable]
	return !blocked
}
