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
func genericRuntimeAdapterSpec(manifest platformRuntimeManifest, runtimeImageURL, workspaceImageURL, archiveImageURL string) (map[string]any, error) {
	if strings.TrimSpace(runtimeImageURL) == "" || strings.TrimSpace(workspaceImageURL) == "" || strings.TrimSpace(archiveImageURL) == "" {
		return nil, fmt.Errorf("运行时、工作区和私有归档镜像地址不能为空")
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
	archiveTarget := "sandbox/private-archive"
	initAssetTarget := "sandbox/init-assets"
	env, err := runtimeAdapterEnv(manifest)
	if err != nil {
		return nil, err
	}
	env, err = mergeRuntimeWorkloadEnv(env, manifest.Runtime.Workload.Env)
	if err != nil {
		return nil, err
	}
	secretEnv, err := secretEnvFromManifest(manifest.SecretsRequired, "runtime")
	if err != nil {
		return nil, err
	}
	initContainers, err := normalizeRuntimeInitContainers(manifest.Runtime.Workload.InitContainers, runtimeImageURL)
	if err != nil {
		return nil, err
	}
	studentShell := map[string]any{
		"name": "student-shell", "image_url": workspaceImageURL, "command": []string{"sleep", "2147483647"},
		"env":                       []map[string]any{{"name": "HOME", "value": "/runtime-state/workspace-home"}},
		"resources":                 map[string]any{"requests": map[string]string{"cpu": "50m", "memory": "64Mi"}, "limits": map[string]string{"cpu": "250m", "memory": "256Mi"}},
		"read_only_root_filesystem": true, "labels": map[string]string{"chaimir.io/student-access": "true"}, "mount_workspace": true,
		"ephemeral_mounts": []map[string]any{{"name": "student-shell-tmp", "mount_path": "/tmp"}, {"name": "student-shell-home", "mount_path": "/runtime-state/workspace-home"}},
		"mount_domains":    []string{sandbox.VolumeDomainPublicAssets},
		"prepull_command":  []string{"/usr/local/bin/chaimir-workspace", "selftest"}, "prepull_hold": true,
	}
	privateArchive := map[string]any{
		"name": "private-archive", "image_url": archiveImageURL, "command": []string{"sleep", "2147483647"},
		"resources":                 map[string]any{"requests": map[string]string{"cpu": "50m", "memory": "64Mi"}, "limits": map[string]string{"cpu": "250m", "memory": "256Mi"}},
		"read_only_root_filesystem": true, "labels": map[string]string{"chaimir.io/student-access": "false", "chaimir.io/sensitivity": "judge-private"}, "mount_workspace": false, "run_as_user": int64(1000), "run_as_group": int64(1000),
		"mount_domains": []string{sandbox.VolumeDomainJudgePrivate}, "writable_mount_domains": []string{sandbox.VolumeDomainJudgePrivate},
		"ephemeral_mounts": []map[string]any{{"name": "private-archive-tmp", "mount_path": "/tmp"}},
		"prepull_command":  []string{"tar", "--help"}, "prepull_hold": false,
	}
	initAssetSidecar := map[string]any{
		"name": "init-assets", "image_url": archiveImageURL, "command": []string{"sleep", "2147483647"},
		"resources":                 map[string]any{"requests": map[string]string{"cpu": "50m", "memory": "64Mi"}, "limits": map[string]string{"cpu": "250m", "memory": "256Mi"}},
		"read_only_root_filesystem": true, "labels": map[string]string{"chaimir.io/student-access": "false", "chaimir.io/sensitivity": "runtime-assets"}, "mount_workspace": false, "run_as_user": int64(1000), "run_as_group": int64(1000),
		"mount_domains": []string{"runtime-state", sandbox.VolumeDomainPublicAssets}, "writable_mount_domains": []string{"runtime-state", sandbox.VolumeDomainPublicAssets},
		"ephemeral_mounts": []map[string]any{{"name": "init-assets-tmp", "mount_path": "/tmp"}},
		"prepull_command":  []string{"tar", "--help"}, "prepull_hold": false,
	}
	initAssets := append([]sandbox.InitAssetSpec(nil), manifest.Runtime.InitAssets...)
	infraSidecars := []map[string]any{studentShell, privateArchive}
	if len(initAssets) > 0 {
		infraSidecars = append(infraSidecars, initAssetSidecar)
	}
	return map[string]any{
		"workspace_dir": "/workspace",
		"volume_domains": []map[string]any{
			{"name": "workspace", "mount_path": "/workspace", "student_access": "read_write", "persistence": "object_storage_code", "snapshot_scope": "always"},
			{"name": "public-assets", "mount_path": "/public-assets", "student_access": "read_only", "persistence": "ephemeral", "snapshot_scope": "never"},
			{"name": "runtime-state", "mount_path": "/runtime-state", "student_access": "none", "persistence": "ephemeral", "snapshot_scope": "snapshot_enabled"},
			{"name": "judge-private", "mount_path": "/judge-private", "student_access": "none", "persistence": "ephemeral", "snapshot_scope": "never"},
			{"name": "runtime-tmp", "mount_path": "/tmp", "student_access": "none", "persistence": "ephemeral", "snapshot_scope": "never"},
		},
		"runtime_container": map[string]any{
			"name": "runtime", "image_url": runtimeImageURL, "ports": ports,
			"env":                       env,
			"secret_env":                secretEnv,
			"resources":                 map[string]any{"requests": map[string]string{"cpu": manifest.Resources.CPURequest, "memory": manifest.Resources.MemoryRequest}, "limits": map[string]string{"cpu": manifest.Resources.CPULimit, "memory": manifest.Resources.MemoryLimit}},
			"read_only_root_filesystem": manifest.Security.ReadOnlyRootFilesystem,
			"labels":                    map[string]string{"chaimir.io/student-access": "false"}, "mount_workspace": manifest.Runtime.Native.MountWorkspace,
			"mount_domains": []string{"runtime-state", "public-assets"}, "writable_mount_domains": []string{"runtime-state", "public-assets"},
			"prepull_command": selftest,
		},
		"infra_sidecars": infraSidecars,
		"pods": []map[string]any{
			{"name": "sandbox", "init_containers": initContainers, "containers": []map[string]any{{"name": "runtime"}}},
			{"name": "workspace", "containers": []map[string]any{studentShell}},
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
			"unpack_tar":  []string{"tar", "-C", "{{domain}}", "-xf", "-"},
		},
		"init_asset_ops": map[string]any{
			"exec_target": initAssetTarget,
			"unpack_tar":  []string{"tar", "-C", "{{domain}}", "-xf", "-"},
		},
		"capability_commands": capabilityCommandSet(nativeHelper, nativeTarget, resetStrategy, manifest.Runtime.Native.Actions),
		"init_assets":         initAssets,
		"capabilities":        manifest.Capabilities,
		"selftest":            runtimeSelftestSpec(manifest),
	}, nil
}

// mergeRuntimeWorkloadEnv 合并 manifest 明确声明的运行期非敏感环境变量。
func mergeRuntimeWorkloadEnv(base, extra []workload.EnvVarSpec) ([]workload.EnvVarSpec, error) {
	out := append([]workload.EnvVarSpec(nil), base...)
	index := make(map[string]int, len(out))
	for position, item := range out {
		index[item.Name] = position
	}
	for _, item := range extra {
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			return nil, fmt.Errorf("runtime.workload.env 包含空环境变量名")
		}
		if current, exists := index[item.Name]; exists {
			if out[current].Value != item.Value {
				return nil, fmt.Errorf("runtime.workload.env 与基础环境变量 %s 冲突", item.Name)
			}
			continue
		}
		index[item.Name] = len(out)
		out = append(out, item)
	}
	return out, nil
}

