// Package contracts 定义模块间交互的接口契约 + DTO + 事件类型。
// 依据 docs/总-工程目录设计.md §3.1/§3.2:
//
//	· 模块只经 contracts 接口互调,禁止 import 其他模块 package。
//	· 接口按层组织,低层 contracts 不引用高层 DTO,保证单向。
//	· main.go 装配时依赖倒置注入各接口实现。
//
// 分层文件:foundation.go(地基)/ engine.go / business.go / aggregation.go / events.go。
// 新增跨模块能力时先在对应层级文件声明契约,再由 cmd/server 注入实现。
package contracts
