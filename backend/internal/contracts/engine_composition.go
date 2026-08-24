// engine_composition 文件定义实验、竞赛和判题共用的沙箱组合声明与编译快照契约。
package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SandboxAccessProfile 表示组合运行期的访问边界。
type SandboxAccessProfile string

const (
	SandboxAccessExperiment               SandboxAccessProfile = "experiment"
	SandboxAccessContestSolve             SandboxAccessProfile = "contest-solve"
	SandboxAccessContestBattle            SandboxAccessProfile = "contest-battle"
	SandboxAccessVulnerabilityPrevalidate SandboxAccessProfile = "vulnerability-prevalidate"
	SandboxAccessJudgePrivate             SandboxAccessProfile = "judge-private"
)

// Valid 校验访问配置是否属于平台支持的闭集。
func (p SandboxAccessProfile) Valid() bool {
	switch p {
	case SandboxAccessExperiment, SandboxAccessContestSolve, SandboxAccessContestBattle, SandboxAccessVulnerabilityPrevalidate, SandboxAccessJudgePrivate:
		return true
	default:
		return false
	}
}

// CompositionComponentRef 是教师显式选择或编译器自动展开的组件引用。
type CompositionComponentRef struct {
	Code      string         `json:"code"`
	Selection string         `json:"selection"`
	Params    map[string]any `json:"params,omitempty"`
}

// CompositionRuntimeRef 是主运行时及其明确镜像版本。
type CompositionRuntimeRef struct {
	Code         string         `json:"runtime_code"`
	ImageVersion string         `json:"image_version"`
	Params       map[string]any `json:"params,omitempty"`
}

// CompositionLink 是组件之间唯一允许的内网连接声明。
type CompositionLink struct {
	SourceComponent string `json:"source_component"`
	SourceEndpoint  string `json:"source_endpoint"`
	TargetComponent string `json:"target_component"`
	TargetEndpoint  string `json:"target_endpoint"`
	Protocol        string `json:"protocol"`
	RequiredAtStart bool   `json:"required_at_start"`
	AccessScope     string `json:"access_scope"`
	ConfigBinding   string `json:"config_binding"`
}

// SandboxCompositionSpec 是单个环境的唯一声明来源。
type SandboxCompositionSpec struct {
	ID              string                    `json:"id"`
	PrimaryRuntime  CompositionRuntimeRef     `json:"primary_runtime"`
	Infra           []CompositionComponentRef `json:"infra,omitempty"`
	Tools           []CompositionComponentRef `json:"tools,omitempty"`
	Links           []CompositionLink         `json:"links,omitempty"`
	AccessProfile   SandboxAccessProfile      `json:"access_profile"`
	ResourceProfile map[string]string         `json:"resource_profile,omitempty"`
	NetworkProfile  map[string]any            `json:"network_profile,omitempty"`
	InitCodeRef     string                    `json:"init_code_ref,omitempty"`
	InitScriptRef   string                    `json:"init_script_ref,omitempty"`
}

// ImageClosureItem 是已编译快照锁定的镜像地址和版本。
type ImageClosureItem struct {
	Category        string              `json:"category"`
	Code            string              `json:"code"`
	ImageURL        string              `json:"image_url"`
	Version         string              `json:"version"`
	PrepullCommand  []string            `json:"prepull_command"`
	PrepullHold     bool                `json:"prepull_hold"`
	EphemeralMounts []ImageClosureMount `json:"ephemeral_mounts,omitempty"`
}

// ImageClosureMount 是镜像预拉取时所需的临时挂载声明,与运行期工作负载解耦以保持契约层无循环依赖。
type ImageClosureMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
}

// CompiledRuntimeSnapshot 冻结一次发布实际使用的运行时目录行、镜像和适配器规格。
type CompiledRuntimeSnapshot struct {
	RuntimeID      int64           `json:"runtime_id"`
	ImageID        int64           `json:"image_id"`
	Code           string          `json:"code"`
	Eco            string          `json:"eco"`
	AdapterLevel   int16           `json:"adapter_level"`
	CapabilityImpl string          `json:"capability_impl"`
	AdapterSpec    json.RawMessage `json:"adapter_spec"`
	ImageURL       string          `json:"image_url"`
	ImageVersion   string          `json:"image_version"`
}

// CompiledComponentSnapshot 冻结一次发布实际使用的工具或基础设施工作负载规格。
type CompiledComponentSnapshot struct {
	ComponentID  int64           `json:"component_id"`
	Category     string          `json:"category"`
	Code         string          `json:"code"`
	Kind         int16           `json:"kind"`
	ResourceSpec json.RawMessage `json:"resource_spec"`
}

