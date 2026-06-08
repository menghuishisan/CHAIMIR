// M2 EVM 运行时最小能力实现。
// 该实现面向 adapter_spec 暴露的 JSON-RPC 端口,提供 deploy/tx/query/reset 及自检。
package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"chaimir/pkg/apperr"
)

// EVMCapability 通过 JSON-RPC 驱动 EVM 沙箱。
type EVMCapability struct {
	httpClient *http.Client
	orch       Orchestrator
}

// NewEVMCapability 构造最小 EVM 能力实现。
func NewEVMCapability(orch Orchestrator, timeout time.Duration) *EVMCapability {
	return &EVMCapability{httpClient: &http.Client{Timeout: timeout}, orch: orch}
}

// Deploy 通过运行时工作目录脚本触发部署。
func (c *EVMCapability) Deploy(ctx context.Context, binding SandboxRuntimeBinding, payload map[string]any) (map[string]any, error) {
	command, err := payloadShell(binding.WorkspaceDir, payload, "deploy.sh")
	if err != nil {
		return nil, err
	}
	var stdout bytes.Buffer
	if err := c.exec(ctx, binding, command, &stdout); err != nil {
		return nil, err
	}
	return map[string]any{"output": strings.TrimSpace(stdout.String())}, nil
}

// SendTx 执行交易脚本。
func (c *EVMCapability) SendTx(ctx context.Context, binding SandboxRuntimeBinding, payload map[string]any) (map[string]any, error) {
	command, err := payloadShell(binding.WorkspaceDir, payload, "tx.sh")
	if err != nil {
		return nil, err
	}
	var stdout bytes.Buffer
	if err := c.exec(ctx, binding, command, &stdout); err != nil {
		return nil, err
	}
	return map[string]any{"output": strings.TrimSpace(stdout.String())}, nil
}

// Query 查询链状态。
func (c *EVMCapability) Query(ctx context.Context, binding SandboxRuntimeBinding, target string) (map[string]any, error) {
	resp, err := c.rpc(ctx, binding, "eth_blockNumber", []any{})
	if err != nil {
		return nil, err
	}
	return map[string]any{"target": target, "result": resp}, nil
}

// Reset 重置 EVM 链到创世态。
func (c *EVMCapability) Reset(ctx context.Context, binding SandboxRuntimeBinding) error {
	_, err := c.rpc(ctx, binding, "anvil_reset", []any{map[string]any{}})
	return err
}

// Selftest 执行最小接入即测。
func (c *EVMCapability) Selftest(ctx context.Context, binding SandboxRuntimeBinding, spec RuntimeSelftestSpec) error {
	if _, err := c.Query(ctx, binding, spec.QueryTarget); err != nil {
		return err
	}
	if _, err := c.Deploy(ctx, binding, spec.DeployPayload); err != nil {
		return err
	}
	if spec.TxPayload != nil {
		if _, err := c.SendTx(ctx, binding, spec.TxPayload); err != nil {
			return err
		}
	}
	return c.Reset(ctx, binding)
}

// rpc 调用运行时声明的集群内 EVM RPC 端口,只暴露标准 L2 能力结果给上层。
func (c *EVMCapability) rpc(ctx context.Context, binding SandboxRuntimeBinding, method string, params []any) (result any, err error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, apperr.ErrSandboxChainOperationFail.WithCause(err)
	}
	endpoint, err := evmRPCEndpoint(binding)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, apperr.ErrSandboxChainOperationFail.WithCause(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperr.ErrSandboxChainOperationFail.WithCause(err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, apperr.ErrSandboxChainOperationFail.WithCause(closeErr))
		}
	}()
	var payload struct {
		Result any            `json:"result"`
		Error  map[string]any `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, apperr.ErrSandboxChainOperationFail.WithCause(err)
	}
	if payload.Error != nil {
		return nil, apperr.ErrSandboxChainOperationFail.WithCause(fmt.Errorf("rpc error: %v", payload.Error))
	}
	return payload.Result, nil
}

// evmRPCEndpoint 从声明式运行时端口生成集群内 RPC 地址,避免 L2 能力硬编码具体端口。
func evmRPCEndpoint(binding SandboxRuntimeBinding) (string, error) {
	service := strings.TrimSpace(binding.ServiceName)
	port := binding.PortByName["rpc"]
	if service == "" || strings.TrimSpace(binding.Namespace) == "" || port <= 0 {
		return "", apperr.ErrSandboxChainOperationFail
	}
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", service, binding.Namespace, port), nil
}

// exec 通过 M2 编排器在运行时主容器内执行命令。
func (c *EVMCapability) exec(ctx context.Context, binding SandboxRuntimeBinding, command []string, stdout *bytes.Buffer) error {
	if c.orch == nil {
		return apperr.ErrRuntimeCapabilityUnavailable
	}
	if err := c.orch.Exec(ctx, binding, command, nil, stdout, nil, false); err != nil {
		return apperr.ErrSandboxChainOperationFail.WithCause(err)
	}
	return nil
}

// payloadShell 构造受限工作目录内脚本执行命令,非法脚本路径直接拒绝而不是改写后执行。
func payloadShell(workspaceDir string, payload map[string]any, defaultScript string) ([]string, error) {
	script := defaultScript
	if v, ok := payload["script"].(string); ok && strings.TrimSpace(v) != "" {
		script = v
	}
	if strings.TrimSpace(workspaceDir) == "" {
		workspaceDir = "/workspace"
	}
	scriptName, err := shellScriptName(script)
	if err != nil {
		return nil, err
	}
	return []string{"sh", "-lc", "cd " + shellQuote(workspaceDir) + " && ./" + shellQuote(scriptName)}, nil
}

// shellScriptName 限制脚本名为 POSIX 相对路径,避免 payload 注入 shell 片段或逃逸工作区。
func shellScriptName(script string) (string, error) {
	trimmed := strings.TrimSpace(script)
	if trimmed == "" || path.IsAbs(trimmed) || strings.Contains(trimmed, "\\") {
		return "", apperr.ErrSandboxFileInvalid.WithCause(fmt.Errorf("invalid script path"))
	}
	if strings.ContainsAny(trimmed, " \t\r\n;&|$`<>") {
		return "", apperr.ErrSandboxFileInvalid.WithCause(fmt.Errorf("invalid script path"))
	}
	for _, part := range strings.Split(trimmed, "/") {
		if part == "" || part == "." || part == ".." {
			return "", apperr.ErrSandboxFileInvalid.WithCause(fmt.Errorf("invalid script path"))
		}
	}
	if cleaned := path.Clean(trimmed); cleaned != trimmed {
		return "", apperr.ErrSandboxFileInvalid.WithCause(fmt.Errorf("invalid script path"))
	}
	return trimmed, nil
}
