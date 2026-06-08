// M4 后端计算适配器:提供 compute=backend 的可插拔执行注册表。
package sim

import (
	"context"
	"sync"

	"chaimir/pkg/apperr"
)

// BackendStepInput 是一次后端仿真步进的输入。
type BackendStepInput struct {
	SessionID int64          `json:"session_id"`
	Tick      int32          `json:"tick"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
	State     map[string]any `json:"state"`
	Config    map[string]any `json:"config"`
}

// BackendStepOutput 是后端仿真步进后的状态输出。
type BackendStepOutput struct {
	Tick  int32          `json:"tick"`
	State map[string]any `json:"state"`
}

// BackendAdapter 是 M4 后端仿真适配器接口。
type BackendAdapter interface {
	// Step 消费一次事件并返回新的确定性状态。
	Step(ctx context.Context, input BackendStepInput) (BackendStepOutput, error)
}

// BackendAdapterFunc 允许用函数快速注册后端适配器。
type BackendAdapterFunc func(ctx context.Context, input BackendStepInput) (BackendStepOutput, error)

// Step 执行函数式适配器。
func (f BackendAdapterFunc) Step(ctx context.Context, input BackendStepInput) (BackendStepOutput, error) {
	return f(ctx, input)
}

// BackendAdapterRegistry 管理 M4 自有后端计算适配器。
type BackendAdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[string]BackendAdapter
}

// NewBackendAdapterRegistry 构造空适配器注册表。
func NewBackendAdapterRegistry() *BackendAdapterRegistry {
	return &BackendAdapterRegistry{adapters: map[string]BackendAdapter{}}
}

// Register 注册一个后端适配器;同名注册会覆盖启动期配置,调用方负责装配顺序。
func (r *BackendAdapterRegistry) Register(code string, adapter BackendAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[code] = adapter
}

// Exists 判断后端适配器是否已注册,用于 compute=backend 包接入校验。
func (r *BackendAdapterRegistry) Exists(code string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[code] != nil
}

// Step 调用已注册适配器执行一步后端仿真。
func (r *BackendAdapterRegistry) Step(ctx context.Context, code string, input BackendStepInput) (BackendStepOutput, error) {
	r.mu.RLock()
	adapter := r.adapters[code]
	r.mu.RUnlock()
	if adapter == nil {
		return BackendStepOutput{}, apperr.ErrSimBackendUnavailable
	}
	out, err := adapter.Step(ctx, input)
	if err != nil {
		return BackendStepOutput{}, apperr.ErrSimBackendUnavailable.WithCause(err)
	}
	return out, nil
}
