// edu_chain.go 实现 Chaimir 自研教学链节点,提供确定性的区块、交易和共识状态接口。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var startedAt = time.Now().Unix()

// main 解析运行参数并启动教学链 HTTP 节点。
func main() {
	selftest := flag.Bool("selftest", false, "run syntax/runtime selftest")
	adapterAction := flag.String("adapter", "", "run a declared chain capability action")
	flag.Parse()
	if *selftest {
		fmt.Println("edu-chain selftest ok")
		return
	}
	if strings.TrimSpace(*adapterAction) != "" {
		if err := runAdapter(*adapterAction); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	port := getenvInt("CHAIMIR_EDU_CHAIN_PORT", 8080)
	maxBodyBytes := int64(getenvInt("CHAIMIR_MAX_BODY_BYTES", 65536))
	chainID := getenv("CHAIMIR_EDU_CHAIN_ID", "chaimir-edu")
	handler := eduChainHandler{chainID: chainID, maxBodyBytes: maxBodyBytes, objects: map[string]map[string]any{}, txs: map[string]map[string]any{}}
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           handler.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

// runAdapter 将平台统一 JSON 请求转成自研教学链的 HTTP API,不向平台泄漏内部存储结构。
func runAdapter(action string) error {
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return fmt.Errorf("读取适配器请求失败: %w", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("适配器输入不是有效 JSON: %w", err)
	}
	endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("CHAIMIR_CHAIN_RPC_URL")), "/")
	if endpoint == "" {
		return errors.New("未配置 CHAIMIR_CHAIN_RPC_URL")
	}
	var out map[string]any
	switch action {
	case "deploy", "tx":
		out, err = adapterPost(endpoint+"/"+action, payload)
	case "query":
		target, ok := payload["target"].(string)
		if !ok || strings.TrimSpace(target) == "" {
			return errors.New("query target is required")
		}
		queryURL, queryErr := adapterQueryURL(endpoint, target)
		if queryErr != nil {
			return queryErr
		}
		out, err = adapterGet(queryURL)
	default:
		return fmt.Errorf("未登记的教学链适配器动作: %s", action)
	}
	if err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		return fmt.Errorf("写出适配器响应失败: %w", err)
	}
	return nil
}

// adapterQueryURL 只允许 manifest 约定的教学链查询目标,避免把任意 URL 变成 SSRF 入口。
func adapterQueryURL(endpoint, target string) (string, error) {
	switch target {
	case "chain":
		return endpoint + "/chain", nil
	case "latest", "block/latest":
		return endpoint + "/block/latest", nil
	default:
		parsed, err := url.Parse(endpoint + "/query")
		if err != nil {
			return "", fmt.Errorf("构造查询地址失败: %w", err)
		}
		query := parsed.Query()
		query.Set("target", target)
		parsed.RawQuery = query.Encode()
		return parsed.String(), nil
	}
}

// adapterPost 向教学链提交 JSON 并严格要求 JSON 对象响应。
func adapterPost(endpoint string, payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("编码适配器请求失败: %w", err)
	}
	return adapterJSONRequest(http.MethodPost, endpoint, body)
}

// adapterGet 查询教学链只读端点并严格要求 JSON 对象响应。
func adapterGet(endpoint string) (map[string]any, error) {
	return adapterJSONRequest(http.MethodGet, endpoint, nil)
}

