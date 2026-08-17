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
// 不比对 compute:表单与 manifest 都不含它,执行位置由服务端按 author_type 派生。
func validateBundleManifestMatchesRequest(manifest bundleManifest, req SubmitPackageRequest) error {
	if strings.TrimSpace(manifest.Meta.Code) != req.Code || strings.TrimSpace(manifest.Meta.Version) != req.Version || strings.TrimSpace(manifest.Meta.Name) != req.Name || strings.TrimSpace(manifest.Meta.Category) != req.Category {
		return apperr.ErrSimPackageValidationFailed
	}
	if !scaleLimitMatchesRequest(manifest.Meta.ScaleLimit, req.ScaleLimit) {
		return apperr.ErrSimPackageValidationFailed
	}
	if len(manifest.InteractionSchema.Events) == 0 {
		return apperr.ErrSimPackageValidationFailed
	}
	return nil
}

// dangerousBundlePatterns 是提交时扫描的危险调用模式。
//
// 它是**给审核人的信号,不是隔离边界**:正则拦不住 `globalThis['fe'+'tch']` 这类拼接,
// 动态语言下的能力访问无法靠模式匹配穷尽。真正的边界是隔离容器 —— 扩展包在 deny-all 网络、
// 只读根、无凭据的 Pod 内执行,`fetch` 在那里本就无处可达(见 docs/04-仿真可视化引擎/07-安全设计.md §3、§5)。
// 故扫描命中会写进审核报告提请人工重点查看,而不再作为准入的唯一硬门禁。

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
var allowedRenderPatternRoles = map[string]struct{}{"primary": {}, "evidence": {}, "timeline": {}, "metrics": {}, "trace": {}, "checkpoints": {}}

