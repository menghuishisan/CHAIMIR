/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** 后端统一 API 根地址 */
  readonly VITE_API_BASE_URL?: string
  /** 可选的独立 WebSocket 根地址;缺省时由 API 地址推导 */
  readonly VITE_WS_BASE_URL?: string
  /** 前端 API 请求超时(毫秒) */
  readonly VITE_API_TIMEOUT_MS?: string
  readonly VITE_RUNTIME_SELFTEST_POLL_INTERVAL_MS?: string
  readonly VITE_RUNTIME_SELFTEST_POLL_TIMEOUT_MS?: string
  readonly VITE_SIM_STEP_INTERVAL_MS?: string
  /** 仿真 Worker 单条指令超时(毫秒) */
  readonly VITE_SIM_WORKER_COMMAND_TIMEOUT_MS?: string
}

interface ChaimirRuntimeConfig {
  /** 部署形态:saas 保留平台层入口,school 为学校私有化 */
  deploymentMode?: string
  /** 独立 HTTPS 工具 origin,用于沙箱 iframe 边界 */
  sandboxToolOrigin?: string
}

interface Window {
  __CHAIMIR_RUNTIME_CONFIG__?: ChaimirRuntimeConfig
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
