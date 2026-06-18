// sim validation_bundle 文件负责仿真包归档安全校验、manifest 协议解析和危险调用静态扫描。
package sim

import (
	"path/filepath"
	"regexp"
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// BundleInput 是 API 边界读取 multipart 后交给 service 的仿真包正文。
type BundleInput struct {
	FileName    string
	ContentType string
	Data        []byte
}

// validateBundleManifestMatchesRequest 确认上传表单与包内自描述元信息一致,防止审核摘要和入库元数据分裂。
func validateBundleManifestMatchesRequest(manifest bundleManifest, req SubmitPackageRequest, compute int16) error {
	if strings.TrimSpace(manifest.Meta.Code) != req.Code || strings.TrimSpace(manifest.Meta.Version) != req.Version || strings.TrimSpace(manifest.Meta.Name) != req.Name || strings.TrimSpace(manifest.Meta.Category) != req.Category {
		return apperr.ErrSimPackageValidationFailed
	}
	manifestCompute, err := computeFromString(manifest.Meta.Compute)
	if err != nil || manifestCompute != compute {
		return apperr.ErrSimPackageValidationFailed
	}
	if len(manifest.InteractionSchema.Events) == 0 {
		return apperr.ErrSimPackageValidationFailed
	}
	return nil
}

var dangerousBundlePatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{name: "eval", re: regexp.MustCompile(`\beval\s*\(`)},
	{name: "function-constructor", re: regexp.MustCompile(`\bFunction\s*\(`)},
	{name: "network-fetch", re: regexp.MustCompile(`\bfetch\s*\(`)},
	{name: "network-xhr", re: regexp.MustCompile(`\bXMLHttpRequest\b`)},
	{name: "dynamic-import", re: regexp.MustCompile(`\bimport\s*\(`)},
	{name: "dom-document", re: regexp.MustCompile(`\bdocument\s*\.`)},
	{name: "dom-window", re: regexp.MustCompile(`\bwindow\s*\.`)},
	{name: "storage-local", re: regexp.MustCompile(`\blocalStorage\b`)},
	{name: "storage-session", re: regexp.MustCompile(`\bsessionStorage\b`)},
	{name: "cookie", re: regexp.MustCompile(`\bcookie\b`)},
	{name: "websocket", re: regexp.MustCompile(`\bWebSocket\b`)},
	{name: "script-tag", re: regexp.MustCompile(`(?i)<\s*script\b`)},
	{name: "inline-event", re: regexp.MustCompile(`(?i)\bon[a-z]+\s*=`)},
	{name: "svg-script", re: regexp.MustCompile(`(?i)<\s*svg\b`)},
	{name: "markdown-html", re: regexp.MustCompile(`(?i)<\s*iframe\b|<\s*object\b|<\s*embed\b`)},
	{name: "node-process", re: regexp.MustCompile(`\bprocess\.(env|mainModule|binding)\b`)},
	{name: "child-process", re: regexp.MustCompile(`\b(child_process|spawn|execFile|execSync)\b`)},
}

const simPackageManifestName = "sim-package.json"

var allowedPatternModes = map[string]struct{}{"graph": {}, "chain": {}, "tree": {}, "matrix": {}, "pipeline": {}, "lane": {}, "chart": {}}

// bundleManifest 保存后端可审核的自描述协议摘要,不承载可执行函数正文。
type bundleManifest struct {
	Meta              simManifestMeta
	InteractionSchema InteractionSchema
	CodeTrace         CodeTraceAudit
}

type simPackageManifest struct {
	Meta         simManifestMeta       `json:"meta"`
	Interactions []simInteractionDef   `json:"interactions"`
	Render       simRenderManifest     `json:"render"`
	Narrative    []map[string]any      `json:"narrative,omitempty"`
	CodeTrace    *simCodeTraceManifest `json:"codeTrace,omitempty"`
}

type simManifestMeta struct {
	Code       string         `json:"code"`
	Name       string         `json:"name"`
	Category   string         `json:"category"`
	Version    string         `json:"version"`
	Compute    string         `json:"compute"`
	ScaleLimit map[string]any `json:"scale_limit,omitempty"`
}

