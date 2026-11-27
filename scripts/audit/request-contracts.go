// 校验 HTTP 请求的实际字段、ID 类型、媒体类型及 WebSocket 票据位置。
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"
)

type requestMapping struct {
	module, name string
	operations   []string
	formFields   []string
	persistent   bool
}

var mappings = []requestMapping{
	{module: "admin", name: "AuditExportRequest", operations: []string{"adminExportAudit"}, persistent: true},
	{module: "content", name: "UploadAttachmentRequest", operations: []string{"contentUploadAttachment"}, formFields: []string{"resource_id", "file"}},
	{module: "content", name: "BatchItemsRequest", operations: []string{"contentBatchItems"}},
	{module: "content", name: "SystemImportRequest", operations: []string{"contentSystemImport"}},
	{module: "experiment", name: "ReportUploadRequest", operations: []string{"experimentSubmitReport"}, formFields: []string{"file"}},
	{module: "grade", name: "AppealDecisionRequest", operations: []string{"gradeAcceptAppeal", "gradeRejectAppeal"}},
	{module: "grade", name: "TranscriptBatchRequest", operations: []string{"gradeGenerateTranscriptBatch"}},
	{module: "grade", name: "RecomputeRequest", operations: []string{"gradeRecomputeStudentGrade"}},
	{module: "grade", name: "WarningScanRequest", operations: []string{"gradeScanWarnings"}},
	{module: "identity", name: "ImportPreviewRequest", operations: []string{"identityPreviewAccountImport"}, formFields: []string{"type", "file"}},
	{module: "identity", name: "ImportPreviewRequest", operations: []string{"identityPreviewOrgImport"}, formFields: []string{"file"}},
	{module: "identity", name: "TenantLogoUploadRequest", operations: []string{"identityUploadTenantLogo"}, formFields: []string{"file"}},
	{module: "judge", name: "SubmitTaskRequest", operations: []string{"judgeSubmitTask"}},
	{module: "judge", name: "RejudgeBatchRequest", operations: []string{"judgeRejudgeBatch"}},
	{module: "judge", name: "FingerprintSimilarityRequest", operations: []string{"judgeSimilarity"}},
	{module: "notify", name: "SendRequest", operations: []string{"notifySend"}},
	{module: "notify", name: "PushRequest", operations: []string{"notifyPush"}},
	{module: "notify", name: "PreferenceRequest", operations: []string{"notifyUpsertPreference"}},
	{module: "sim", name: "SubmitPackageRequest", operations: []string{"simSubmitPackage", "simUpdatePackage"}, formFields: []string{"code", "version", "name", "category", "scale_limit", "bundle"}},
	{module: "sim", name: "RejectReviewRequest", operations: []string{"simRejectReview"}},
	{module: "sim", name: "CreateSessionRequest", operations: []string{"simCreateSession"}},
	{module: "sim", name: "ReportActionRequest", operations: []string{"simReportAction"}},
	{module: "sim", name: "RecycleRequest", operations: []string{"simRecycleSessions"}},
	{module: "sim", name: "CreateShareRequest", operations: []string{"simShareSession"}},
	{module: "teaching", name: "CourseCoverUploadRequest", operations: []string{"teachingUploadCourseCover"}, formFields: []string{"file"}},
	{module: "teaching", name: "LessonMaterialUploadRequest", operations: []string{"teachingUploadLessonMaterial"}, formFields: []string{"file"}},
	{module: "teaching", name: "CloneCourseRequest", operations: []string{"teachingCloneCourse"}},
	{module: "teaching", name: "DraftRequest", operations: []string{"teachingSaveDraft"}},
	{module: "teaching", name: "GradeSubmissionRequest", operations: []string{"teachingGradeSubmission"}},
	{module: "teaching", name: "PinRequest", operations: []string{"teachingPinPost", "teachingPinAnnouncement"}},
	{module: "teaching", name: "CourseGradeExportRequest", operations: []string{"teachingExportGrades"}, persistent: true},
}

type audit struct {
	root       string
	components map[string]any
	types      map[string]*ast.StructType
	errors     []string
}

