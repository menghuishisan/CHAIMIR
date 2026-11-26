package sim

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed runtime_protocol.schema.json
var runtimeProtocolJSON []byte

// 只装载仓库生成的契约，不允许 schema 引用网络或文件。
var runtimeProtocols = sync.OnceValues(func() (map[string]*jsonschema.Schema, error) {
	var resource any
	if err := json.Unmarshal(runtimeProtocolJSON, &resource); err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	const location = "urn:chaimir:sim:runtime"
	if err := compiler.AddResource(location, resource); err != nil {
		return nil, err
	}
	compiled := make(map[string]*jsonschema.Schema)
	for _, name := range []string{"TeachingFrame", "SimPackageDescriptor", "RuntimeFrame"} {
		schema, err := compiler.Compile(location + "#/definitions/" + name)
		if err != nil {
			return nil, err
		}
		compiled[name] = schema
	}
	return compiled, nil
})

func validateRuntimeProtocol(name string, value any) error {
	schemas, err := runtimeProtocols()
	if err != nil {
		return frameError(fmt.Sprintf("加载运行契约失败: %v", err))
	}
	if err := schemas[name].Validate(value); err != nil {
		return frameError(fmt.Sprintf("%s: %v", name, err))
	}
	return nil
}
