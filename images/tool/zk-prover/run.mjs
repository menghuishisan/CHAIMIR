// 将命令工具的结构化输入交给官方 CLI；隔离与资源上限由 M2 统一执行。
import { fileURLToPath } from 'node:url'

const entries = {
  circom2: './node_modules/circom2/cli.js',
  snarkjs: './node_modules/snarkjs/build/cli.cjs',
}

try {
  process.stdin.setEncoding('utf8')
  let input = ''
  for await (const chunk of process.stdin) input += chunk
  const request = JSON.parse(input)
  if (
    request === null || typeof request !== 'object' || Array.isArray(request) ||
    Object.keys(request).some((key) => key !== 'tool' && key !== 'args') ||
    typeof request.tool !== 'string' || !Object.hasOwn(entries, request.tool) || !Array.isArray(request.args) ||
    !request.args.every((arg) => typeof arg === 'string' && !arg.includes('\0'))
  ) throw new Error('Provide a supported tool and its argument array.')

  const entry = new URL(entries[request.tool], import.meta.url)
  process.argv = [process.execPath, fileURLToPath(entry), ...request.args]
  await import(entry.href)
} catch (error) {
  console.error(error.message)
  process.exitCode = 64
}
