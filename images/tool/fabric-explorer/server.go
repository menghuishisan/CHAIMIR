// 本文件提供 Fabric 原生浏览器服务,所有链上操作都通过受控 peer CLI 执行。
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var fabricNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

type explorer struct {
	timeout              time.Duration
	ordererAddress       string
	ordererTLSRootCA     string
	ordererTLSClientCert string
	ordererTLSClientKey  string
}

type chaincodeRequest struct {
	Channel string   `json:"channel"`
	Name    string   `json:"name"`
	Fcn     string   `json:"fcn"`
	Args    []string `json:"args"`
}

func main() {
	port := requiredPort("CHAIMIR_FABRIC_EXPLORER_PORT")
	httpTimeout := time.Duration(requiredPositiveInt("CHAIMIR_HTTP_TIMEOUT_SECONDS")) * time.Second
	requiredEndpoint("CORE_PEER_ADDRESS")
	ordererAddress := requiredEndpoint("CHAIMIR_FABRIC_ORDERER_ADDRESS")
	explorerServer := &explorer{
		timeout:              httpTimeout,
		ordererAddress:       ordererAddress,
		ordererTLSRootCA:     requiredFile("CHAIMIR_FABRIC_ORDERER_TLS_ROOTCERT_FILE"),
		ordererTLSClientCert: requiredFile("CHAIMIR_FABRIC_ORDERER_TLS_CLIENTCERT_FILE"),
		ordererTLSClientKey:  requiredFile("CHAIMIR_FABRIC_ORDERER_TLS_CLIENTKEY_FILE"),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", explorerServer.healthz)
	mux.HandleFunc("/api/channels", explorerServer.channels)
	mux.HandleFunc("/api/block", explorerServer.block)
	mux.HandleFunc("/api/chaincode/query", explorerServer.chaincodeQuery)
	mux.HandleFunc("/api/chaincode/submit", explorerServer.chaincodeSubmit)
	mux.HandleFunc("/", explorerServer.index)
	log.Printf("fabric explorer listening port=%d", port)
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           requestLog(mux),
		ReadHeaderTimeout: httpTimeout,
		ReadTimeout:       httpTimeout,
		WriteTimeout:      httpTimeout,
		IdleTimeout:       httpTimeout,
		MaxHeaderBytes:    16 << 10,
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func (e *explorer) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不受支持", "")
		return
	}
	if _, _, err := e.runPeer(r.Context(), "version"); err != nil {
		writeError(w, http.StatusServiceUnavailable, "浏览器服务正在准备,请稍候", traceID())
		return
	}
	writeJSON(w, map[string]any{"status": "ok"})
}

func (e *explorer) channels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不受支持", "")
		return
	}
	stdout, stderr, err := e.runPeer(r.Context(), "channel", "list")
	if err != nil {
		writePeerError(w, "读取通道列表失败", stderr, err)
		return
	}
	items := make([]string, 0)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "channel peers") {
			continue
		}
		if fabricNamePattern.MatchString(line) {
			items = append(items, line)
		}
	}
	writeJSON(w, map[string]any{"channels": items})
}

func (e *explorer) block(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不受支持", "")
		return
	}
	channel := strings.TrimSpace(r.URL.Query().Get("channel"))
	number := strings.TrimSpace(r.URL.Query().Get("number"))
	if !fabricNamePattern.MatchString(channel) || !validBlockNumber(number) {
		writeError(w, http.StatusBadRequest, "通道或区块编号不正确", "")
		return
	}
	file, err := os.CreateTemp("/tmp", "fabric-block-*.pb")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "暂时无法读取区块", traceID())
		return
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "暂时无法读取区块", traceID())
		return
	}
	defer func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("trace_id=%s operation=remove-fabric-block-file error=%q", traceID(), err.Error())
		}
	}()
	args := []string{"channel", "fetch", number, path, "-c", channel}
	args = e.appendOrdererAddress(args)
	_, stderr, runErr := e.runPeer(r.Context(), args...)
	if runErr != nil {
		writePeerError(w, "读取区块失败", stderr, runErr)
		return
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取区块结果失败", traceID())
		return
	}
	sum := sha256.Sum256(data)
	writeJSON(w, map[string]any{"channel": channel, "number": number, "size": len(data), "sha256": hex.EncodeToString(sum[:])})
}

func (e *explorer) chaincodeQuery(w http.ResponseWriter, r *http.Request) {
	e.chaincode(w, r, false)
}

func (e *explorer) chaincodeSubmit(w http.ResponseWriter, r *http.Request) {
	e.chaincode(w, r, true)
}

