// sms-gateway.mjs 模拟本地短信代理网关，供浏览器验收短信登录与找回密码闭环。
// 鉴权 token 与后端进程同源：均取自 env.mjs 统一加载的 deploy/config/secret.env，
// 因此这里不接受任何形式的默认值或宽松校验。

import fs from 'node:fs'
import http from 'node:http'
import path from 'node:path'
import { loadE2EEnv, requireEnv } from './env.mjs'

const env = loadE2EEnv()
const token = requireEnv(env, 'SMS_HTTP_TOKEN')
const port = Number(process.env.E2E_SMS_PORT || 18888)

// 收到的验证码落盘成 ndjson，供用例读取验证码完成短信登录与找回密码闭环。
const logPath = process.env.E2E_SMS_LOG_PATH
if (logPath) {
  fs.mkdirSync(path.dirname(logPath), { recursive: true })
}

const server = http.createServer((req, res) => {
  if (req.method !== 'POST' || req.url !== '/send') {
    res.writeHead(404).end()
    return
  }
  if (req.headers.authorization !== `Bearer ${token}`) {
    // token 不匹配说明本进程与后端读到了不同配置，打印出来便于立刻定位编排问题。
    console.error('拒绝短信请求: Authorization 与本地网关 token 不一致，请确认后端与网关加载的是同一份 deploy/config/secret.env')
    res.writeHead(401).end()
    return
  }
  const chunks = []
  req.on('data', (chunk) => chunks.push(chunk))
  req.on('end', () => {
    try {
      const payload = JSON.parse(Buffer.concat(chunks).toString('utf8'))
      if (!payload.phone || !payload.template || !payload.code) {
        res.writeHead(400).end()
        return
      }
      const record = JSON.stringify({ received_at: new Date().toISOString(), ...payload })
      console.log(record)
      if (logPath) {
        fs.appendFileSync(logPath, record + '\n')
      }
      res.writeHead(204).end()
    } catch {
      res.writeHead(400).end()
    }
  })
})

server.listen(port, '127.0.0.1', () => {
  console.log(`Chaimir E2E SMS gateway listening on http://127.0.0.1:${port}/send`)
})
