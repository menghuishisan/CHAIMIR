// contracts tenant_modules 定义租户功能开关的封闭集合与稳定来源模块映射。
package contracts

import "strings"

const (
	// TenantModuleTeaching 控制课程教学入口。
	TenantModuleTeaching = "teaching"
	// TenantModuleExperiment 控制实验编排与工作台。
	TenantModuleExperiment = "experiment"
	// TenantModuleContest 控制竞赛与漏洞题工坊。
	TenantModuleContest = "contest"
	// TenantModuleSim 控制仿真场景与回放。
	TenantModuleSim = "sim"
	// TenantModuleGrade 控制总评、成绩中心和预警。
	TenantModuleGrade = "grade"
)

// TenantModules 返回协议顺序的完整集合，每次独立分配，调用方不能改写全局登记。
func TenantModules() []string {
	return []string{TenantModuleTeaching, TenantModuleExperiment, TenantModuleContest, TenantModuleSim, TenantModuleGrade}
}

// TenantModuleFromSourceRef 读取已校验稳定来源的模块段，不反查上层业务数据。
func TenantModuleFromSourceRef(sourceRef string) string {
	module, _, ok := strings.Cut(sourceRef, ":")
	if !ok {
		return ""
	}
	for _, known := range TenantModules() {
		if module == known {
			return module
		}
	}
	return ""
}
