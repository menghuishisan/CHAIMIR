// sms-gateway.mjs 是 local-dev E2E 使用的临时短信代理网关实现。
// 鉴权 token 只从运行时注入的 SMS_HTTP_TOKEN 读取，不接受默认值或配置文件路径。

import fs from 'node:fs'
import http from 'node:http'
import path from 'node:path'

const token = (process.env.SMS_HTTP_TOKEN || '').trim()
if (!token) {
  throw new Error('SMS_HTTP_TOKEN 未注入，拒绝启动短信验收网关')
}
const port = Number(process.env.SMS_GATEWAY_PORT || 18080)
if (!Number.isInteger(port) || port < 1 || port > 65535) {
  throw new Error('SMS_GATEWAY_PORT 必须是有效端口')
}

// 收到的验证码落盘成 ndjson，供用例读取验证码完成短信登录与找回密码闭环。
const logPath = process.env.SMS_GATEWAY_LOG_PATH
if (logPath) {
  fs.mkdirSync(path.dirname(logPath), { recursive: true })
}

const server = http.createServer((req, res) => {
  if (req.method === 'GET' && req.url === '/-/readyz') {
    res.writeHead(200).end('ok')
    return
  }
  if (req.method !== 'POST' || req.url !== '/sms') {
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
      if (logPath) {
        fs.appendFileSync(logPath, record + '\n')
      }
      res.writeHead(204).end()
    } catch {
      res.writeHead(400).end()
    }
  })
})

server.listen(port, '0.0.0.0', () => {
  console.log(`Chaimir E2E SMS gateway listening on 0.0.0.0:${port}/sms`)
})