type simInteractionDef struct {
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	Label         string         `json:"label"`
	Emits         string         `json:"emits"`
	Params        []simFieldDef  `json:"params,omitempty"`
	Target        string         `json:"target,omitempty"`
	ElementFilter string         `json:"element_filter,omitempty"`
	AvailableWhen map[string]any `json:"available_when,omitempty"`
	LabelTag      string         `json:"label_tag,omitempty"`
	CooldownMS    int64          `json:"cooldown_ms,omitempty"`
}

type simFieldDef struct {
	Name     string           `json:"name"`
	Type     string           `json:"type"`
	Default  any              `json:"default,omitempty"`
	Min      *float64         `json:"min,omitempty"`
	Max      *float64         `json:"max,omitempty"`
	Step     *float64         `json:"step,omitempty"`
	Options  []simFieldOption `json:"options,omitempty"`
	Required bool             `json:"required,omitempty"`
}

type simFieldOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

type simRenderManifest struct {
	Patterns []simPatternBinding `json:"patterns"`
}

type simPatternBinding struct {
	Mode   string         `json:"mode"`
	Region string         `json:"region,omitempty"`
	Config map[string]any `json:"config,omitempty"`
}

type simCodeTraceManifest struct {
	SourceCode    string             `json:"sourceCode"`
	Language      string             `json:"language"`
	LineMapping   []simLineMapping   `json:"lineMapping"`
	VariableWatch []simVariableWatch `json:"variableWatch,omitempty"`
}

type simLineMapping struct {
	Line             int    `json:"line"`
	TriggerCondition string `json:"triggerCondition"`
	Annotation       string `json:"annotation,omitempty"`
	HighlightStyle   string `json:"highlightStyle,omitempty"`
}

type simVariableWatch struct {
	Name    string `json:"name"`
	Extract string `json:"extract"`
	Format  string `json:"format,omitempty"`
}

// analyzeBundle 校验归档结构、计算 SHA-256 并执行危险调用静态扫描。
func analyzeBundle(input BundleInput, limits upload.ArchiveLimits) (string, StaticScanReport, bundleManifest, error) {
	if strings.TrimSpace(input.FileName) == "" || len(input.Data) == 0 {
		return "", StaticScanReport{}, bundleManifest{}, apperr.ErrSimBundleUnreadable
	}
	if len(input.Data) > 0 {
		hash := crypto.SHA256Hex(input.Data)
		findings, manifest, err := scanBundleEntries(input.FileName, input.Data, limits)
		if err != nil {
			return "", StaticScanReport{}, bundleManifest{}, apperr.ErrSimBundleUnreadable.WithCause(err)
		}
		if len(findings) > 0 {
			return hash, StaticScanReport{Status: validationFailed, Findings: findings}, bundleManifest{}, nil
		}
		return hash, StaticScanReport{Status: validationPassed}, manifest, nil
	}
	return "", StaticScanReport{}, bundleManifest{}, apperr.ErrSimBundleUnreadable
}

// scanBundleEntries 遍历 ZIP/TAR 普通文件,对代码和 JSON 契约文件执行保守静态扫描。
func scanBundleEntries(name string, data []byte, limits upload.ArchiveLimits) ([]string, bundleManifest, error) {
	findings := []string{}
	var manifestRaw []byte
	err := upload.WalkArchiveFiles(name, data, limits, func(file upload.ArchiveFile) error {
		content, err := upload.ReadArchiveFileContent(file, limits.MaxUnpackedBytes)
		if err != nil {
			return err
		}
		if cleanManifestName(file.Name) == simPackageManifestName {
			manifestRaw = append([]byte(nil), content...)
		}
		if !scanCandidate(file.Name) {
			return nil
		}
		findings = append(findings, scanContent(file.Name, content)...)
		return nil
	})
	if err != nil {
		return nil, bundleManifest{}, err
	}
	if len(manifestRaw) == 0 {
		findings = append(findings, "manifest:missing")
		return findings, bundleManifest{}, nil
	}
	manifest, manifestFindings := parseBundleManifest(manifestRaw)
	if len(manifestFindings) > 0 {
		findings = append(findings, manifestFindings...)
	}
	return findings, manifest, nil
}

// scanCandidate 仅扫描可执行/契约文本文件,避免对图片等资产误报。
func scanCandidate(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".json", ".html", ".htm", ".svg", ".md", ".markdown", ".css":
		return true
	default:
		return false
	}
}