// normalizeRuntimeInitContainers 只接受 manifest 显式声明、使用已证明 digest 的初始化容器。
func normalizeRuntimeInitContainers(items []workload.ComponentSpec, runtimeImageURL string) ([]workload.ComponentSpec, error) {
	out := make([]workload.ComponentSpec, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" || !workload.ValidNonShellCommand(item.Command) || item.PrepullHold {
			return nil, fmt.Errorf("runtime.workload.init_containers 声明无效")
		}
		if _, exists := seen[item.Name]; exists {
			return nil, fmt.Errorf("runtime.workload.init_containers 名称重复: %s", item.Name)
		}
		switch strings.TrimSpace(item.ImageURL) {
		case "@self":
			item.ImageURL = runtimeImageURL
		case "":
			return nil, fmt.Errorf("runtime.workload.init_containers %s 缺少 image_url", item.Name)
		default:
			if !strings.Contains(item.ImageURL, "@sha256:") {
				return nil, fmt.Errorf("runtime.workload.init_containers %s 必须使用不可变镜像 digest", item.Name)
			}
		}
		if item.MountWorkspace == nil || *item.MountWorkspace {
			return nil, fmt.Errorf("runtime.workload.init_containers %s 不得挂载学生工作区", item.Name)
		}
		seen[item.Name] = struct{}{}
		out = append(out, item)
	}
	return out, nil
}

// capabilityCommandSet 仅为声明了 native helper 的 runtime 生成平台原生动作；L1 runtime 保留基础工作负载而不伪造动作。
func capabilityCommandSet(helper, target, resetStrategy string, actions map[string][]string) map[string]any {
	if strings.TrimSpace(helper) == "" {
		return map[string]any{}
	}
	command := func(action string, timeout int) map[string]any {
		argv := append([]string(nil), actions[action]...)
		if len(argv) == 0 {
			argv = []string{helper, action}
		}
		return map[string]any{"exec_target": target, "command": argv, "timeout_seconds": timeout}
	}
	return map[string]any{
		"deploy":         command("deploy", 120),
		"tx":             command("tx", 120),
		"query":          command("query", 60),
		"reset_strategy": resetStrategy,
	}
}

// runtimeCapabilityImpl 只为声明真实 native helper 的 L2/L3 runtime 选择内置执行器。
// L1 runtime 仅提供基础节点能力，不能因为目录初始化而被强行纳入原生动作自检。
func runtimeCapabilityImpl(manifest platformRuntimeManifest) string {
	if manifest.Runtime.AdapterLevel < sandbox.RuntimeAdapterLevelStandard {
		return ""
	}
	if strings.TrimSpace(manifest.Runtime.Native.Helper) == "" {
		return ""
	}
	return sandbox.BuiltinExecCapability
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
func runtimeAdapterEnv(manifest platformRuntimeManifest) ([]workload.EnvVarSpec, error) {
	port := runtimeRPCPort(manifest.Ports)
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	env := []workload.EnvVarSpec{
		{Name: "CHAIMIR_RUNTIME_PROFILE", Value: manifest.Runtime.Native.Profile},
		{Name: "CHAIMIR_CHAIN_RPC_URL", Value: url},
		{Name: "HOME", Value: "/runtime-state/adapter-home"},
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
		env = append(env, workload.EnvVarSpec{Name: name, Value: string(raw)})
	}
	for action, method := range manifest.Runtime.Native.Methods {
		method = strings.TrimSpace(method)
		if method == "" {
			continue
		}
		name := "CHAIMIR_ADAPTER_" + strings.ToUpper(strings.TrimSpace(action)) + "_METHOD"
		env = append(env, workload.EnvVarSpec{Name: name, Value: method})
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
