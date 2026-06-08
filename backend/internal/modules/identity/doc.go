// Package identity 实现 M1 身份与租户(第0层 地基)。
// 职责:租户/账号/认证/授权/审计表;所有模块的前置依赖。
// 边界(CLAUDE.md §3):只读写自己的表;对外只经 contracts.IdentityService;
//
//	提供全平台唯一 audit_log 的写入实现(platform/audit.Writer)。
//
// 内部结构:api.go / service*.go / repo.go / model.go / dto.go / enum.go / internal/(私有,sqlc 生成)。
package identity
