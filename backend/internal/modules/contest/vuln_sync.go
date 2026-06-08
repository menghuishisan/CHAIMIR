// M8 漏洞源同步:按配置驱动的通用 HTTP 适配器拉取案例并生成漏洞题草稿。
package contest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/netx"
	"chaimir/pkg/apperr"
)

// vulnSyncConfig 是 vuln_source.config 的服务端解析结果。
type vulnSyncConfig struct {
	Endpoint  string
	Method    string
	Timeout   time.Duration
	Headers   map[string]string
	Body      any
	CasesPath string
	Mapping   map[string]string
}

// SyncVulnSource 同步外部漏洞源并生成漏洞题草稿。
func (s *Service) SyncVulnSource(ctx context.Context, sourceID int64) (VulnSyncResultDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return VulnSyncResultDTO{}, apperr.ErrUnauthorized
	}
	if sourceID <= 0 {
		return VulnSyncResultDTO{}, apperr.ErrContestVulnSourceInvalid
	}
	source, err := s.store.GetVulnSource(ctx, sourceID)
	if err != nil {
		return VulnSyncResultDTO{}, err
	}
	if !source.Enabled {
		return VulnSyncResultDTO{}, apperr.ErrContestVulnSourceInvalid
	}
	revealed, err := revealVulnSourceConfig(s.cipher, source.Config)
	if err != nil {
		return VulnSyncResultDTO{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	cfg, err := s.parseVulnSyncConfig(revealed)
	if err != nil {
		return VulnSyncResultDTO{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	cases, err := s.fetchVulnCases(ctx, cfg)
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return VulnSyncResultDTO{}, err
		}
		return VulnSyncResultDTO{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}

	// 将每条外部案例映射为 M8 漏洞题草稿,正本仍由后续 finalize 固化入 M5。
	problems := make([]VulnProblemDTO, 0, len(cases))
	for _, item := range cases {
		req, err := vulnImportRequestFromCase(source, cfg.Mapping, item)
		if err != nil {
			return VulnSyncResultDTO{}, apperr.ErrContestVulnProblemInvalid.WithCause(err)
		}
		problem, err := s.store.CreateVulnProblem(ctx, id, s.nextID(), req)
		if err != nil {
			return VulnSyncResultDTO{}, err
		}
		problems = append(problems, problem)
	}
	if _, err := s.store.MarkVulnSourceSynced(ctx, sourceID); err != nil {
		return VulnSyncResultDTO{}, err
	}
	return VulnSyncResultDTO{SourceID: ids.Format(sourceID), ImportedCount: len(problems), Problems: problems}, nil
}

// parseVulnSyncConfig 校验并解析漏洞源同步配置,缺省超时来自服务启动配置。
func (s *Service) parseVulnSyncConfig(cfg map[string]any) (vulnSyncConfig, error) {
	endpoint := strings.TrimSpace(stringValueAt(cfg, "endpoint"))
	endpoint, err := netx.ValidatePublicHTTPURL(endpoint)
	if err != nil {
		return vulnSyncConfig{}, fmt.Errorf("漏洞源 endpoint 非法")
	}
	method := strings.ToUpper(strings.TrimSpace(stringValueAt(cfg, "method")))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodPost {
		return vulnSyncConfig{}, fmt.Errorf("漏洞源 method 非法")
	}
	timeout := s.vulnSourceTimeoutSeconds
	if _, exists := cfg["timeout_seconds"]; exists {
		timeout = secondsFromAny(cfg["timeout_seconds"])
	}
	if timeout < 1 || timeout > 60 {
		return vulnSyncConfig{}, fmt.Errorf("漏洞源 timeout_seconds 非法")
	}
	mapping := stringMapFromAny(cfg["mapping"])
	if strings.TrimSpace(mapping["title"]) == "" || strings.TrimSpace(mapping["draft_body"]) == "" {
		return vulnSyncConfig{}, fmt.Errorf("漏洞源 mapping 缺少必要字段")
	}
	return vulnSyncConfig{
		Endpoint:  endpoint,
		Method:    method,
		Timeout:   time.Duration(timeout) * time.Second,
		Headers:   headersFromAny(cfg["headers"]),
		Body:      cfg["body"],
		CasesPath: strings.TrimSpace(stringValueAt(cfg, "cases_path")),
		Mapping:   mapping,
	}, nil
}

// fetchVulnCases 请求外部源并抽取案例数组。
func (s *Service) fetchVulnCases(ctx context.Context, cfg vulnSyncConfig) (cases []map[string]any, err error) {
	client := s.httpClient
	if client == nil {
		client = netx.NewPublicHTTPClient(cfg.Timeout)
	}
	reqCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	var body io.Reader
	if cfg.Method == http.MethodPost && cfg.Body != nil {
		data, err := json.Marshal(cfg.Body)
		if err != nil {
			return nil, fmt.Errorf("漏洞源请求体编码失败: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(reqCtx, cfg.Method, cfg.Endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("漏洞源请求创建失败: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("漏洞源请求失败: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("漏洞源响应关闭失败: %w", closeErr))
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("漏洞源响应状态异常: %d", resp.StatusCode)
	}
	limit := s.vulnSourceMaxResponseBytes
	if limit <= 0 {
		return nil, fmt.Errorf("漏洞源响应大小上限未配置")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("漏洞源响应读取失败: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, apperr.ErrContestVulnSourceTooLarge
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("漏洞源响应不是有效 JSON: %w", err)
	}
	return caseListFromJSON(decoded, cfg.CasesPath)
}

// vulnImportRequestFromCase 将外部案例映射为漏洞题草稿导入请求。
func vulnImportRequestFromCase(source VulnSourceDTO, mapping map[string]string, item map[string]any) (VulnProblemImportRequest, error) {
	title := strings.TrimSpace(stringValueFromPath(item, mapping["title"]))
	if title == "" {
		return VulnProblemImportRequest{}, fmt.Errorf("漏洞案例缺少标题")
	}
	level, err := vulnLevelFromAny(valueFromPath(item, mapping["level"]), source.DefaultLevel)
	if err != nil {
		return VulnProblemImportRequest{}, err
	}
	mode, err := vulnRuntimeFromAny(valueFromPath(item, mapping["runtime_mode"]))
	if err != nil {
		return VulnProblemImportRequest{}, err
	}
	body := mapValueFromAny(valueFromPath(item, mapping["draft_body"]))
	if len(body) == 0 {
		return VulnProblemImportRequest{}, fmt.Errorf("漏洞案例缺少草稿正文")
	}
	return VulnProblemImportRequest{
		SourceID:    source.ID,
		ExternalRef: strings.TrimSpace(stringValueFromPath(item, mapping["external_ref"])),
		Title:       title,
		Level:       level,
		RuntimeMode: mode,
		DraftBody:   body,
	}, nil
}

// caseListFromJSON 从 JSON 根或 cases_path 指向位置抽取对象数组。
func caseListFromJSON(decoded any, casesPath string) ([]map[string]any, error) {
	target := decoded
	if strings.TrimSpace(casesPath) != "" {
		root, ok := decoded.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("漏洞源响应根不是对象")
		}
		target = valueFromPath(root, casesPath)
	}
	items, ok := target.([]any)
	if !ok {
		return nil, fmt.Errorf("漏洞源案例列表不是数组")
	}
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("漏洞源案例不是对象")
		}
		out = append(out, item)
	}
	return out, nil
}

// valueFromPath 按点路径从 JSON 对象读取值。
func valueFromPath(root map[string]any, path string) any {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	var current any = root
	for _, part := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = obj[strings.TrimSpace(part)]
	}
	return current
}

// stringValueFromPath 按点路径读取字符串化值。
func stringValueFromPath(root map[string]any, path string) string {
	return stringFromAny(valueFromPath(root, path))
}

// stringValueAt 从 map 读取字符串化值。
func stringValueAt(root map[string]any, key string) string {
	return stringFromAny(root[key])
}

// stringFromAny 把 JSON 标量转为字符串。
func stringFromAny(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	default:
		return ""
	}
}

// mapValueFromAny 把 JSON 对象或文本转为草稿正文对象。
func mapValueFromAny(v any) map[string]any {
	switch val := v.(type) {
	case map[string]any:
		return val
	case string:
		if strings.TrimSpace(val) == "" {
			return map[string]any{}
		}
		return map[string]any{"content": val}
	default:
		return map[string]any{}
	}
}

// headersFromAny 读取 HTTP 请求头配置。
func headersFromAny(v any) map[string]string {
	raw, ok := v.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if s := stringFromAny(v); strings.TrimSpace(k) != "" && strings.TrimSpace(s) != "" {
			out[k] = s
		}
	}
	return out
}