// entryPathPattern 限定入口模块只能是归档内相对路径的 .mjs 文件。
//
// 只接受 .mjs:归档内没有 package.json 时 Node 按 CJS 解析 .js,而扩展包必须默认导出
// SimPackage(ESM 语义)。允许 .js 会让"能不能装配"取决于归档里有没有 type:module ——
// 同一份代码在不同打包方式下表现不同,收敛成单一扩展名消掉这类不确定性。
var entryPathPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*\.mjs$`)

// bundleManifest 保存后端可审核的自描述协议摘要,不承载可执行函数正文。
type bundleManifest struct {
	Meta              simManifestMeta
	InteractionSchema InteractionSchema
	CodeTrace         CodeTraceAudit
}

// simPackageManifest 是 sim-package.json 的严格解析结构,只包含后端审核允许的顶层字段。
type simPackageManifest struct {
	Meta         simManifestMeta       `json:"meta"`
	Interactions []simInteractionDef   `json:"interactions"`
	Render       simRenderManifest     `json:"render"`
	Narrative    []map[string]any      `json:"narrative,omitempty"`
	CodeTrace    *simCodeTraceManifest `json:"codeTrace,omitempty"`
	Checkpoints  []simCheckpointDef    `json:"checkpoints"`
}

// simManifestMeta 保存包元信息和规模上限,需要与上传表单保持完全一致。
// 不含 compute:执行位置按 author_type 派生,manifest 与表单都不声明它。
type simManifestMeta struct {
	Code       string         `json:"code"`
	Name       string         `json:"name"`
	Category   string         `json:"category"`
	Version    string         `json:"version"`
	Entry      string         `json:"entry,omitempty"`
	ScaleLimit map[string]any `json:"scale_limit,omitempty"`
}

// simInteractionDef 描述一个可由通用工作台发出的受控交互。
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

// simFieldDef 描述交互 payload 中单个参数的可校验约束。
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

// simFieldOption 描述 select 参数允许的枚举值。
type simFieldOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// simRenderManifest 只保留 TeachingFrame 静态审核声明,不包含执行期渲染函数。
type simRenderManifest struct {
	Protocol string              `json:"protocol"`
	Patterns []simPatternBinding `json:"patterns"`
}

// simPatternBinding 声明一个受控渲染模式及其可承担的教学职责。
type simPatternBinding struct {
	ID    string   `json:"id"`
	Mode  string   `json:"mode"`
	Roles []string `json:"roles,omitempty"`
}

// simCodeTraceManifest 保存代码追踪配置,后端只提取审核摘要不保存源码正文。
type simCodeTraceManifest struct {
	SourceCode    string             `json:"sourceCode"`
	Language      string             `json:"language"`
	LineMapping   []simLineMapping   `json:"lineMapping"`
	VariableWatch []simVariableWatch `json:"variableWatch,omitempty"`
}

// simLineMapping 描述源码行与仿真阶段或事件的对应关系。
type simLineMapping struct {
	Line             int    `json:"line"`
	TriggerCondition string `json:"triggerCondition"`
	Annotation       string `json:"annotation,omitempty"`
	HighlightStyle   string `json:"highlightStyle,omitempty"`
}

// simVariableWatch 描述代码追踪面板允许观察的状态变量。
type simVariableWatch struct {
	Name    string `json:"name"`
	Extract string `json:"extract"`
	Format  string `json:"format,omitempty"`
}

// simCheckpointDef 声明仿真检查点锚点,判分逻辑仍由受控评测链路处理。
type simCheckpointDef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
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

// scanBundleEntries 遍历 ZIP/TAR 普通文件,对代码和 JSON 契约文件执行保守静态扫描,
// 并确认 manifest 声明的入口模块在归档内真实存在。
func scanBundleEntries(name string, data []byte, limits upload.ArchiveLimits) ([]string, bundleManifest, error) {
	findings := []string{}
	var manifestRaw []byte
	members := map[string]struct{}{}
	err := upload.WalkArchiveFiles(name, data, limits, func(file upload.ArchiveFile) error {
		content, err := upload.ReadArchiveFileContent(file, limits.MaxUnpackedBytes)
		if err != nil {
			return err
		}
		members[cleanMemberName(file.Name)] = struct{}{}
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
	// 入口模块声明合法不代表它真的在包里:容器装配时找不到入口只能报运行失败,
	// 而那已经是学生打开场景之后了。在上传边界就确认存在性,把缺陷挡在审核之前。
	if entry := strings.TrimSpace(manifest.Meta.Entry); entry != "" {
		if _, exists := members[entry]; !exists {
			findings = append(findings, "manifest:entry-not-in-archive")
		}
	}
	return findings, manifest, nil
}

// cleanMemberName 归一化归档成员名,剥掉归档工具自动包的唯一顶层目录,
// 使成员名与 manifest 里的相对路径处于同一坐标系。
func cleanMemberName(name string) string {
	value := strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"), "./")
	parts := strings.Split(value, "/")
	if len(parts) > 1 && strings.Contains(strings.Join(parts[1:], "/"), simPackageManifestName) {
		return strings.Join(parts[1:], "/")
	}
	return value
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
// 扩展包必须声明入口模块(隔离容器据此装配),故 requiresEntry 恒为真。
func parseBundleManifest(raw []byte) (bundleManifest, []string) {
	var doc simPackageManifest
	if err := jsonx.DecodeStrictKnownFields(raw, &doc); err != nil {
		return bundleManifest{}, []string{"manifest:invalid-json"}
	}
	return buildBundleManifest(doc, true)
}

// entryFindings 校验入口模块声明:扩展包必须有且路径合法,内置包不得声明。
//
// 路径校验必须严格:entry 来自不可信上传,容器会按它拼出写入与 import 路径,
// 绝对路径、`..` 与盘符都会把装配点带出工作目录。
func entryFindings(entry string, requiresEntry bool) []string {
	value := strings.TrimSpace(entry)
	if !requiresEntry {
		if value != "" {
			return []string{"manifest:entry-not-allowed"}
		}
		return nil
	}
	if value == "" {
		return []string{"manifest:entry-missing"}
	}
	if len(value) > 255 || !entryPathPattern.MatchString(value) {
		return []string{"manifest:entry-invalid"}
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return []string{"manifest:entry-invalid"}
		}
	}
	return nil
}

// buildBundleManifest 把 manifest 转为数据库中的审核摘要,同时执行协议结构校验。
// requiresEntry 为真时(扩展包)必须声明合法入口模块;内置包由 registry 按 code 装配,不得声明 entry。
func buildBundleManifest(doc simPackageManifest, requiresEntry bool) (bundleManifest, []string) {
	findings := []string{}
	if !simCodePattern.MatchString(strings.TrimSpace(doc.Meta.Code)) || !semverPattern.MatchString(strings.TrimSpace(doc.Meta.Version)) || strings.TrimSpace(doc.Meta.Name) == "" || !categoryPattern.MatchString(strings.TrimSpace(doc.Meta.Category)) {
		findings = append(findings, "manifest:meta-invalid")
	}
	if !validManifestScaleLimit(doc.Meta.ScaleLimit) {
		findings = append(findings, "manifest:scale-limit-invalid")
	}
	findings = append(findings, entryFindings(doc.Meta.Entry, requiresEntry)...)
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
	findings = append(findings, renderManifestFindings(doc.Render)...)
	trace, traceFindings := codeTraceAuditFromManifest(doc.CodeTrace)
	findings = append(findings, traceFindings...)
	findings = append(findings, checkpointFindingsFromManifest(doc.Checkpoints)...)
	return bundleManifest{Meta: doc.Meta, InteractionSchema: normalizeInteractionSchema(schema), CodeTrace: trace}, findings
}

// renderManifestFindings 校验 TeachingFrame 的静态渲染声明。
func renderManifestFindings(render simRenderManifest) []string {
	findings := []string{}
	if strings.TrimSpace(render.Protocol) != "teaching-frame" {
		findings = append(findings, "manifest:render-protocol-invalid")
	}
	if len(render.Patterns) == 0 || len(render.Patterns) > 3 {
		findings = append(findings, "manifest:render-pattern-count")
		return findings
	}
	seen := map[string]struct{}{}
	for _, pattern := range render.Patterns {
		id := strings.TrimSpace(pattern.ID)
		if !payloadKeyPattern.MatchString(id) {
			findings = append(findings, "manifest:render-pattern-id-invalid")
		}
		if _, exists := seen[id]; exists {
			findings = append(findings, "manifest:render-pattern-duplicate")
		}
		seen[id] = struct{}{}
		if _, ok := allowedPatternModes[strings.TrimSpace(pattern.Mode)]; !ok {
			findings = append(findings, "manifest:render-mode-invalid")
		}
		if len(pattern.Roles) == 0 {
			findings = append(findings, "manifest:render-pattern-role-empty")
		}
		for _, role := range pattern.Roles {
			if _, ok := allowedRenderPatternRoles[strings.TrimSpace(role)]; !ok {
				findings = append(findings, "manifest:render-pattern-role-invalid")
			}
		}
	}
	return findings
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
	if !validInteractionLabelTag(in.LabelTag) {
		return InteractionEventSchema{}, []string{"manifest:interaction-invalid"}
	}
	if kind == "select-element" && target != "element" {
		return InteractionEventSchema{}, []string{"manifest:interaction-invalid"}
	}
	if target == "element" && strings.TrimSpace(in.ElementFilter) == "" {
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
		if reservedInteractionPayloadParam(param.Name) {
			findings = append(findings, "manifest:interaction-param-reserved")
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

// reservedInteractionPayloadParam 保留平台通用控件字段,防止仿真包把系统字段声明为算法参数。
func reservedInteractionPayloadParam(name string) bool {
	switch strings.TrimSpace(name) {
	case "target", "active", "phase", "startX", "startY", "currentX", "currentY", "deltaX", "deltaY":
		return true
	default:
		return false
	}
}

// interactionParamFromManifest 转换字段定义为后端可校验的参数摘要。
func interactionParamFromManifest(in simFieldDef) (InteractionParam, bool) {
	name := strings.TrimSpace(in.Name)
	typ := strings.TrimSpace(in.Type)
	if name == "" || !payloadKeyPattern.MatchString(name) || !validFieldType(typ) {
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
		return CodeTraceAudit{}, []string{"manifest:code-trace-missing"}
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

// checkpointFindingsFromManifest 校验检查点声明,确保后续检查点上报有受控锚点。
func checkpointFindingsFromManifest(in []simCheckpointDef) []string {
	if len(in) == 0 || len(in) > 64 {
		return []string{"manifest:checkpoint-invalid"}
	}
	seen := map[string]struct{}{}
	for _, checkpoint := range in {
		id := strings.TrimSpace(checkpoint.ID)
		if !checkpointIDPattern.MatchString(id) || strings.TrimSpace(checkpoint.Label) == "" || len(checkpoint.Label) > 128 {
			return []string{"manifest:checkpoint-invalid"}
		}
		if _, exists := seen[id]; exists {
			return []string{"manifest:checkpoint-duplicate"}
		}
		seen[id] = struct{}{}
	}
	return nil
}

// validManifestScaleLimit 校验 manifest 中的规模上限字段,与前端 Worker 强制项保持一致。
func validManifestScaleLimit(value map[string]any) bool {
	nodes, nodesOK := positiveJSONInt(value["nodes"])
	maxTick, tickOK := positiveJSONInt(value["max_tick"])
	maxEvents, eventsOK := positiveJSONInt(value["max_events"])
	return nodesOK && tickOK && eventsOK && nodes <= 10000 && maxTick <= 100000 && maxEvents <= 100000
}

// scaleLimitMatchesRequest 确认 manifest 与上传表单中的规模上限同源。
func scaleLimitMatchesRequest(manifest map[string]any, raw []byte) bool {
	request, err := jsonx.ObjectMapStrict(raw)
	if err != nil {
		return false
	}
	return jsonx.Equal(manifest, request)
}

// positiveJSONInt 从 JSON 数字或数字字符串读取正整数。
func positiveJSONInt(value any) (int, bool) {
	out := jsonx.IntFromAny(value)
	return out, out > 0
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

// validInteractionLabelTag 校验交互视觉标签是否落在封闭四值集(见 docs/04-仿真可视化引擎/03 §3.3)。
// 空值合法,运行时按 normal 处理;但未知值必须在上架前拒绝 ——
// 标签决定按钮配色与攻击类的二次确认,认不出的标签会让扩展包的破坏性操作画成普通推进色。
func validInteractionLabelTag(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "normal", "recover", "perturb", "attack":
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
