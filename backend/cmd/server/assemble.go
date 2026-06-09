// 模块装配编排:按分层顺序(地基→引擎→业务→聚合)调用各模块装配函数。
// 依据 docs/总-工程目录设计.md §2.0:每模块一装配文件,main.go 仅按依赖层次顺序调用。
package main

import (
	"context"
	"fmt"

	"chaimir/internal/platform/config"
)

// moduleDeps 是各模块装配时可取用的共享依赖。
type moduleDeps struct {
	ctx   context.Context
	cfg   *config.Config
	infra *infra
}

// assembleModules 按分层顺序装配 11 个模块。
// 反向通信(低层通知高层)走 eventbus 事件,不在此形成反向调用。
func assembleModules(d *moduleDeps) error {
	if d.ctx == nil {
		return fmt.Errorf("模块装配失败: 缺少进程 context,后台任务无法受控停止")
	}
	// 第0层 地基。
	if err := assembleIdentity(d); err != nil {
		return err
	}
	// 第1层 引擎(仅依赖地基;sim/sandbox/content 互不依赖,judge 可依赖 sandbox/content)。
	if err := assembleSandbox(d); err != nil {
		return err
	}
	if err := assembleContent(d); err != nil {
		return err
	}
	if err := assembleSim(d); err != nil {
		return err
	}
	if err := assembleJudge(d); err != nil {
		return err
	}
	// 第2层 业务(依赖地基/引擎,业务间不互依赖)。
	if err := assembleTeaching(d); err != nil {
		return err
	}
	if err := assembleExperiment(d); err != nil {
		return err
	}
	if err := assembleContest(d); err != nil {
		return err
	}
	// 第3层 聚合/横切(依赖下层只读接口;notify 提供通知能力)。
	if err := assembleNotify(d); err != nil {
		return err
	}
	if err := assembleAdmin(d); err != nil {
		return err
	}
	if err := assembleGrade(d); err != nil {
		return err
	}
	return nil
}
