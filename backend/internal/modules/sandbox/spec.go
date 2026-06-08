// M2 运行时与工具声明式 spec 解析与校验。
// 本文件负责把 runtime.adapter_spec / tool.resource_spec 从 JSONB 还原为模块内部结构,
// 并在边界处做格式校验,避免编排阶段才暴露坏配置。
package sandbox

import (
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"

	"k8s.io/apimachinery/pkg/api/resource"
)

// parseRuntimeAdapterSpec 解析并校验 runtime.adapter_spec。
func parseRuntimeAdapterSpec(raw []byte) (RuntimeAdapterSpec, error) {
	var spec RuntimeAdapterSpec
	if len(raw) == 0 {
		return spec, apperr.ErrRuntimeInvalid
	}
	if err := jsonx.DecodeStrict(raw, &spec); err != nil {
		return spec, apperr.ErrRuntimeInvalid.WithCause(err)
	}
	if strings.TrimSpace(spec.WorkspaceDir) == "" {
		return spec, apperr.ErrRuntimeInvalid
	}
	if err := validateContainerSpec(spec.RuntimeContainer, true); err != nil {
		return spec, err
	}
	for _, sidecar := range spec.InfraSidecars {
		if err := validateContainerSpec(sidecar, false); err != nil {
			return spec, err
		}
	}
	if strings.TrimSpace(spec.Selftest.QueryTarget) == "" {
		spec.Selftest.QueryTarget = "latest"
	}
	return spec, nil
}

// parseToolResourceSpec 解析并校验 tool.resource_spec。
func parseToolResourceSpec(tool ToolDefinition, raw []byte) (ToolResourceSpec, error) {
	var spec ToolResourceSpec
	if len(raw) == 0 {
		if tool.Kind == ToolKindTerminal || tool.Kind == ToolKindPlatformBuiltin {
			return spec, nil
		}
		return spec, apperr.ErrToolCreateInvalid
	}
	if err := jsonx.DecodeStrict(raw, &spec); err != nil {
		return spec, apperr.ErrToolCreateInvalid.WithCause(err)
	}
	if tool.Kind == ToolKindWebEmbed {
		if len(spec.Command) == 0 || tool.Port <= 0 {
			return spec, apperr.ErrToolCreateInvalid
		}
	}
	if err := validateResourceSpec(spec.Resources, apperr.ErrToolCreateInvalid); err != nil {
		return spec, err
	}
	if err := validateProbeSpec(spec.ReadinessProbe, apperr.ErrToolCreateInvalid); err != nil {
		return spec, err
	}
	if err := validateProbeSpec(spec.LivenessProbe, apperr.ErrToolCreateInvalid); err != nil {
		return spec, err
	}
	return spec, nil
}

// validateContainerSpec 校验运行时与 sidecar 容器的最小声明。
func validateContainerSpec(spec ContainerSpec, runtimeMain bool) error {
	if strings.TrimSpace(spec.Name) == "" {
		return apperr.ErrRuntimeInvalid
	}
	if runtimeMain {
		if len(spec.Command) == 0 {
			return apperr.ErrRuntimeInvalid
		}
	} else if strings.TrimSpace(spec.ImageURL) == "" {
		return apperr.ErrRuntimeInvalid
	}
	for _, port := range spec.Ports {
		if strings.TrimSpace(port.Name) == "" || port.ContainerPort <= 0 || port.ServicePort <= 0 {
			return apperr.ErrRuntimeInvalid
		}
	}
	if err := validateResourceSpec(spec.Resources, apperr.ErrRuntimeInvalid); err != nil {
		return err
	}
	if err := validateProbeSpec(spec.ReadinessProbe, apperr.ErrRuntimeInvalid); err != nil {
		return err
	}
	if err := validateProbeSpec(spec.LivenessProbe, apperr.ErrRuntimeInvalid); err != nil {
		return err
	}
	return nil
}

// validateResourceSpec 使用 Kubernetes 官方 quantity parser 校验资源声明。
func validateResourceSpec(spec ResourceSpec, code *apperr.Error) error {
	for _, value := range []string{spec.Requests.CPU, spec.Requests.Memory, spec.Limits.CPU, spec.Limits.Memory} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if _, err := resource.ParseQuantity(value); err != nil {
			return code.WithCause(err)
		}
	}
	return nil
}

// validateProbeSpec 校验探针定义;空探针允许,非空必须是平台支持的三种之一。
func validateProbeSpec(spec ProbeSpec, code *apperr.Error) error {
	if spec.Type == "" {
		return nil
	}
	switch spec.Type {
	case "tcp", "http", "exec":
	default:
		return code
	}
	if spec.PeriodSeconds <= 0 {
		spec.PeriodSeconds = 2
	}
	if spec.FailureThreshold <= 0 {
		spec.FailureThreshold = 30
	}
	return nil
}
