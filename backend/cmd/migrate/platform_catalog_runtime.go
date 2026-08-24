// platform_catalog_runtime 定义平台目录使用的声明式运行时插件适配器。
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"chaimir/internal/modules/sandbox"
	"chaimir/internal/platform/workload"
)

// genericRuntimeAdapterSpec 从 runtime manifest 生成统一的可执行适配器。
// 运行时镜像自己提供链能力 helper;平台只声明平台私有工作区 sidecar,
// 不把教师可选工具或 base/chain-tools 伪装成链适配器。
func genericRuntimeAdapterSpec(manifest platformRuntimeManifest, runtimeImageURL, workspaceImageURL string) (map[string]any, error) {
	if strings.TrimSpace(runtimeImageURL) == "" || strings.TrimSpace(workspaceImageURL) == "" {
		return nil, fmt.Errorf("运行时和工作区镜像地址不能为空")
	}
	selftest, err := platformManifestSelftestCommand(manifest.Selftest)
	if err != nil {
		return nil, err
	}
	if len(selftest) == 0 {
		return nil, fmt.Errorf("运行时必须声明 selftest.commands")
	}
	ports := make([]map[string]any, 0, len(manifest.Ports))
	for _, port := range manifest.Ports {
		if strings.TrimSpace(port.Name) == "" || port.ContainerPort <= 0 {
			return nil, fmt.Errorf("运行时端口声明无效")
		}
		protocol := defaultProtocol(port.Protocol)
		ports = append(ports, map[string]any{"name": port.Name, "container_port": port.ContainerPort, "service_port": port.ContainerPort, "protocol": protocol})
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("运行时必须声明至少一个端口")
	}
	nativeHelper := strings.TrimSpace(manifest.Runtime.Native.Helper)
	nativeProfile := strings.TrimSpace(manifest.Runtime.Native.Profile)
	nativeTarget := strings.TrimSpace(manifest.Runtime.Native.ExecTarget)
	resetStrategy := strings.TrimSpace(manifest.Runtime.Native.ResetStrategy)
	if nativeHelper != "" || nativeProfile != "" || nativeTarget != "" || resetStrategy != "" {
		if nativeHelper == "" || nativeProfile == "" || nativeTarget == "" || resetStrategy == "" {
			return nil, fmt.Errorf("运行时 native helper/profile/exec_target/reset_strategy 必须同时声明")
		}
		if !workload.ValidNonShellCommand([]string{nativeHelper}) {
			return nil, fmt.Errorf("运行时 native helper 必须是受控可执行路径")
		}
		if !strings.Contains(nativeTarget, "/") {
			return nil, fmt.Errorf("运行时 native exec_target 必须使用 pod/container")
		}
	}
	archiveTarget := nativeTarget
	if archiveTarget == "" {
		archiveTarget = "sandbox/runtime"
	}
	env, err := runtimeAdapterEnv(manifest)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"workspace_dir": "/workspace",
		"volume_domains": []map[string]any{
			{"name": "workspace", "mount_path": "/workspace", "student_access": "read_write", "persistence": "minio_code", "snapshot_scope": "always"},
			{"name": "runtime-state", "mount_path": "/runtime-state", "student_access": "none", "persistence": "ephemeral", "snapshot_scope": "snapshot_enabled"},
			{"name": "judge-private", "mount_path": "/judge-private", "student_access": "none", "persistence": "ephemeral", "snapshot_scope": "never"},
			{"name": "runtime-tmp", "mount_path": "/tmp", "student_access": "none", "persistence": "ephemeral", "snapshot_scope": "never"},
		},
		"runtime_container": map[string]any{
			"name": "runtime", "image_url": runtimeImageURL, "ports": ports,
			"env":                       env,
			"secret_env":                secretEnvFromManifest(manifest.SecretsRequired),
			"resources":                 map[string]any{"requests": map[string]string{"cpu": manifest.Resources.CPURequest, "memory": manifest.Resources.MemoryRequest}, "limits": map[string]string{"cpu": manifest.Resources.CPULimit, "memory": manifest.Resources.MemoryLimit}},
			"read_only_root_filesystem": manifest.Security.ReadOnlyRootFilesystem,
			"labels":                    map[string]string{"chaimir.io/student-access": "false"}, "mount_workspace": false,
			"prepull_command": selftest,
		},
		"infra_sidecars": []map[string]any{{
			"name": "student-shell", "image_url": workspaceImageURL, "command": []string{"sleep", "2147483647"},
			"env":                       []map[string]any{{"name": "HOME", "value": "/runtime-state/workspace-home"}},
			"resources":                 map[string]any{"requests": map[string]string{"cpu": "50m", "memory": "64Mi"}, "limits": map[string]string{"cpu": "250m", "memory": "256Mi"}},
			"read_only_root_filesystem": true, "labels": map[string]string{"chaimir.io/student-access": "true"}, "mount_workspace": true,
			"ephemeral_mounts": []map[string]any{{"name": "student-shell-tmp", "mount_path": "/tmp"}, {"name": "student-shell-home", "mount_path": "/runtime-state/workspace-home"}},
			"prepull_command":  []string{"/usr/local/bin/chaimir-workspace", "selftest"}, "prepull_hold": true,
		}},
		"pods": []map[string]any{
			{"name": "sandbox", "containers": []map[string]any{{"name": "runtime"}}},
			{"name": "workspace", "containers": []map[string]any{{"name": "student-shell"}}},
		},
		"workspace_ops": map[string]any{
			"exec_target": "workspace/student-shell",
			"read_file":   []string{"/usr/local/bin/chaimir-workspace", "read", "{{workspace}}", "{{path}}"},
			"write_file":  []string{"/usr/local/bin/chaimir-workspace", "write", "{{workspace}}", "{{path}}"},
			"list_files":  []string{"/usr/local/bin/chaimir-workspace", "list", "{{workspace}}", "{{path}}"},
			"pack_tar":    []string{"/usr/local/bin/chaimir-workspace", "pack", "{{workspace}}", "{{path}}"},
			"unpack_tar":  []string{"/usr/local/bin/chaimir-workspace", "unpack", "{{workspace}}", "{{path}}"},
			"run_script":  []string{"/usr/local/bin/chaimir-workspace", "run", "{{workspace}}", "{{workspace}}", "{{script}}"},
			"terminal":    []string{"/usr/local/bin/chaimir-workspace", "terminal", "{{workspace}}"},
			"selftest":    []string{"/usr/local/bin/chaimir-workspace", "selftest"},
		},
		"private_archive_ops": map[string]any{
			"exec_target": archiveTarget,
			"unpack_tar":  []string{"/usr/local/bin/chaimir-workspace", "unpack", "{{domain}}", "{{domain}}"},
		},
		"capability_commands": capabilityCommandSet(nativeHelper, nativeTarget, resetStrategy),
		"capabilities":        manifest.Capabilities,
		"selftest":            runtimeSelftestSpec(manifest),
	}, nil
}

