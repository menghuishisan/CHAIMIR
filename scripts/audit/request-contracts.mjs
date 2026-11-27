// 用后端已有 YAML 依赖核对 HTTP 边界，无需安装另一份解析器。
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

try {
  process.stdout.write(execFileSync('go', ['run', '../scripts/audit/request-contracts.go'], {
    cwd: fileURLToPath(new URL('../../backend', import.meta.url)),
    encoding: 'utf8',
    maxBuffer: 8 * 1024 * 1024,
  }))
} catch (error) {
  process.stderr.write(error.stderr ?? error.message)
  process.stdout.write(error.stdout ?? '')
  process.exitCode = 1
}