// scanContent 查找危险调用模式并返回可审计的命中项。
func scanContent(name string, content []byte) []string {
	text := string(content)
	findings := []string{}
	for _, item := range dangerousBundlePatterns {
		if item.re.MatchString(text) {
			findings = append(findings, name+":"+item.name)
		}
	}
	return findings
}

// cleanManifestName 允许 manifest 位于归档根或单个顶层目录下,其他同名文件不作为协议入口。
func cleanManifestName(name string) string {
	name = strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
	if name == simPackageManifestName {
		return simPackageManifestName
	}
	parts := strings.Split(name, "/")
	if len(parts) == 2 && parts[1] == simPackageManifestName {
		return simPackageManifestName
	}
	return name
}

// parseBundleManifest 强校验自描述协议,并提取后端运行时需要的最小白名单。
func parseBundleManifest(raw []byte) (bundleManifest, []string) {
	var doc simPackageManifest
	if err := jsonx.DecodeStrictKnownFields(raw, &doc); err != nil {
		return bundleManifest{}, []string{"manifest:invalid-json"}
	}
	return buildBundleManifest(doc)
}

// buildBundleManifest 把 manifest 转为数据库中的审核摘要,同时执行协议结构校验。
func buildBundleManifest(doc simPackageManifest) (bundleManifest, []string) {
	findings := []string{}
	if !simCodePattern.MatchString(strings.TrimSpace(doc.Meta.Code)) || !semverPattern.MatchString(strings.TrimSpace(doc.Meta.Version)) || strings.TrimSpace(doc.Meta.Name) == "" || !categoryPattern.MatchString(strings.TrimSpace(doc.Meta.Category)) {
		findings = append(findings, "manifest:meta-invalid")
	}
	compute, err := computeFromString(doc.Meta.Compute)
	if err != nil || (compute != ComputeFrontend && compute != ComputeBackend) {
		findings = append(findings, "manifest:compute-invalid")
	}
	if len(doc.Interactions) == 0 {
		findings = append(findings, "manifest:interactions-empty")
	}
	schema := InteractionSchema{Events: map[string]InteractionEventSchema{}}
	for _, interaction := range doc.Interactions {
		event, itemFindings := interactionSchemaFromManifest(interaction)
		if len(itemFindings) > 0 {
			findings = append(findings, itemFindings...)
			continue
		}
		emits := strings.TrimSpace(interaction.Emits)
		if _, exists := schema.Events[emits]; exists {
			findings = append(findings, "manifest:interaction-duplicate-event")
			continue
		}
		schema.Events[emits] = event
	}
	if len(doc.Render.Patterns) == 0 || len(doc.Render.Patterns) > 3 {
		findings = append(findings, "manifest:render-pattern-count")
	}
	for _, pattern := range doc.Render.Patterns {
		if _, ok := allowedPatternModes[strings.TrimSpace(pattern.Mode)]; !ok {
			findings = append(findings, "manifest:render-mode-invalid")
		}
	}
	trace, traceFindings := codeTraceAuditFromManifest(doc.CodeTrace)
	findings = append(findings, traceFindings...)
	return bundleManifest{Meta: doc.Meta, InteractionSchema: normalizeInteractionSchema(schema), CodeTrace: trace}, findings
}

// interactionSchemaFromManifest 校验单个交互声明并生成事件白名单。
func interactionSchemaFromManifest(in simInteractionDef) (InteractionEventSchema, []string) {
	findings := []string{}
	id := strings.TrimSpace(in.ID)
	kind := strings.TrimSpace(in.Kind)
	target := strings.TrimSpace(in.Target)
	if target == "" {
		target = "global"
	}
	if id == "" || strings.TrimSpace(in.Label) == "" || !eventTypePattern.MatchString(strings.TrimSpace(in.Emits)) || !validInteractionKind(kind) || (target != "global" && target != "element") {
		return InteractionEventSchema{}, []string{"manifest:interaction-invalid"}
	}
	params := make([]InteractionParam, 0, len(in.Params))
	seen := map[string]struct{}{}
	for _, field := range in.Params {
		param, ok := interactionParamFromManifest(field)
		if !ok {
			findings = append(findings, "manifest:interaction-param-invalid")
			continue
		}
		if _, exists := seen[param.Name]; exists {
			findings = append(findings, "manifest:interaction-param-duplicate")
			continue
		}
		seen[param.Name] = struct{}{}
		params = append(params, param)
	}
	return InteractionEventSchema{InteractionID: id, Kind: kind, Target: target, Params: params}, findings
}

