// 本程序通过 Optimism 官方测试配置生成受控 OP-Stack 初始化输入。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	chaimirgenesis "github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	chaimirconfig "github.com/ethereum-optimism/optimism/op-e2e/config"
)

func main() {
	out := flag.String("out", "", "输出目录")
	flag.Parse()
	if *out == "" {
		panic("必须提供 -out")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(err)
	}
	write := func(name string, value any) {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			panic(fmt.Errorf("序列化 %s: %w", name, err))
		}
		if err := os.WriteFile(filepath.Join(*out, name), append(data, '\n'), 0o644); err != nil {
			panic(fmt.Errorf("写入 %s: %w", name, err))
		}
	}
	allocType := chaimirconfig.DefaultAllocType
	write("l1-allocs.json", chaimirconfig.L1Allocs(allocType))
	write("l1-deployments.json", chaimirconfig.L1Deployments(allocType))
	write("deploy-config.json", chaimirconfig.DeployConfig(allocType))
	write("l2-allocs.json", chaimirconfig.L2Allocs(allocType, chaimirgenesis.L2AllocsGranite))
	fmt.Println("官方 OP-Stack 受控配置与 L1/L2 allocs 已生成")
}
