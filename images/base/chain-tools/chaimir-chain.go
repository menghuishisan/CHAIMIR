// 本文件提供声明驱动的链能力插件入口,不按生态分支,只执行运行时镜像声明的协议。
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type request struct {
	Target  string         `json:"target,omitempty"`
	Method  string         `json:"method,omitempty"`
	Params  []any          `json:"params,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

type response map[string]any

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("chaimir-chain 2")
		return
	}
	if len(os.Args) != 2 {
		fail("参数不正确")
	}
	var in request
	if err := json.NewDecoder(bufio.NewReader(os.Stdin)).Decode(&in); err != nil {
		fail("输入不是有效 JSON")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var out response
	var err error
	switch os.Args[1] {
	case "selftest":
		out, err = runConfigured(ctx, "selftest", in, true)
	case "deploy", "tx":
		out, err = runConfigured(ctx, os.Args[1], in, false)
	case "query":
		out, err = query(ctx, in)
	case "reset":
		out, err = runConfigured(ctx, "reset", in, false)
	default:
		fail("未登记的链能力动作")
	}
	if err != nil {
		fail(err.Error())
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fail("输出链能力结果失败")
	}
}

// query 优先使用运行时声明的方法,否则使用请求目标;方法为空时拒绝隐式生态分支。
func query(ctx context.Context, in request) (response, error) {
	if configured, err := runConfigured(ctx, "query", in, false); configured != nil || err != nil {
		return configured, err
	}
	method := strings.TrimSpace(in.Method)
	if method == "" {
		method = strings.TrimSpace(in.Target)
	}
	if method == "" {
		return nil, errors.New("查询必须声明 method 或 target")
	}
	return jsonRPC(ctx, method, in.Params)
}

// runConfigured 执行 manifest/环境声明的 argv 插件;它不解释生态名,只负责边界校验和 JSON I/O。
func runConfigured(ctx context.Context, action string, in request, allowProbe bool) (response, error) {
	argv, err := configuredArgv(action)
	if err != nil {
		return nil, err
	}
	if len(argv) == 0 {
		method, err := configuredMethod(action)
		if err != nil {
			return nil, err
		}
		if method != "" {
			params := in.Params
			if len(params) == 0 && in.Payload != nil {
				params = []any{in.Payload}
			}
			result, rpcErr := jsonRPC(ctx, method, params)
			if rpcErr != nil {
				return nil, rpcErr
			}
			if allowProbe {
				return response{"result": "passed", "probe": result}, nil
			}
			return result, nil
		}
		if allowProbe {
			if strings.TrimSpace(os.Getenv("CHAIMIR_ADAPTER_SELFTEST_METHOD")) == "" {
				return nil, errors.New("运行时未声明 selftest 方法")
			}
			result, rpcErr := jsonRPC(ctx, strings.TrimSpace(os.Getenv("CHAIMIR_ADAPTER_SELFTEST_METHOD")), nil)
			if rpcErr != nil {
				return nil, rpcErr
			}
			return response{"result": "passed", "probe": result}, nil
		}
		return nil, nil
	}
	input, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("编码插件请求失败: %w", err)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(input)
	stdout, stderr, err := runCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("插件 %s 执行失败: %s: %w", action, strings.TrimSpace(string(stderr)), err)
	}
	var out response
	if err := json.Unmarshal(stdout, &out); err != nil {
		return nil, fmt.Errorf("插件 %s 未返回 JSON: %w", action, err)
	}
	return out, nil
}

// configuredMethod 读取 manifest 注入的 RPC 方法,保持方法型插件与 argv 型插件同一协议边界。
func configuredMethod(action string) (string, error) {
	name := "CHAIMIR_ADAPTER_" + strings.ToUpper(action) + "_METHOD"
	method := strings.TrimSpace(os.Getenv(name))
	if method == "" {
		return "", nil
	}
	if strings.ContainsAny(method, " \t\r\n") {
		return "", fmt.Errorf("%s 不能包含空白字符", name)
	}
	return method, nil
}

func configuredArgv(action string) ([]string, error) {
	name := "CHAIMIR_ADAPTER_" + strings.ToUpper(action) + "_ARGV"
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, nil
	}
	var argv []string
	if err := json.Unmarshal([]byte(raw), &argv); err != nil || len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return nil, fmt.Errorf("%s 必须是非空 JSON argv", name)
	}
	for _, part := range argv {
		if strings.TrimSpace(part) == "" {
			return nil, fmt.Errorf("%s 包含空参数", name)
		}
	}
	return argv, nil
}

func jsonRPC(ctx context.Context, method string, params []any) (response, error) {
	endpoint := strings.TrimSpace(os.Getenv("CHAIMIR_CHAIN_RPC_URL"))
	if endpoint == "" {
		return nil, errors.New("未配置 CHAIMIR_CHAIN_RPC_URL")
	}
	body := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("编码 RPC 请求失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("创建 RPC 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RPC 请求失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 RPC 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("RPC 返回 HTTP %d", resp.StatusCode)
	}
	var out response
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("RPC 响应不是 JSON: %w", err)
	}
	if value, ok := out["error"]; ok && value != nil {
		return nil, fmt.Errorf("RPC 返回错误: %v", value)
	}
	return out, nil
}

func runCommand(cmd *exec.Cmd) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func fail(message string) {
	_, _ = fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