// interactionParamFromManifest 转换字段定义为后端可校验的参数摘要。
func interactionParamFromManifest(in simFieldDef) (InteractionParam, bool) {
	name := strings.TrimSpace(in.Name)
	typ := strings.TrimSpace(in.Type)
	if name == "" || !validFieldType(typ) {
		return InteractionParam{}, false
	}
	out := InteractionParam{Name: name, Type: typ, Required: in.Required, Min: in.Min, Max: in.Max}
	for _, option := range in.Options {
		value := strings.TrimSpace(jsonx.StringFromAny(option.Value))
		if value == "" {
			return InteractionParam{}, false
		}
		out.Options = append(out.Options, value)
	}
	if (typ == "select") != (len(out.Options) > 0) {
		return InteractionParam{}, false
	}
	return out, true
}

// codeTraceAuditFromManifest 校验代码追踪声明并生成不含源码正文的审核摘要。
func codeTraceAuditFromManifest(in *simCodeTraceManifest) (CodeTraceAudit, []string) {
	if in == nil {
		return CodeTraceAudit{}, nil
	}
	source := strings.TrimSpace(in.SourceCode)
	if source == "" || len(source) > 100000 || !validCodeTraceLanguage(in.Language) || len(in.LineMapping) == 0 || len(in.LineMapping) > 500 || len(in.VariableWatch) > 100 {
		return CodeTraceAudit{}, []string{"manifest:code-trace-invalid"}
	}
	lineCount := strings.Count(source, "\n") + 1
	for _, item := range in.LineMapping {
		if item.Line <= 0 || item.Line > lineCount || strings.TrimSpace(item.TriggerCondition) == "" || !validHighlightStyle(item.HighlightStyle) {
			return CodeTraceAudit{}, []string{"manifest:code-trace-line-invalid"}
		}
	}
	for _, item := range in.VariableWatch {
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Extract) == "" || !validVariableFormat(item.Format) {
			return CodeTraceAudit{}, []string{"manifest:code-trace-variable-invalid"}
		}
	}
	return CodeTraceAudit{Enabled: true, Language: strings.TrimSpace(in.Language), LineCount: lineCount, MappingCount: len(in.LineMapping), VariableCount: len(in.VariableWatch)}, nil
}

// validInteractionKind 校验交互声明类型是否落在受控封闭集。
func validInteractionKind(value string) bool {
	switch value {
	case "button", "slider", "hold", "select-element", "drag", "form":
		return true
	default:
		return false
	}
}

// validFieldType 校验交互字段类型是否为后端可审核的封闭类型。
func validFieldType(value string) bool {
	switch value {
	case "number", "string", "boolean", "select", "range":
		return true
	default:
		return false
	}
}

// validCodeTraceLanguage 校验代码追踪协议语言是否在受控白名单内。
func validCodeTraceLanguage(value string) bool {
	switch strings.TrimSpace(value) {
	case "solidity", "rust", "go", "javascript", "pseudocode":
		return true
	default:
		return false
	}
}

// validHighlightStyle 校验代码追踪高亮样式是否是支持的有限集合。
func validHighlightStyle(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "normal", "success", "error":
		return true
	default:
		return false
	}
}

// validVariableFormat 校验变量提取格式是否是支持的有限集合。
func validVariableFormat(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "hex", "number", "string", "bool":
		return true
	default:
		return false
	}
}

// normalizeInteractionSchema 补齐交互白名单索引,方便运行时快速查找。
func normalizeInteractionSchema(schema InteractionSchema) InteractionSchema {
	if len(schema.Events) == 0 {
		schema.Events = map[string]InteractionEventSchema{}
		return schema
	}
	for event, item := range schema.Events {
		if item.ParamIndex == nil {
			item.ParamIndex = map[string]InteractionParam{}
			for _, param := range item.Params {
				item.ParamIndex[param.Name] = param
			}
			schema.Events[event] = item
		}
	}
	return schema
}
