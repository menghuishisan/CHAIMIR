// M2 EVM 链能力测试:确认 L2 标准能力通过沙箱编排器真实执行。
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"chaimir/pkg/apperr"
)

type recordingOrchestrator struct {
	command []string
	stdout  string
}

func (o *recordingOrchestrator) PrepullImage(context.Context, ImagePrepullSpec) (ImagePrepullStatus, error) {
	return ImagePrepullStatus{}, nil
}
func (o *recordingOrchestrator) Create(context.Context, SandboxCreateSpec) error { return nil }
func (o *recordingOrchestrator) WaitReady(context.Context, string) error         { return nil }
func (o *recordingOrchestrator) SnapshotAvailable(context.Context) error         { return nil }
func (o *recordingOrchestrator) SnapshotWorkspace(context.Context, SnapshotSpec) (SnapshotResult, error) {
	return SnapshotResult{}, nil
}
func (o *recordingOrchestrator) Pause(context.Context, SandboxRuntimeBinding) error { return nil }
func (o *recordingOrchestrator) Resume(context.Context, SandboxCreateSpec) error    { return nil }
func (o *recordingOrchestrator) Recycle(context.Context, string) error              { return nil }
func (o *recordingOrchestrator) RuntimeBinding(context.Context, string) (SandboxRuntimeBinding, error) {
	return SandboxRuntimeBinding{}, nil
}
func (o *recordingOrchestrator) ToolEndpoint(context.Context, string, string) (SandboxToolEndpoint, error) {
	return SandboxToolEndpoint{}, nil
}
func (o *recordingOrchestrator) Exec(_ context.Context, _ SandboxRuntimeBinding, command []string, _ io.Reader, stdout, _ io.Writer, _ bool) error {
	o.command = command
	if stdout != nil {
		_, _ = stdout.Write([]byte(o.stdout))
	}
	return nil
}

// TestEVMDeployExecutesScriptThroughOrchestrator 确认部署能力通过沙箱 shell 执行。
func TestEVMDeployExecutesScriptThroughOrchestrator(t *testing.T) {
	orch := &recordingOrchestrator{stdout: "deployed\n"}
	capability := NewEVMCapability(orch, time.Second)

	result, err := capability.Deploy(context.Background(), SandboxRuntimeBinding{WorkspaceDir: "/workspace"}, map[string]any{
		"script": "deploy-counter.sh",
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	if result["output"] != "deployed" {
		t.Fatalf("unexpected deploy output: %#v", result)
	}
	if len(orch.command) != 3 || !bytes.Contains([]byte(orch.command[2]), []byte("deploy-counter.sh")) {
		t.Fatalf("deploy did not execute expected script: %#v", orch.command)
	}
}

// TestEVMDeployRejectsUnsafeScriptPath 确认链能力脚本引用遇到绝对路径、逃逸或 shell 片段时直接拒绝。
func TestEVMDeployRejectsUnsafeScriptPath(t *testing.T) {
	capability := NewEVMCapability(&recordingOrchestrator{}, time.Second)
	binding := SandboxRuntimeBinding{WorkspaceDir: "/workspace"}
	for _, script := range []string{"/tmp/pwn.sh", "../deploy.sh", "deploy.sh; curl attacker"} {
		if _, err := capability.Deploy(context.Background(), binding, map[string]any{"script": script}); err == nil {
			t.Fatalf("unsafe script path %q must be rejected", script)
		} else if !hasAppCode(err, apperr.ErrSandboxFileInvalid.Code) {
			t.Fatalf("unsafe script path %q should use path error code, got %v", script, err)
		}
	}
}

// TestEVMRPCReturnsBodyCloseFailure 确认链 RPC 响应体关闭失败会进入应用错误链。
func TestEVMRPCReturnsBodyCloseFailure(t *testing.T) {
	closeFailure := errors.New("close failed")
	capability := NewEVMCapability(nil, time.Second)
	capability.httpClient = &http.Client{Transport: evmRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       closeFailBody{Reader: strings.NewReader(`{"result":"0x1"}`), err: closeFailure},
		}, nil
	})}

	_, err := capability.Query(context.Background(), SandboxRuntimeBinding{
		Namespace:   "sbx-test",
		ServiceName: "runtime-svc",
		PortByName:  map[string]int32{"rpc": 18545},
	}, "height")
	if !errors.Is(err, closeFailure) || !hasAppCode(err, apperr.ErrSandboxChainOperationFail.Code) {
		t.Fatalf("expected chain operation close failure, got %v", err)
	}
}

// TestEVMRPCUsesRuntimeBindingEndpoint 确认链 RPC URL 来自运行时声明式端口,不硬编码服务名或端口。
func TestEVMRPCUsesRuntimeBindingEndpoint(t *testing.T) {
	var gotURL string
	capability := NewEVMCapability(nil, time.Second)
	capability.httpClient = &http.Client{Transport: evmRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"result":"0x1"}`)),
		}, nil
	})}

	_, err := capability.Query(context.Background(), SandboxRuntimeBinding{
		Namespace:   "sbx-test",
		ServiceName: "runtime-svc",
		PortByName:  map[string]int32{"rpc": 18545},
	}, "height")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	want := "http://runtime-svc.sbx-test.svc.cluster.local:18545"
	if gotURL != want {
		t.Fatalf("expected RPC URL %s, got %s", want, gotURL)
	}
}

type evmRoundTripFunc func(*http.Request) (*http.Response, error)

func (f evmRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type closeFailBody struct {
	io.Reader
	err error
}

func (b closeFailBody) Close() error { return b.err }

func hasAppCode(err error, code string) bool {
	appErr, ok := apperr.As(err)
	return ok && appErr.Code == code
}
