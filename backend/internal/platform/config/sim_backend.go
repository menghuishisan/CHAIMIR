// config sim_backend 文件定义 M4 stdio-json 后端计算能力目录及其启动期安全校验。
package config

import (
	"fmt"
	"os"
	"strings"

	"chaimir/internal/platform/workload"
	"chaimir/pkg/privacy"

	"k8s.io/apimachinery/pkg/api/resource"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

// SimBackendAdapterConfig 描述一个由部署方登记的 stdio-json 后端计算能力。
type SimBackendAdapterConfig struct {
	Code                  string            `json:"code"`
	Name                  string            `json:"name"`
	Description           string            `json:"description"`
	Protocol              string            `json:"protocol"`
	Image                 string            `json:"image"`
	IdleCommand           []string          `json:"idle_command"`
	Command               []string          `json:"command"`
	Env                   map[string]string `json:"env"`
	CPURequest            string            `json:"cpu_request"`
	CPULimit              string            `json:"cpu_limit"`
	MemoryRequest         string            `json:"memory_request"`
	MemoryLimit           string            `json:"memory_limit"`
	EphemeralStorageLimit string            `json:"ephemeral_storage_limit"`
	ExecTimeoutSeconds    int               `json:"exec_timeout_seconds"`
	MaxInputBytes         int64             `json:"max_input_bytes"`
	MaxOutputBytes        int64             `json:"max_output_bytes"`
}

// SimBackendConfig 描述 M4 隔离执行的共享边界、能力目录与闸门阈值。
type SimBackendConfig struct {
	NamespacePrefix        string
	PodReadyTimeoutSeconds int
	StdioAdapters          []SimBackendAdapterConfig
	// PackageRunnerAdapterCode 是扩展包通用运行器能力编号,服务端按作者类型自动绑定给
	// 教师/第三方包,教师无从选择;它必须存在于 StdioAdapters 中,否则扩展接入整条链路不可用。
	PackageRunnerAdapterCode string
	// MaxConcurrentSessionsPerTenant 限定单租户同时活跃的隔离执行会话数。
	// 一个会话一个 Pod,没有这道闸门循环建会话可耗尽节点。
	MaxConcurrentSessionsPerTenant int
	// PreviewPollIntervalSeconds 是隔离预览任务的轮询间隔。
	PreviewPollIntervalSeconds int
	// PreviewBatchSize 是单次认领的待审包数量上限。
	PreviewBatchSize int
	// PreviewFrameCount 是隔离预览渲制的样例教学帧数量,供平台管理员判断算法实现是否正确。
	PreviewFrameCount int
}

// imageAttestationAllows 确保 M4 只执行供应链证明中已签名且扫描通过的精确 digest 镜像。
func imageAttestationAllows(items []PlatformImageAttestation, imageURL, registry string) bool {
	imageURL = strings.TrimSpace(imageURL)
	registry = strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(registry, "https://"), "http://")), "/")
	digestIndex := strings.LastIndex(imageURL, "@sha256:")
	if imageURL == "" || registry == "" || digestIndex <= 0 || !strings.HasPrefix(imageURL, registry+"/") {
		return false
	}
	digest := imageURL[digestIndex+1:]
	for _, item := range items {
		if strings.TrimSpace(item.ImageURL) == imageURL && strings.TrimSpace(item.Digest) == digest && item.CosignVerified && strings.EqualFold(strings.TrimSpace(item.TrivyStatus), "passed") {
			return true
		}
	}
	return false
}

// readSimBackendAdapters 严格解析部署方登记的 stdio-json 能力目录。
func readSimBackendAdapters(key string, errs *[]string) []SimBackendAdapterConfig {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		*errs = append(*errs, "缺少必填环境变量: "+key)
		return nil
	}
	var out []SimBackendAdapterConfig
	if err := decodeStrictJSON(raw, &out); err != nil {
		*errs = append(*errs, fmt.Sprintf("环境变量 %s 需为严格的后端计算能力 JSON 数组: %v", key, err))
		return nil
	}
	return out
}

// simAdapterRegistered 判定给定能力编号是否已登记在能力目录中。
func simAdapterRegistered(items []SimBackendAdapterConfig, code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item.Code) == code {
			return true
		}
	}
	return false
}

