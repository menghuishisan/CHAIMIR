// Package prepull 统一预拉取闭包的收集、校验与比较。
package prepull

import (
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

// RuntimeDefinition 是运行时对预拉取闭包有影响的最小声明。
type RuntimeDefinition struct {
	RuntimeContainerName  string
	RuntimePrepullCommand []string
	InfraSidecars         []workload.ComponentSpec
	Pods                  []workload.PodSpec
}

// RuntimeImageSpecs 汇总运行时自身与辅助组件的预拉取闭包。
func RuntimeImageSpecs(imageURL string, definition RuntimeDefinition) ([]ImageSpec, error) {
	specs := []ImageSpec{{ImageURL: imageURL, Command: definition.RuntimePrepullCommand}}
	add := func(component workload.ComponentSpec) {
		specs = append(specs, ImageSpec{ImageURL: component.ImageURL, Command: component.PrepullCommand, Hold: component.PrepullHold, EphemeralMounts: component.EphemeralMounts})
	}
	for _, component := range definition.InfraSidecars {
		add(component)
	}
	for _, pod := range definition.Pods {
		for _, component := range pod.InitContainers {
			add(component)
		}
		for _, component := range pod.Containers {
			if strings.TrimSpace(component.Name) == strings.TrimSpace(definition.RuntimeContainerName) {
				continue
			}
			add(component)
		}
	}
	return Collect(specs)
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

// Collect 校验并合并预拉取声明，保留首次出现顺序，不改写输入。
func Collect(specs []ImageSpec) ([]ImageSpec, error) {
	out := make([]ImageSpec, 0, len(specs))
	seen := make(map[string]int, len(specs))
	for _, spec := range specs {
		imageURL := strings.TrimSpace(spec.ImageURL)
		if imageURL == "" {
			continue
		}
		command := compactCommand(spec.Command)
		if len(command) == 0 {
			return nil, fmt.Errorf("预拉取镜像 %s 缺少自检命令", imageURL)
		}
		key := imageSpecKey(imageURL, command, spec.Hold)
		if index, exists := seen[key]; exists {
			out[index].EphemeralMounts = mergeMounts(out[index].EphemeralMounts, spec.EphemeralMounts)
			continue
		}
		seen[key] = len(out)
		out = append(out, ImageSpec{ImageURL: imageURL, Command: command, Hold: spec.Hold, EphemeralMounts: mergeMounts(nil, spec.EphemeralMounts)})
	}
	return out, nil
}

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

func imageSpecKey(imageURL string, command []string, hold bool) string {
	return fmt.Sprintf("%s\x00%s\x00%t", imageURL, strings.Join(compactCommand(command), "\x00"), hold)
}

func mergeMounts(base, extra []workload.EphemeralMountSpec) []workload.EphemeralMountSpec {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	var out []workload.EphemeralMountSpec
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

func compactCommand(command []string) []string {
	out := make([]string, 0, len(command))
	for _, part := range command {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
