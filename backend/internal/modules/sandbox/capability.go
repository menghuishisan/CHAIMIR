// M2 L2 链能力接口:封装部署、交易、查询、重置四个跨运行时动作。
package sandbox

import "context"

// ChainCapability 是运行时 L2 标准能力实现器。
type ChainCapability interface {
	Deploy(ctx context.Context, binding SandboxRuntimeBinding, payload map[string]any) (map[string]any, error)
	SendTx(ctx context.Context, binding SandboxRuntimeBinding, payload map[string]any) (map[string]any, error)
	Query(ctx context.Context, binding SandboxRuntimeBinding, target string) (map[string]any, error)
	Reset(ctx context.Context, binding SandboxRuntimeBinding) error
}

// RuntimeSelftester 是支持“接入即测”的运行时能力实现器。
type RuntimeSelftester interface {
	Selftest(ctx context.Context, binding SandboxRuntimeBinding, spec RuntimeSelftestSpec) error
}

// CapabilityRegistry 按 capability_impl 查找链能力实现器。
type CapabilityRegistry interface {
	Get(impl string) (ChainCapability, bool)
}

type staticCapabilityRegistry struct {
	items map[string]ChainCapability
}

// NewStaticCapabilityRegistry 构造静态能力注册表。
func NewStaticCapabilityRegistry(items map[string]ChainCapability) CapabilityRegistry {
	if items == nil {
		items = map[string]ChainCapability{}
	}
	return staticCapabilityRegistry{items: items}
}

// Get 按配置中的实现键返回能力实现器,未注册时由 service 转成运行时能力错误。
func (r staticCapabilityRegistry) Get(impl string) (ChainCapability, bool) {
	c, ok := r.items[impl]
	return c, ok
}
