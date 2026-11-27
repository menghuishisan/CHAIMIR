// Package prepull 统一预拉取闭包的收集、校验与比较。
package prepull

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
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
	RuntimeContainer workload.ComponentSpec
	InfraSidecars    []workload.ComponentSpec
	Pods             []workload.PodSpec
}

// RuntimeImageSpecs 汇总运行时自身与辅助组件的预拉取闭包。
func RuntimeImageSpecs(imageURL string, definition RuntimeDefinition) ([]ImageSpec, error) {
	main := definition.RuntimeContainer
	specs := []ImageSpec{{ImageURL: imageURL, Command: main.PrepullCommand, Hold: main.PrepullHold, EphemeralMounts: main.EphemeralMounts}}
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
			if strings.TrimSpace(component.Name) == strings.TrimSpace(main.Name) {
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

// Collect 校验并去重相同执行环境的自检，保留首次出现顺序，不改写输入。
func Collect(specs []ImageSpec) ([]ImageSpec, error) {
	out := make([]ImageSpec, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		imageURL := strings.TrimSpace(spec.ImageURL)
		if imageURL == "" {
			continue
		}
		if !workload.ValidCommand(spec.Command) {
			return nil, fmt.Errorf("预拉取镜像 %s 自检命令无效", imageURL)
		}
		item := ImageSpec{ImageURL: imageURL, Command: slices.Clone(spec.Command), Hold: spec.Hold, EphemeralMounts: canonicalMounts(spec.EphemeralMounts)}
		key := imageSpecKey(item)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out, nil
}

func canonical(specs []ImageSpec) []ImageSpec {
	out := make([]ImageSpec, 0, len(specs))
	for _, spec := range specs {
		item := ImageSpec{ImageURL: strings.TrimSpace(spec.ImageURL), Command: slices.Clone(spec.Command), Hold: spec.Hold}
		item.EphemeralMounts = canonicalMounts(spec.EphemeralMounts)
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return imageSpecKey(out[i]) < imageSpecKey(out[j])
	})
	return out
}

func imageSpecKey(spec ImageSpec) string {
	encoded, _ := json.Marshal(spec)
	return string(encoded)
}

func canonicalMounts(mounts []workload.EphemeralMountSpec) []workload.EphemeralMountSpec {
	var out []workload.EphemeralMountSpec
	seen := map[string]struct{}{}
	for _, mount := range mounts {
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
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name+"\x00"+out[i].MountPath < out[j].Name+"\x00"+out[j].MountPath
	})
	return out
}
