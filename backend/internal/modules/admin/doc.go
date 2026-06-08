// Package admin 实现 M9 管理后台(第3层 聚合)。
// 职责:运营看板/配置/审计查询(只读聚合)。
// 边界(CLAUDE.md §3.4):只读不跨写,只调其他模块只读接口。
package admin