func (e *explorer) chaincode(w http.ResponseWriter, r *http.Request, submit bool) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不受支持", "")
		return
	}
	var req chaincodeRequest
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || !validChaincodeRequest(req) {
		writeError(w, http.StatusBadRequest, "链码请求参数不正确", "")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "链码请求参数不正确", "")
		return
	}
	args := append([]string{req.Fcn}, req.Args...)
	payload, err := json.Marshal(map[string]any{"Args": args})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "链码请求编码失败", traceID())
		return
	}
	command := []string{"chaincode"}
	if submit {
		command = append(command, "invoke")
	} else {
		command = append(command, "query")
	}
	command = append(command, "-C", req.Channel, "-n", req.Name, "-c", string(payload))
	if submit {
		command = append(command, "--waitForEvent")
		command = e.appendOrdererAddress(command)
	}
	stdout, stderr, runErr := e.runPeer(r.Context(), command...)
	if runErr != nil {
		writePeerError(w, "链码操作失败", stderr, runErr)
		return
	}
	writeJSON(w, map[string]any{"channel": req.Channel, "name": req.Name, "submitted": submit, "result": strings.TrimSpace(stdout)})
}

// appendOrdererAddress 只使用组合编译注入的内部 orderer 地址,请求体不能改变网络目标。
func (e *explorer) appendOrdererAddress(args []string) []string {
	return append(args,
		"-o", e.ordererAddress,
		"--tls",
		"--cafile", e.ordererTLSRootCA,
		"--clientauth",
		"--certfile", e.ordererTLSClientCert,
		"--keyfile", e.ordererTLSClientKey,
		"--ordererTLSHostnameOverride", "orderer",
	)
}

func requiredConfig(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}

// requiredFile 校验运行期共享安全域中的证书或密钥路径,不读取或打印其内容。
func requiredFile(key string) string {
	value := requiredConfig(key)
	if _, err := os.Stat(filepath.Clean(value)); err != nil {
		log.Fatalf("%s points to an unavailable file", key)
	}
	return value
}

// requiredEndpoint accepts only the host:port form used by Fabric CLI flags.
func requiredEndpoint(key string) string {
	value := requiredConfig(key)
	if strings.Contains(value, "://") || strings.ContainsAny(value, "/?#@") {
		log.Fatalf("%s must be a host:port endpoint", key)
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil || host == "" {
		log.Fatalf("%s must be a host:port endpoint", key)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		log.Fatalf("%s must use a TCP port in 1-65535", key)
	}
	return value
}

func requiredPort(key string) int {
	value := requiredConfig(key)
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		log.Fatalf("%s must be a valid TCP port", key)
	}
	return port
}

// requiredPositiveInt 读取 HTTP 服务的有限正整数配置。
func requiredPositiveInt(key string) int {
	value := requiredConfig(key)
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		log.Fatalf("%s must be a positive integer", key)
	}
	return parsed
}

func (e *explorer) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := io.WriteString(w, `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Fabric 网络浏览器</title><style>body{font-family:system-ui,sans-serif;max-width:900px;margin:40px auto;padding:0 20px;color:#1f2937}button{padding:8px 12px;margin:4px 0}input{padding:8px;margin:4px 0;width:100%;box-sizing:border-box}pre{background:#f3f4f6;padding:12px;white-space:pre-wrap}</style></head><body><h1>Fabric 网络浏览器</h1><p>通过平台代理查看当前网络状态并执行已授权的链码操作。</p><button id="refresh">读取通道</button><pre id="output">请选择操作</pre><script>const out=document.getElementById('output');document.getElementById('refresh').onclick=async()=>{const r=await fetch('/api/channels');out.textContent=JSON.stringify(await r.json(),null,2)};</script></body></html>`); err != nil {
		log.Printf("trace_id=%s operation=write-fabric-index error=%q", traceID(), err.Error())
	}
}

func (e *explorer) runPeer(parent context.Context, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(parent, e.timeout)
	defer cancel()
	command := exec.CommandContext(ctx, "peer", args...)
	stdout, stderr := new(strings.Builder), new(strings.Builder)
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	if ctx.Err() != nil {
		return stdout.String(), stderr.String(), ctx.Err()
	}
	return stdout.String(), stderr.String(), err
}

func validChaincodeRequest(req chaincodeRequest) bool {
	if !fabricNamePattern.MatchString(req.Channel) || !fabricNamePattern.MatchString(req.Name) || !fabricNamePattern.MatchString(req.Fcn) || len(req.Args) > 32 {
		return false
	}
	for _, arg := range req.Args {
		if len(arg) > 4096 || strings.ContainsAny(arg, "\x00\r\n") {
			return false
		}
	}
	return true
}

func validBlockNumber(value string) bool {
	number, err := strconv.ParseUint(value, 10, 64)
	return err == nil && number <= 9_223_372_036_854_775_807
}

func writePeerError(w http.ResponseWriter, message, stderr string, err error) {
	id := traceID()
	log.Printf("trace_id=%s operation=fabric-peer error=%q detail=%q", id, err.Error(), sanitizeLog(stderr))
	writeError(w, http.StatusBadGateway, message, id)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("trace_id=%s operation=write-json error=%q", traceID(), err.Error())
	}
}

func writeError(w http.ResponseWriter, status int, message, id string) {
	body := map[string]any{"message": message}
	if id != "" {
		body["trace_id"] = id
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("trace_id=%s operation=write-error error=%q", traceID(), err.Error())
	}
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("method=%s path=%s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func traceID() string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "fabric-explorer"
	}
	return hex.EncodeToString(data[:])
}

func sanitizeLog(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 512 {
		value = value[:512]
	}
	return value
}
