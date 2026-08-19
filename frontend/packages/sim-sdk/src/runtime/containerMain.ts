// 本文件是隔离容器内 runner 进程的入口,把 stdio-json 协议接到容器执行宿主上。
//
// 协议(与 SIM_BACKEND_STDIO_ADAPTERS_JSON 的 stdio-json 一致,后端 StdioJSONAdapter 消费):
//   一次 exec = 从标准输入读一行 JSON 命令 → 向标准输出写一行 JSON 响应 → 进程退出。
//   后端为每个事件发起一次 exec,容器不常驻会话状态。
//
// 只做协议接线:能力封锁、归档校验、装配与状态推进都在 containerHost 与 SimEngine 内,
// 这里不承载任何业务判断,以免容器侧与浏览器侧对同一件事出现两套口径。

import { runCommand, type RunnerCommand } from './containerHost'

/** maxInputBytes 与后端能力目录保持同一配置来源,避免容器侧出现第二套阈值。 */
const maxInputBytes = readConfiguredInputLimit()

/**
 * writeResponse 在装配仿真包之前就绑定好标准输出。
 *
 * 必须在模块加载期取到:执行宿主会在装配前封锁 `process` 等宿主入口(见 containerGuards),
 * 封锁之后再取 `process.stdout` 就拿不到了。先绑定、后封锁,响应通道才不会被自己的防护切断。
 */
const writeResponse = process.stdout.write.bind(process.stdout)

/** readInput 同样在封锁前绑定标准输入。 */
const stdin = process.stdin

/**
 * main 读取单条命令、执行并输出单行 JSON 响应。
 * 任何异常都转成 `{ok:false,error}` 而非非零退出加空输出 —— 后端需要能定位到具体原因,
 * 只给退出码会让"包有缺陷"和"runner 自身故障"无法区分。
 */
async function main(): Promise<void> {
  try {
    const raw = await readStdin()
    const command = JSON.parse(raw) as RunnerCommand
    const response = await runCommand(command)
    writeResponse(`${JSON.stringify(response)}\n`)
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    writeResponse(`${JSON.stringify({ ok: false, error: message })}\n`)
  }
}

/**
 * readStdin 收齐标准输入正文并执行体积兜底。
 */
function readStdin(): Promise<string> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = []
    let size = 0
    let settled = false
    const cleanup = () => {
      stdin.off('data', onData)
      stdin.off('end', onEnd)
      stdin.off('error', onError)
    }
    const fail = (error: Error) => {
      if (settled) return
      settled = true
      cleanup()
      stdin.pause()
      reject(error)
    }
    const onData = (chunk: Buffer) => {
      if (settled) return
      size += chunk.length
      if (size > maxInputBytes) {
        fail(new Error('仿真运行输入超过容器上限'))
        return
      }
      chunks.push(chunk)
    }
    const onEnd = () => {
      if (settled) return
      settled = true
      cleanup()
      resolve(Buffer.concat(chunks).toString('utf8'))
    }
    const onError = (error: Error) => fail(error)
    stdin.on('data', onData)
    stdin.on('end', onEnd)
    stdin.on('error', onError)
  })
}

/** readConfiguredInputLimit 读取受控部署注入的 stdio 输入上限。 */
function readConfiguredInputLimit(): number {
  const raw = process.env.SIM_RUNNER_INPUT_MAX_BYTES?.trim() ?? ''
  const value = Number(raw)
  if (!Number.isSafeInteger(value) || value <= 0) {
    throw new Error('仿真运行器缺少有效的输入体积配置')
  }
  return value
}

void main()