func object(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func (a *audit) fail(format string, values ...any) {
	a.errors = append(a.errors, fmt.Sprintf(format, values...))
}

func (a *audit) resolve(value any) map[string]any {
	current := object(value)
	seen := map[string]bool{}
	for current != nil {
		ref, ok := current["$ref"].(string)
		if !ok {
			return current
		}
		if !strings.HasPrefix(ref, "#/components/") || seen[ref] {
			a.fail("unsupported or cyclic local reference %q", ref)
			return nil
		}
		seen[ref] = true
		current = a.components
		for _, part := range strings.Split(strings.TrimPrefix(ref, "#/components/"), "/") {
			current = object(current[part])
		}
	}
	return nil
}

func (a *audit) readYAML(path string) (map[string]any, error) {
	data, err := os.ReadFile(filepath.Join(a.root, path))
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := yaml.UnmarshalStrict(data, &result); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return result, nil
}

func jsonFields(body *ast.StructType) map[string]ast.Expr {
	fields := map[string]ast.Expr{}
	for _, field := range body.Fields.List {
		if field.Tag == nil {
			continue
		}
		tag, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			continue
		}
		name := strings.Split(reflect.StructTag(tag).Get("json"), ",")[0]
		if name != "" && name != "-" {
			fields[name] = field.Type
		}
	}
	return fields
}

func (a *audit) schemaMatches(schema map[string]any, typ ast.Expr, module string) bool {
	schema = a.resolve(schema)
	if schema == nil {
		return false
	}
	switch value := typ.(type) {
	case *ast.StarExpr:
		return a.schemaMatches(schema, value.X, module)
	case *ast.ArrayType:
		return schema["type"] == "array" && a.schemaMatches(object(schema["items"]), value.Elt, module)
	case *ast.SelectorExpr:
		pkg, _ := value.X.(*ast.Ident)
		if pkg == nil {
			return false
		}
		if pkg.Name == "ids" && value.Sel.Name == "ID" {
			return schema["type"] == "string" && schema["pattern"] == "^[1-9][0-9]*$"
		}
		if pkg.Name == "time" && value.Sel.Name == "Time" {
			return schema["type"] == "string" && schema["format"] == "date-time"
		}
		return pkg.Name == "json" && value.Sel.Name == "RawMessage"
	case *ast.MapType:
		return schema["type"] == "object"
	case *ast.Ident:
		switch value.Name {
		case "string":
			return schema["type"] == "string"
		case "bool":
			return schema["type"] == "boolean"
		case "int", "int16", "int32", "int64":
			return schema["type"] == "integer"
		case "float64":
			return schema["type"] == "number"
		}
		if nested := a.types[module+":"+value.Name]; nested != nil {
			return a.fieldsMatch(schema, jsonFields(nested), module)
		}
	}
	return false
}

func (a *audit) fieldsMatch(schema map[string]any, fields map[string]ast.Expr, module string) bool {
	properties := object(schema["properties"])
	if schema["type"] != "object" || schema["additionalProperties"] != false || len(properties) != len(fields) {
		return false
	}
	for name, typ := range fields {
		if !a.schemaMatches(object(properties[name]), typ, module) {
			return false
		}
	}
	return true
}

