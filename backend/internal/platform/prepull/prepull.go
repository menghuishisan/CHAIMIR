// prepull 文件定义平台目录和沙箱引擎共用的预拉取闭包比较规则。
package prepull

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"chaimir/internal/platform/workload"
)

// ImageSpec 描述一个镜像的真实预拉取与最小自检声明。
type ImageSpec struct {
	ImageURL        string
	Command         []string
	Hold            bool
	EphemeralMounts []workload.EphemeralMountSpec
}

// ToolDefinition 是工具对指定生态的预拉取贡献,只包含共享比较所需字段。
type ToolDefinition struct {
	Available      bool
	Components     []workload.ComponentSpec
	PrepullCommand []string
}

// RuntimeDefinition 是运行时对预拉取闭包有影响的最小声明。
type RuntimeDefinition struct {
	RuntimeContainerName  string
	RuntimePrepullCommand []string
	InfraSidecars         []workload.ComponentSpec
	Pods                  []workload.PodSpec
}

// RuntimeDefinitionFromJSON 从持久化适配器声明读取预拉取所需字段。
func RuntimeDefinitionFromJSON(raw []byte) (RuntimeDefinition, error) {
	var stored struct {
		RuntimeContainer workload.ComponentSpec   `json:"runtime_container"`
		InfraSidecars    []workload.ComponentSpec `json:"infra_sidecars"`
		Pods             []workload.PodSpec       `json:"pods"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		return RuntimeDefinition{}, fmt.Errorf("解析运行时预拉取声明失败: %w", err)
	}
	return RuntimeDefinition{
		RuntimeContainerName:  stored.RuntimeContainer.Name,
		RuntimePrepullCommand: stored.RuntimeContainer.PrepullCommand,
		InfraSidecars:         stored.InfraSidecars,
		Pods:                  stored.Pods,
	}, nil
}

// RuntimeImageSpecs 汇总运行时自身与辅助组件的预拉取闭包。
func RuntimeImageSpecs(imageURL string, definition RuntimeDefinition) ([]ImageSpec, error) {
	collector := newCollector(1 + len(definition.InfraSidecars) + len(definition.Pods))
	if err := collector.add(imageURL, definition.RuntimePrepullCommand, false, nil); err != nil {
		return nil, err
	}
	for _, component := range definition.InfraSidecars {
		if err := collector.add(component.ImageURL, component.PrepullCommand, component.PrepullHold, component.EphemeralMounts); err != nil {
			return nil, err
		}
	}
	for _, pod := range definition.Pods {
		for _, component := range pod.Containers {
			if strings.TrimSpace(component.Name) == strings.TrimSpace(definition.RuntimeContainerName) {
				continue
			}
			if err := collector.add(component.ImageURL, component.PrepullCommand, component.PrepullHold, component.EphemeralMounts); err != nil {
				return nil, err
			}
		}
	}
	return collector.specs, nil
}

// RuntimeDefinitionsChanged 比较两个运行时声明产生的完整预拉取闭包。
func RuntimeDefinitionsChanged(previous, current RuntimeDefinition) (bool, error) {
	previousSpecs, err := RuntimeImageSpecs("runtime-image", previous)
	if err != nil {
		return false, err
	}
	currentSpecs, err := RuntimeImageSpecs("runtime-image", current)
	if err != nil {
		return false, err
	}
	return !Equivalent(previousSpecs, currentSpecs), nil
}

// Equivalent 比较两个预拉取闭包,忽略声明顺序但保留镜像、命令、常驻用途和挂载语义。
func Equivalent(left, right []ImageSpec) bool {
	return reflect.DeepEqual(canonical(left), canonical(right))
}

// ToolDefinitionsChanged 比较已声明工具集合的全部预拉取贡献,不按生态标签过滤。
func ToolDefinitionsChanged(previous, current []ToolDefinition) (bool, error) {
	previousSpecs, err := collectToolSpecs(previous)
	if err != nil {
		return false, err
	}
	currentSpecs, err := collectToolSpecs(current)
	if err != nil {
		return false, err
	}
	return !Equivalent(previousSpecs, currentSpecs), nil
}

// collectToolSpecs 汇总调用方已选且可用的工具预拉取贡献。
func collectToolSpecs(tools []ToolDefinition) ([]ImageSpec, error) {
	collector := newCollector(len(tools))
	for _, tool := range tools {
		if !tool.Available {
			continue
		}
		for _, component := range tool.Components {
			command := component.PrepullCommand
			if len(command) == 0 {
				command = tool.PrepullCommand
			}
			if err := collector.add(component.ImageURL, command, component.PrepullHold, component.EphemeralMounts); err != nil {
				return nil, err
			}
		}
	}
	return collector.specs, nil
}

type collector struct {
	seen  map[string]int
	specs []ImageSpec
}

// newCollector 创建保持首次声明顺序的预拉取集合收集器。
func newCollector(capacity int) *collector {
	return &collector{seen: make(map[string]int), specs: make([]ImageSpec, 0, capacity)}
}

// add 合并相同镜像、命令和用途的临时挂载,并拒绝无自检命令的镜像。
func (c *collector) add(imageURL string, command []string, hold bool, mounts []workload.EphemeralMountSpec) error {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return nil
	}
	command = compactCommand(command)
	if len(command) == 0 {
		return fmt.Errorf("预拉取镜像 %s 缺少自检命令", imageURL)
	}
	key := imageSpecKey(imageURL, command, hold)
	if index, exists := c.seen[key]; exists {
		c.specs[index].EphemeralMounts = mergeMounts(c.specs[index].EphemeralMounts, mounts)
		return nil
	}
	c.seen[key] = len(c.specs)
	c.specs = append(c.specs, ImageSpec{ImageURL: imageURL, Command: command, Hold: hold, EphemeralMounts: mergeMounts(nil, mounts)})
	return nil
}

// canonical 把预拉取集合归一化为可稳定比较的顺序。
func canonical(specs []ImageSpec) []ImageSpec {
	out := make([]ImageSpec, 0, len(specs))
	for _, spec := range specs {
		item := ImageSpec{ImageURL: strings.TrimSpace(spec.ImageURL), Command: append([]string(nil), compactCommand(spec.Command)...), Hold: spec.Hold}
		item.EphemeralMounts = mergeMounts(nil, spec.EphemeralMounts)
		sort.Slice(item.EphemeralMounts, func(i, j int) bool {
			return item.EphemeralMounts[i].Name+"\x00"+item.EphemeralMounts[i].MountPath < item.EphemeralMounts[j].Name+"\x00"+item.EphemeralMounts[j].MountPath
		})
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return imageSpecKey(out[i].ImageURL, out[i].Command, out[i].Hold) < imageSpecKey(out[j].ImageURL, out[j].Command, out[j].Hold)
	})
	return out
}

// imageSpecKey 生成包含镜像、命令和常驻用途的比较键。
func imageSpecKey(imageURL string, command []string, hold bool) string {
	return fmt.Sprintf("%s\x00%s\x00%t", imageURL, strings.Join(compactCommand(command), "\x00"), hold)
}

// mergeMounts 合并并去重有效的临时目录挂载。
func mergeMounts(base, extra []workload.EphemeralMountSpec) []workload.EphemeralMountSpec {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make([]workload.EphemeralMountSpec, 0, len(base)+len(extra))
	seen := map[string]struct{}{}
	for _, mount := range append(append([]workload.EphemeralMountSpec(nil), base...), extra...) {
		name, mountPath := strings.TrimSpace(mount.Name), strings.TrimSpace(mount.MountPath)
		if name == "" || mountPath == "" {
			continue
		}
		key := name + "\x00" + mountPath
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, workload.EphemeralMountSpec{Name: name, MountPath: mountPath})
	}
	return out
}

// compactCommand 移除命令中的空参数,避免等价声明出现伪差异。
func compactCommand(command []string) []string {
	out := make([]string, 0, len(command))
	for _, part := range command {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