// adapterJSONRequest 执行有界 HTTP 请求,底层错误只返回给运行时内部日志。
func adapterJSONRequest(method, endpoint string, body []byte) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("创建教学链请求失败: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("教学链请求失败: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("读取教学链响应失败: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("教学链返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var out map[string]any
	if err := json.Unmarshal(responseBody, &out); err != nil {
		return nil, fmt.Errorf("教学链响应不是 JSON 对象: %w", err)
	}
	return out, nil
}

// eduChainHandler 保存教学链 HTTP 处理所需的运行配置。
type eduChainHandler struct {
	chainID      string
	maxBodyBytes int64
	mu           sync.RWMutex
	objects      map[string]map[string]any
	txs          map[string]map[string]any
}

// routes 注册教学链只读查询和受控交易提交端点。
func (h eduChainHandler) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/chain", h.chain)
	mux.HandleFunc("/block/latest", h.latestBlock)
	mux.HandleFunc("/deploy", h.deploy)
	mux.HandleFunc("/tx", h.tx)
	mux.HandleFunc("/query", h.query)
	return mux
}

// healthz 返回节点存活状态。
func (h eduChainHandler) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, map[string]any{"status": "ok"})
}

// chain 返回教学链基本信息。
func (h eduChainHandler) chain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, map[string]any{"chain_id": h.chainID, "consensus": "round-robin", "started_at": startedAt})
}

// latestBlock 返回按时间推进的确定性最新区块。
func (h eduChainHandler) latestBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	height := maxInt64(1, (time.Now().Unix()-startedAt)/5+1)
	writeJSON(w, map[string]any{"height": height, "hash": h.blockHash(height), "previous_hash": h.blockHash(height - 1)})
}

// tx 接收教学交易并返回确定性交易哈希。
func (h eduChainHandler) tx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.maxBodyBytes))
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	if len(body) == 0 || !json.Valid(body) {
		http.Error(w, "交易内容无效", http.StatusBadRequest)
		return
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "交易内容无效", http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	h.txs[hash] = payload
	h.mu.Unlock()
	writeJSON(w, map[string]any{"accepted": true, "tx_hash": hash})
}

// deploy 创建教学链上的可查询对象,返回由内容摘要确定的对象标识。
func (h eduChainHandler) deploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.maxBodyBytes))
	if err != nil || len(body) == 0 || !json.Valid(body) {
		http.Error(w, "部署内容无效", http.StatusBadRequest)
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "部署内容无效", http.StatusBadRequest)
		return
	}
	data := strings.TrimSpace(stringValue(payload, "data"))
	if data == "" {
		data = strings.TrimSpace(stringValue(payload, "code"))
	}
	if data == "" {
		http.Error(w, "部署内容必须提供 data 或 code", http.StatusBadRequest)
		return
	}
	sum := sha256.Sum256([]byte(h.chainID + ":object:" + data))
	objectID := hex.EncodeToString(sum[:])
	h.mu.Lock()
	h.objects[objectID] = map[string]any{"object_id": objectID, "data": data, "created_at": time.Now().UTC().Format(time.RFC3339)}
	h.mu.Unlock()
	writeJSON(w, map[string]any{"object_id": objectID, "accepted": true})
}

// query 查询教学链已部署对象或已提交交易,不暴露内部存储结构。
func (h eduChainHandler) query(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		http.Error(w, "查询目标不能为空", http.StatusBadRequest)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if strings.HasPrefix(target, "object:") {
		id := strings.TrimPrefix(target, "object:")
		object, ok := h.objects[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, object)
		return
	}
	if strings.HasPrefix(target, "tx:") {
		id := strings.TrimPrefix(target, "tx:")
		transaction, ok := h.txs[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{"tx_hash": id, "payload": transaction})
		return
	}
	http.Error(w, "查询目标格式无效", http.StatusBadRequest)
}

func stringValue(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

// blockHash 按高度生成确定性教学区块哈希。
func (h eduChainHandler) blockHash(height int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", h.chainID, height)))
	return hex.EncodeToString(sum[:])
}

// writeJSON 输出紧凑 JSON 响应。
func writeJSON(w http.ResponseWriter, payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if _, err := w.Write(body); err != nil {
		log.Printf("write response failed: %v", err)
	}
}

// getenv 读取字符串环境变量。
func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getenvInt 读取正整数环境变量。
func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// maxInt64 返回两个 int64 中较大的值。
func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