func (a *audit) run() error {
	root, err := a.readYAML("backend/api/openapi.yaml")
	if err != nil {
		return err
	}
	a.components = object(root["components"])
	modules := filepath.Join(a.root, "backend/internal/modules")
	if err := filepath.WalkDir(modules, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "internal" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		module := filepath.Base(filepath.Dir(path))
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				typ, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if body, ok := typ.Type.(*ast.StructType); ok {
					a.types[module+":"+typ.Name.Name] = body
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	operations := map[string]map[string]any{}
	files, err := filepath.Glob(filepath.Join(a.root, "backend/api/paths/*.yaml"))
	if err != nil {
		return err
	}
	for _, file := range files {
		rel, err := filepath.Rel(a.root, file)
		if err != nil {
			return err
		}
		paths, err := a.readYAML(rel)
		if err != nil {
			return err
		}
		for _, value := range paths {
			for _, value := range object(value) {
				op := object(value)
				name, _ := op["operationId"].(string)
				if name == "" {
					continue
				}
				if operations[name] != nil {
					a.fail("duplicate operationId %s", name)
				}
				operations[name] = op
				ws, _ := op["x-chaimir-websocket"].(bool)
				ticket, wsAuth := false, false
				parameters, _ := op["parameters"].([]any)
				for _, parameter := range parameters {
					p := a.resolve(parameter)
					if p["name"] == "ticket" && p["in"] == "query" && p["required"] == true {
						ticket = true
					}
				}
				security, _ := op["security"].([]any)
				for _, item := range security {
					if _, ok := object(item)["wsTicketAuth"]; ok {
						wsAuth = true
					}
				}
				if ws && (!ticket || !wsAuth) || !ws && (wsAuth || ticket && name != "identityCASCallback") {
					a.fail("%s: WebSocket ticket/security declared on wrong operation", name)
				}
			}
		}
	}
	for _, mapping := range mappings {
		key := mapping.module + ":" + mapping.name
		body := a.types[key]
		if body == nil {
			a.fail("missing DTO %s", key)
			continue
		}
		fields := jsonFields(body)
		for name, typ := range fields {
			// request_id 是业务幂等来源标识，不是雪花 ID。
			if mapping.module == "notify" && name == "request_id" {
				continue
			}
			if !strings.HasSuffix(name, "_id") && !strings.HasSuffix(name, "_ids") && !(mapping.module == "notify" && name == "receivers") {
				continue
			}
			if array, ok := typ.(*ast.ArrayType); ok {
				typ = array.Elt
			}
			selector, ok := typ.(*ast.SelectorExpr)
			if !ok {
				a.fail("%s.%s: public IDs must use ids.ID", key, name)
				continue
			}
			pkg, _ := selector.X.(*ast.Ident)
			if pkg == nil || pkg.Name != "ids" || selector.Sel.Name != "ID" {
				a.fail("%s.%s: public IDs must use ids.ID", key, name)
			}
		}
		if mapping.module == "sim" && (mapping.name == "CreateSessionRequest" || mapping.name == "RecycleRequest") {
			if fields["tenant_id"] != nil || fields["source_ref"] != nil {
				a.fail("%s: signed identity must not be accepted from JSON", key)
			}
		}
		for _, name := range mapping.operations {
			op := operations[name]
			if op == nil {
				a.fail("missing operation %s", name)
				continue
			}
			if mapping.persistent {
				if op["requestBody"] != nil || len(fields) != 0 {
					a.fail("%s: persistent record exposed as request body", name)
				}
				continue
			}
			request := a.resolve(op["requestBody"])
			media := "application/json"
			if mapping.formFields != nil {
				media = "multipart/form-data"
			}
			schema := a.resolve(object(object(request["content"])[media])["schema"])
			if request["required"] != true || schema == nil {
				a.fail("%s: missing required %s request", name, media)
				continue
			}
			if mapping.formFields == nil {
				if !a.fieldsMatch(schema, fields, mapping.module) {
					a.fail("%s: JSON schema differs from %s fields/types", name, key)
				}
				continue
			}
			properties := object(schema["properties"])
			if len(properties) != len(mapping.formFields) || schema["type"] != "object" {
				a.fail("%s: multipart fields differ from actual upload boundary", name)
			}
			for _, field := range mapping.formFields {
				property := object(properties[field])
				if property["type"] != "string" || (field == "file" || field == "bundle") && property["format"] != "binary" {
					a.fail("%s: invalid multipart field %s", name, field)
				}
			}
		}
	}
	if len(a.errors) != 0 {
		slices.Sort(a.errors)
		return fmt.Errorf("%s", strings.Join(a.errors, "\n"))
	}
	fmt.Printf("HTTP 请求契约通过：%d 项边界映射，%d 个操作的 WebSocket 票据位置。\n", len(mappings), len(operations))
	return nil
}

func main() {
	root := flag.String("root", "..", "repository root")
	flag.Parse()
	a := audit{root: *root, types: map[string]*ast.StructType{}}
	if err := a.run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
