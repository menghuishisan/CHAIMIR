// rules_composition_params 统一验证目录声明的编排参数,不加载外部schema或注入默认值。
package sandbox

import (
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// validateComponentParams 由预览和最终编译共用标准校验器,映射授权仍由M2控制。
func validateComponentParams(params map[string]any, schema map[string]any) error {
	if len(schema) == 0 {
		if len(params) > 0 {
			return fmt.Errorf("组件未声明可配置参数")
		}
		return nil
	}
	if typ, _ := schema["type"].(string); typ != "" && typ != "object" {
		return fmt.Errorf("参数 schema 必须是 object")
	}
	// 每个可注入项必须在目录中有明确属性,不能用additionalProperties开放任意配置。
	properties, _ := schema["properties"].(map[string]any)
	for name := range params {
		if _, declared := properties[name]; !declared {
			return fmt.Errorf("包含未声明参数 %s", name)
		}
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	// 空scheme加载器仅保留库内置的标准元schema,禁止$ref访问网络或文件。
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	const resource = "urn:chaimir:sandbox:parameters"
	if err := compiler.AddResource(resource, schema); err != nil {
		return fmt.Errorf("参数 schema 读取失败: %w", err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return fmt.Errorf("参数 schema 编译失败: %w", err)
	}
	if params == nil {
		params = map[string]any{}
	}
	if err := compiled.Validate(params); err != nil {
		return fmt.Errorf("组件参数不符合目录约束: %w", err)
	}
	return nil
}
