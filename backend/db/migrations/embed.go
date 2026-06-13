// migrations 嵌入版本化 SQL 迁移文件,供 cmd/migrate 在 distroless 镜像中执行。
package migrations

import "embed"

// FS 包含全部 up/down SQL 迁移文件。
//
//go:embed *.sql
var FS embed.FS