// validateSimBackendAdapters 在启动边界统一校验能力编号、镜像证明、命令和资源限制。
func validateSimBackendAdapters(items []SimBackendAdapterConfig, attestations []PlatformImageAttestation, registry string) []string {
	if len(items) == 0 {
		return []string{"SIM_BACKEND_STDIO_ADAPTERS_JSON 至少登记一个生产能力"}
	}
	seen := make(map[string]struct{}, len(items))
	var errs []string
	for index, item := range items {
		prefix := fmt.Sprintf("SIM_BACKEND_STDIO_ADAPTERS_JSON 第 %d 项", index)
		code := strings.TrimSpace(item.Code)
		if problems := k8svalidation.IsDNS1123Label(code); len(problems) > 0 {
			errs = append(errs, prefix+" code 必须是可用作 Kubernetes label 的 DNS-1123 名称")
		}
		if _, exists := seen[code]; code != "" && exists {
			errs = append(errs, prefix+" code 重复: "+code)
		}
		seen[code] = struct{}{}
		if strings.TrimSpace(item.Name) == "" || len([]rune(item.Name)) > 64 {
			errs = append(errs, prefix+" name 必须为 1 到 64 个字符")
		}
		if strings.TrimSpace(item.Description) == "" || len([]rune(item.Description)) > 240 {
			errs = append(errs, prefix+" description 必须为 1 到 240 个字符")
		}
		if item.Protocol != "stdio-json" {
			errs = append(errs, prefix+" protocol 必须为 stdio-json")
		}
		if !imageAttestationAllows(attestations, item.Image, registry) {
			errs = append(errs, prefix+" image 必须使用完整 digest 并命中已签名且扫描通过的镜像证明")
		}
		errs = append(errs, validateSimBackendCommand(prefix+" idle_command", item.IdleCommand)...)
		errs = append(errs, validateSimBackendCommand(prefix+" command", item.Command)...)
		for key, value := range item.Env {
			if problems := k8svalidation.IsEnvVarName(key); len(problems) > 0 || len(value) > 4096 || strings.ContainsRune(value, '\x00') {
				errs = append(errs, prefix+" env 包含无效名称或值")
			}
			if privacy.IsCredentialKey(key) {
				errs = append(errs, prefix+" env 不得携带密码、token、私钥或其他凭据")
			}
		}
		if item.ExecTimeoutSeconds <= 0 || item.MaxInputBytes <= 0 || item.MaxOutputBytes <= 0 {
			errs = append(errs, prefix+" 执行超时和输入输出上限必须大于 0")
		}
		errs = append(errs, validateSimBackendResources(prefix, item)...)
	}
	return errs
}

// validateSimBackendResources 校验 requests/limits 格式和大小关系。
func validateSimBackendResources(prefix string, item SimBackendAdapterConfig) []string {
	quantities := map[string]string{
		"cpu_request": item.CPURequest, "cpu_limit": item.CPULimit,
		"memory_request": item.MemoryRequest, "memory_limit": item.MemoryLimit,
		"ephemeral_storage_limit": item.EphemeralStorageLimit,
	}
	parsed := make(map[string]resource.Quantity, len(quantities))
	var errs []string
	for name, value := range quantities {
		quantity, err := resource.ParseQuantity(value)
		if err != nil || quantity.Sign() <= 0 {
			errs = append(errs, fmt.Sprintf("%s %s 必须为大于 0 的 Kubernetes quantity", prefix, name))
			continue
		}
		parsed[name] = quantity
	}
	if request, ok := parsed["cpu_request"]; ok {
		if limit, exists := parsed["cpu_limit"]; exists && request.Cmp(limit) > 0 {
			errs = append(errs, prefix+" cpu_request 不得大于 cpu_limit")
		}
	}
	if request, ok := parsed["memory_request"]; ok {
		if limit, exists := parsed["memory_limit"]; exists && request.Cmp(limit) > 0 {
			errs = append(errs, prefix+" memory_request 不得大于 memory_limit")
		}
	}
	return errs
}

// validateSimBackendCommand 校验容器启动或算法执行命令,禁止空参数和 PATH 隐式解析。
func validateSimBackendCommand(name string, command []string) []string {
	if len(command) > 32 || !workload.ValidNonShellCommand(command) || !strings.HasPrefix(command[0], "/") {
		return []string{name + " 必须包含 1 到 32 个非 shell 参数且入口使用绝对路径"}
	}
	var errs []string
	for index, value := range command {
		if strings.TrimSpace(value) == "" || len(value) > 4096 || strings.ContainsRune(value, '\x00') {
			errs = append(errs, fmt.Sprintf("%s 第 %d 个参数无效", name, index))
		}
	}
	return errs
}