// SandboxCompositionSnapshot 是发布后不可变的组合事实来源。
type SandboxCompositionSnapshot struct {
	Digest       string                      `json:"composition_digest"`
	Spec         SandboxCompositionSpec      `json:"spec"`
	Runtime      CompiledRuntimeSnapshot     `json:"runtime"`
	Components   []CompiledComponentSnapshot `json:"components,omitempty"`
	ImageClosure []ImageClosureItem          `json:"image_closure"`
}

// CanonicalDigest 对规范化组合生成稳定摘要，保证相同声明在不同调用方得到同一 digest。
func CanonicalDigest(spec SandboxCompositionSpec) (string, error) {
	if strings.TrimSpace(spec.ID) == "" || !spec.AccessProfile.Valid() {
		return "", fmt.Errorf("组合标识或访问配置无效")
	}
	if strings.TrimSpace(spec.PrimaryRuntime.Code) == "" || strings.TrimSpace(spec.PrimaryRuntime.ImageVersion) == "" {
		return "", fmt.Errorf("组合必须声明唯一主运行时及镜像版本")
	}
	canonical := normalizeComposition(spec)
	b, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("组合规范化失败: %w", err)
	}
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

// CanonicalSnapshotDigest 对完整编译结果生成摘要，确保目录变化不能静默改变已发布工作负载。
func CanonicalSnapshotDigest(snapshot SandboxCompositionSnapshot) (string, error) {
	if strings.TrimSpace(snapshot.Spec.ID) == "" || !snapshot.Spec.AccessProfile.Valid() {
		return "", fmt.Errorf("组合标识或访问配置无效")
	}
	if snapshot.Runtime.RuntimeID <= 0 || snapshot.Runtime.ImageID <= 0 || strings.TrimSpace(snapshot.Runtime.Code) == "" || len(snapshot.Runtime.AdapterSpec) == 0 {
		return "", fmt.Errorf("组合快照缺少完整运行时执行规格")
	}
	canonical := snapshot
	canonical.Digest = ""
	canonical.Spec = normalizeComposition(snapshot.Spec)
	canonical.Components = append([]CompiledComponentSnapshot(nil), snapshot.Components...)
	canonical.ImageClosure = append([]ImageClosureItem(nil), snapshot.ImageClosure...)
	sort.Slice(canonical.Components, func(i, j int) bool {
		left := canonical.Components[i].Category + "\x00" + canonical.Components[i].Code
		right := canonical.Components[j].Category + "\x00" + canonical.Components[j].Code
		return left < right
	})
	sort.Slice(canonical.ImageClosure, func(i, j int) bool {
		left := imageClosureSortKey(canonical.ImageClosure[i])
		right := imageClosureSortKey(canonical.ImageClosure[j])
		return left < right
	})
	b, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("组合快照规范化失败: %w", err)
	}
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

// imageClosureSortKey 为可能共享同一镜像但使用不同自检命令的组件提供稳定排序。
func imageClosureSortKey(item ImageClosureItem) string {
	encoded, err := json.Marshal(item)
	if err != nil {
		return item.Category + "\x00" + item.Code + "\x00" + item.ImageURL + "\x00" + item.Version
	}
	return string(encoded)
}

// normalizeComposition 只排序无序集合，保留参数和连接的字段含义。
func normalizeComposition(spec SandboxCompositionSpec) SandboxCompositionSpec {
	out := spec
	out.Infra = append([]CompositionComponentRef(nil), spec.Infra...)
	out.Tools = append([]CompositionComponentRef(nil), spec.Tools...)
	out.Links = append([]CompositionLink(nil), spec.Links...)
	sort.Slice(out.Infra, func(i, j int) bool { return out.Infra[i].Code < out.Infra[j].Code })
	sort.Slice(out.Tools, func(i, j int) bool { return out.Tools[i].Code < out.Tools[j].Code })
	sort.Slice(out.Links, func(i, j int) bool {
		left := out.Links[i].SourceComponent + "\x00" + out.Links[i].SourceEndpoint + "\x00" + out.Links[i].TargetComponent + "\x00" + out.Links[i].TargetEndpoint
		right := out.Links[j].SourceComponent + "\x00" + out.Links[j].SourceEndpoint + "\x00" + out.Links[j].TargetComponent + "\x00" + out.Links[j].TargetEndpoint
		return left < right
	})
	return out
}
