// sim builtin_catalog 文件负责把平台内置仿真包标准库入库,是内置包唯一的写入路径。
//
// 为什么需要它:`GET /sim/packages` 读的是 sim_package 表,而内置包的权威声明是前端
// `@chaimir/sim-sdk` 的 TypeScript 源码(41 个包的 meta/interactions/render/codeTrace/checkpoints)。
// 两者之间必须有一次部署期同步,否则生产库里没有任何内置包 —— 学生仿真实验室、实验编排的场景
// 选择、课时的仿真形态全都取不到数据。
//
// 清单由 `scripts/codegen/export-sim-builtin-catalog.mjs` 从 sim-sdk 导出为标准
// `sim-package.json` 形态,本文件 go:embed 后复用**与扩展包完全相同**的 parseBundleManifest
// 做协议校验 —— 内置包不走第二套解析,也不因为"是平台自己的"就跳过校验。
package sim

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"
)

// builtinCatalogFile 是导出产物名,与前端导出脚本的输出路径一一对应。
const builtinCatalogFile = "builtin_catalog.json"

// builtinBundleRefPrefix 是内置包的逻辑装配来源,不是对象存储地址。
// 内置包内核由 sim-sdk 在 Worker 内装配(见 docs/04-仿真可视化引擎/05-接口设计.md),
// 因此这里只登记"从哪装配",运行授权也只回 builtin_code 而不签下载令牌。
const builtinBundleRefPrefix = "builtin://sim-sdk/"

//go:embed builtin_catalog.json
var builtinCatalogFS embed.FS

// builtinCatalogDoc 是导出产物的顶层结构,只接受已知字段。
type builtinCatalogDoc struct {
	Source    string               `json:"source"`
	Generator string               `json:"generator"`
	Packages  []simPackageManifest `json:"packages"`
}

// BuiltinSyncResult 汇总一次内置包同步的结果,供部署日志核对。
type BuiltinSyncResult struct {
	Upserted int
	Archived []string
}

// SyncBuiltinPackages 把内置仿真包标准库幂等写入 sim_package。
//
// 三步:①解析并校验嵌入清单(与扩展包同一套 manifest 校验);②按 (code, version) upsert;
// ③把已从标准库移除的内置包置为已下架 —— 不物理删除,因为历史实验定义与仿真会话按
// (code, version) 引用它们,删掉会让旧实验取不到场景。
func SyncBuiltinPackages(ctx context.Context, store Store, ids snowflake.Generator) (BuiltinSyncResult, error) {
	if store == nil || ids == nil {
		return BuiltinSyncResult{}, apperr.ErrInternal.WithCause(fmt.Errorf("内置仿真包同步缺少 store 或 ID 生成器"))
	}
	packages, err := loadBuiltinCatalog()
	if err != nil {
		return BuiltinSyncResult{}, err
	}

	out := BuiltinSyncResult{}
	liveKeys := make([]string, 0, len(packages))
	for _, pkg := range packages {
		liveKeys = append(liveKeys, pkg.Code+"@"+pkg.Version)
	}
	sort.Strings(liveKeys)

	if err := store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		for _, pkg := range packages {
			pkg.ID = ids.Generate()
			if _, err := tx.UpsertBuiltinPackage(ctx, pkg); err != nil {
				return apperr.ErrSimPackageInvalid.WithCause(fmt.Errorf("写入内置仿真包 %s@%s 失败: %w", pkg.Code, pkg.Version, err))
			}
			out.Upserted++
		}
		retired, err := tx.ArchiveRetiredBuiltinPackages(ctx, liveKeys)
		if err != nil {
			return apperr.ErrSimPackageInvalid.WithCause(fmt.Errorf("下架已移除内置仿真包失败: %w", err))
		}
		for _, item := range retired {
			out.Archived = append(out.Archived, item.Code+"@"+item.Version)
		}
		return nil
	}); err != nil {
		return BuiltinSyncResult{}, err
	}
	return out, nil
}

// loadBuiltinCatalog 解析嵌入清单并转换为领域模型。
// 任一包协议不合法即整体失败:内置标准库是平台交付物,部分入库会让仿真实验室出现残缺场景。
func loadBuiltinCatalog() ([]Package, error) {
	raw, err := builtinCatalogFS.ReadFile(builtinCatalogFile)
	if err != nil {
		return nil, apperr.ErrInternal.WithCause(fmt.Errorf("读取内置仿真包清单失败: %w", err))
	}
	var doc builtinCatalogDoc
	if err := jsonx.DecodeStrictKnownFields(raw, &doc); err != nil {
		return nil, apperr.ErrInternal.WithCause(fmt.Errorf("解析内置仿真包清单失败: %w", err))
	}
	if len(doc.Packages) == 0 {
		return nil, apperr.ErrInternal.WithCause(fmt.Errorf("内置仿真包清单为空"))
	}

	seen := make(map[string]struct{}, len(doc.Packages))
	out := make([]Package, 0, len(doc.Packages))
	for _, item := range doc.Packages {
		pkg, err := builtinPackageFromManifest(item)
		if err != nil {
			return nil, err
		}
		key := pkg.Code + "@" + pkg.Version
		if _, exists := seen[key]; exists {
			return nil, apperr.ErrInternal.WithCause(fmt.Errorf("内置仿真包 %s 在清单中重复", key))
		}
		seen[key] = struct{}{}
		out = append(out, pkg)
	}
	return out, nil
}

// builtinPackageFromManifest 用与扩展包相同的 manifest 校验把清单条目转成领域模型。
// 三条内置包专属约束:code 必须带 builtin__ 前缀(与数据库 CHECK 及前端前缀判定同源)、
// 不得声明 entry(内置包由 sim-sdk registry 按 code 装配,不走归档装配路径)、
// bundle_hash 由协议内容派生(内置包没有归档字节,用协议摘要表达版本完整性)。
func builtinPackageFromManifest(doc simPackageManifest) (Package, error) {
	code := strings.TrimSpace(doc.Meta.Code)
	version := strings.TrimSpace(doc.Meta.Version)
	if !strings.HasPrefix(code, builtinSimCodePrefix) {
		return Package{}, apperr.ErrInternal.WithCause(fmt.Errorf("内置仿真包 %q 缺少 %s 前缀", code, builtinSimCodePrefix))
	}
	manifest, findings := buildBundleManifest(doc, false)
	if len(findings) > 0 {
		return Package{}, apperr.ErrInternal.WithCause(fmt.Errorf("内置仿真包 %s@%s 协议校验失败: %s", code, version, strings.Join(findings, ",")))
	}

	canonical, err := jsonx.AnyBytes(doc, apperr.ErrInternal)
	if err != nil {
		return Package{}, err
	}
	return Package{
		Code:              code,
		Version:           version,
		Name:              strings.TrimSpace(manifest.Meta.Name),
		Category:          strings.TrimSpace(manifest.Meta.Category),
		Compute:           ComputeBrowser,
		ScaleLimit:        manifest.Meta.ScaleLimit,
		BundleKey:         builtinBundleRefPrefix + code + "@" + version,
		BundleHash:        pkgcrypto.SHA256Hex(canonical),
		InteractionSchema: manifest.InteractionSchema,
		CodeTrace:         manifest.CodeTrace,
		AuthorType:        AuthorPlatformBuiltIn,
		Status:            PackageStatusPublished,
	}, nil
}
