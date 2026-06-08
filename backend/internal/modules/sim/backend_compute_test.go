// M4 后端计算测试:覆盖适配器注册、确定性事件执行与未知适配器错误。
package sim

import (
	"context"
	"testing"
)

// TestBackendComputeRegistryRunsRegisteredAdapter 确认后端计算只通过 M4 自有适配器注册表执行。
func TestBackendComputeRegistryRunsRegisteredAdapter(t *testing.T) {
	registry := NewBackendAdapterRegistry()
	registry.Register("counter", BackendAdapterFunc(func(ctx context.Context, input BackendStepInput) (BackendStepOutput, error) {
		next := input.Tick + 1
		return BackendStepOutput{Tick: next, State: map[string]any{"tick": float64(next), "event": input.EventType}}, nil
	}))

	out, err := registry.Step(context.Background(), "counter", BackendStepInput{Tick: 4, EventType: "tick"})
	if err != nil {
		t.Fatalf("expected adapter to run, got %v", err)
	}
	if out.Tick != 5 || out.State["event"] != "tick" {
		t.Fatalf("unexpected adapter output: %+v", out)
	}
}

// TestBackendComputeRegistryRejectsUnknownAdapter 确认未知后端适配器不会静默降级。
func TestBackendComputeRegistryRejectsUnknownAdapter(t *testing.T) {
	registry := NewBackendAdapterRegistry()
	if _, err := registry.Step(context.Background(), "missing", BackendStepInput{}); err == nil {
		t.Fatalf("expected unknown adapter to be rejected")
	}
}
