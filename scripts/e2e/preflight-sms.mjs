// preflight-sms.mjs 是联调前的短信链路探活：跑用例之前先打通
// 后端 -> 本地短信 mock 的完整调用，确认两个进程读到的 SMS_HTTP_TOKEN 一致。
// 任何失败都直接以非零退出码结束，避免带着坏环境跑完整套用例后才发现问题。

import { loadE2EEnv, requireEnv } from './env.mjs'

const env = loadE2EEnv()
const backendBaseURL = process.env.E2E_BACKEND_BASE_URL || 'http://127.0.0.1:8080'
const smsEndpoint = process.env.E2E_SMS_ENDPOINT || 'http://127.0.0.1:18888/send'

// 探活使用验收种子中的固定学生账号（backend/cmd/migrate/acceptance_seed.go），
// 其手机号在验收租户内唯一，因此无需显式传 tenant_id。
const PROBE_PHONE = process.env.E2E_SMS_PROBE_PHONE || '13900002003'
const SCENE_LOGIN = 1

/**
 * fail 统一输出可直接行动的失败原因并终止启动流程。
 */
function fail(reason, detail) {
  console.error(`[preflight-sms] 失败: ${reason}`)
  if (detail) {
    console.error(`[preflight-sms] 详情: ${detail}`)
  }
  process.exit(1)
}

/**
 * probeGatewayToken 直接以配置中的 token 调用短信 mock，
 * 把「mock 侧 token 不一致」与「后端侧 token 不一致」两类故障区分开。
 */
async function probeGatewayToken(token) {
  let response
  try {
    response = await fetch(smsEndpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ phone: PROBE_PHONE, template: 'preflight', code: 'PREFLT' }),
    })
  } catch (error) {
    fail(`短信 mock 未就绪，无法连接 ${smsEndpoint}`, `请先启动 scripts/e2e/sms-gateway.mjs（${error.message}）`)
  }
  if (response.status === 401) {
    fail(
      '短信 mock 拒绝了配置中的 token',
      '运行中的 mock 进程加载的不是当前 deploy/config/secret.env，请重启 scripts/e2e/sms-gateway.mjs',
    )
  }
  if (response.status !== 204) {
    fail(`短信 mock 返回异常状态 ${response.status}`, `期望 204，端点 ${smsEndpoint}`)
  }
}

/**
 * probeBackendSendSMS 走真实业务接口发一次验证码，
 * 覆盖后端配置装载 + 出站校验 + mock 鉴权的完整链路。
 */
async function probeBackendSendSMS() {
  const url = `${backendBaseURL}/api/v1/auth/sms/send`
  let response
  try {
    response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ phone: PROBE_PHONE, scene: SCENE_LOGIN }),
    })
  } catch (error) {
    fail(`后端未就绪，无法连接 ${url}`, error.message)
  }
  // 后端统一信封固定返回 HTTP 200，业务结果看 body.code。
  if (response.status !== 200) {
    fail(`短信发送接口返回 HTTP ${response.status}`, `期望 200，端点 ${url}`)
  }
  let body
  try {
    body = await response.json()
  } catch (error) {
    fail('短信发送接口响应不是合法 JSON', error.message)
  }
  if (body.code !== '0') {
    const hint =
      body.code === '11000'
        ? '多为后端与短信 mock 的 SMS_HTTP_TOKEN 不一致，请确认两个进程都加载了 deploy/config/secret.env 后重启'
        : `业务错误码 ${body.code}`
    fail(`短信发送接口未成功: ${body.message}`, `${hint}；trace_id=${body.trace_id}`)
  }
}

const token = requireEnv(env, 'SMS_HTTP_TOKEN')
await probeGatewayToken(token)
await probeBackendSendSMS()
console.log('[preflight-sms] 通过: 后端与短信 mock 的 SMS_HTTP_TOKEN 一致，验证码链路可用')