// stringMapFromAny 读取字符串映射配置。
func stringMapFromAny(v any) map[string]string {
	raw, ok := v.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if s := stringFromAny(v); strings.TrimSpace(k) != "" && strings.TrimSpace(s) != "" {
			out[k] = s
		}
	}
	return out
}

// secondsFromAny 读取秒级超时配置;缺省值由服务配置提供,这里只处理显式覆盖。
func secondsFromAny(v any) int {
	switch val := v.(type) {
	case nil:
		return 0
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

// vulnLevelFromAny 解析漏洞分级。
func vulnLevelFromAny(v any, defaultValue int16) (int16, error) {
	if defaultValue == 0 {
		defaultValue = VulnLevelC
	}
	switch strings.ToUpper(strings.TrimSpace(stringFromAny(v))) {
	case "":
		return defaultValue, nil
	case "1", "A":
		return VulnLevelA, nil
	case "2", "B":
		return VulnLevelB, nil
	case "3", "C":
		return VulnLevelC, nil
	default:
		return 0, fmt.Errorf("漏洞分级非法")
	}
}

// vulnRuntimeFromAny 解析漏洞题运行时模式。
func vulnRuntimeFromAny(v any) (int16, error) {
	switch strings.ToLower(strings.TrimSpace(stringFromAny(v))) {
	case "", "1", "isolated":
		return VulnRuntimeIsolated, nil
	case "2", "forked":
		return VulnRuntimeForked, nil
	default:
		return 0, fmt.Errorf("漏洞运行时模式非法")
	}
}
