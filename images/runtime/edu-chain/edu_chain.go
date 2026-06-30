// edu_chain.go 实现 Chaimir 自研教学链节点,提供确定性的区块、交易和共识状态接口。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

var startedAt = time.Now().Unix()

// main 解析运行参数并启动教学链 HTTP 节点。
func main() {
	selftest := flag.Bool("selftest", false, "run syntax/runtime selftest")
	flag.Parse()
	if *selftest {
		fmt.Println("edu-chain selftest ok")
		return
	}
	port := getenvInt("CHAIMIR_EDU_CHAIN_PORT", 8080)
	maxBodyBytes := int64(getenvInt("CHAIMIR_MAX_BODY_BYTES", 65536))
	chainID := getenv("CHAIMIR_EDU_CHAIN_ID", "chaimir-edu")
	handler := eduChainHandler{chainID: chainID, maxBodyBytes: maxBodyBytes}
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           handler.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

// eduChainHandler 保存教学链 HTTP 处理所需的运行配置。
type eduChainHandler struct {
	chainID      string
	maxBodyBytes int64
}

// routes 注册教学链只读查询和受控交易提交端点。
func (h eduChainHandler) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/chain", h.chain)
	mux.HandleFunc("/block/latest", h.latestBlock)
	mux.HandleFunc("/tx", h.tx)
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
	sum := sha256.Sum256(body)
	writeJSON(w, map[string]any{"accepted": true, "tx_hash": hex.EncodeToString(sum[:])})
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