// capabilityCommandSet 仅为声明了 native helper 的 runtime 生成平台原生动作；L1 runtime 保留基础工作负载而不伪造动作。
func capabilityCommandSet(helper, target, resetStrategy string) map[string]any {
	if strings.TrimSpace(helper) == "" {
		return map[string]any{}
	}
	return map[string]any{
		"deploy":         map[string]any{"exec_target": target, "command": []string{helper, "deploy"}, "timeout_seconds": 120},
		"tx":             map[string]any{"exec_target": target, "command": []string{helper, "tx"}, "timeout_seconds": 120},
		"query":          map[string]any{"exec_target": target, "command": []string{helper, "query"}, "timeout_seconds": 60},
		"reset_strategy": resetStrategy,
	}
}

// runtimeManifestCapabilityReason 返回标准链能力清单缺失的明确原因。
// 迁移层不替运行时补写动作或样例数据;缺失时必须保持 onboarding。
func runtimeManifestCapabilityReason(manifest platformRuntimeManifest) string {
	if manifest.Runtime.AdapterLevel < sandbox.RuntimeAdapterLevelStandard {
		return ""
	}
	for _, action := range []string{"deploy", "tx", "query"} {
		if len(manifest.Runtime.Native.Actions[action]) > 0 || strings.TrimSpace(manifest.Runtime.Native.Methods[action]) != "" {
			continue
		}
		return fmt.Sprintf("运行时未声明 native.%s 动作或方法", action)
	}
	deployPayload, deployOK := manifest.CapabilitySelftest["deploy_payload"].(map[string]any)
	if !deployOK || len(deployPayload) == 0 {
		return "运行时未声明 capability_selftest.deploy_payload"
	}
	queryTarget, queryOK := manifest.CapabilitySelftest["query_target"].(string)
	if !queryOK || strings.TrimSpace(queryTarget) == "" {
		return "运行时未声明 capability_selftest.query_target"
	}
	return ""
}

// runtimeSelftestSpec 只复制 manifest 明确声明的能力自检输入,不再生成生态固定 fixture。
func runtimeSelftestSpec(manifest platformRuntimeManifest) map[string]any {
	out := map[string]any{"native_profile": strings.TrimSpace(manifest.Runtime.Native.Profile)}
	for key, value := range manifest.CapabilitySelftest {
		out[key] = value
	}
	return out
}

// runtimeAdapterEnv 将运行时 manifest 的插件 argv/RPC 方法转成受控环境变量,不解释生态语义。
func runtimeAdapterEnv(manifest platformRuntimeManifest) ([]map[string]any, error) {
	port := runtimeRPCPort(manifest.Ports)
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	env := []map[string]any{
		{"name": "CHAIMIR_RUNTIME_PROFILE", "value": manifest.Runtime.Native.Profile},
		{"name": "CHAIMIR_CHAIN_RPC_URL", "value": url},
		{"name": "HOME", "value": "/runtime-state/adapter-home"},
	}
	for action, argv := range manifest.Runtime.Native.Actions {
		if len(argv) == 0 {
			continue
		}
		raw, err := json.Marshal(argv)
		if err != nil {
			return nil, fmt.Errorf("编码运行时 %s 动作 argv 失败: %w", action, err)
		}
		name := "CHAIMIR_ADAPTER_" + strings.ToUpper(strings.TrimSpace(action)) + "_ARGV"
		env = append(env, map[string]any{"name": name, "value": string(raw)})
	}
	for action, method := range manifest.Runtime.Native.Methods {
		method = strings.TrimSpace(method)
		if method == "" {
			continue
		}
		name := "CHAIMIR_ADAPTER_" + strings.ToUpper(strings.TrimSpace(action)) + "_METHOD"
		env = append(env, map[string]any{"name": name, "value": method})
	}
	return env, nil
}

// runtimeRPCPort 按端口名称选择链能力入口,不能把 P2P/共识端口误当成 RPC。
func runtimeRPCPort(ports []toolManifestPort) int32 {
	for _, preferred := range []string{"rpc", "rest", "http"} {
		for _, port := range ports {
			if strings.EqualFold(strings.TrimSpace(port.Name), preferred) && port.ContainerPort > 0 {
				return port.ContainerPort
			}
		}
	}
	for _, port := range ports {
		if port.ContainerPort > 0 {
			return port.ContainerPort
		}
	}
	return 8545
}
