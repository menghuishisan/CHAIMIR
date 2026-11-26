/** loadRuntimeConfiguration 从同源唯一入口读取配置,超时与失败均显式拒绝应用启动。 */
export function loadRuntimeConfiguration(): Promise<void> {
  const timeoutMs = Number(import.meta.env.VITE_API_TIMEOUT_MS)
  if (!Number.isInteger(timeoutMs) || timeoutMs <= 0) {
    return Promise.reject(new Error('启动配置读取超时必须为正整数'))
  }

  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = '/runtime-config.js'

    const finish = (error?: Error) => {
      window.clearTimeout(timer)
      script.onload = null
      script.onerror = null
      script.remove()
      if (error) reject(error)
      else resolve()
    }

    const timer = window.setTimeout(
      () => finish(new Error('部署配置读取超时')),
      timeoutMs,
    )
    script.onload = () => finish()
    script.onerror = () => finish(new Error('部署配置读取失败'))
    document.head.append(script)
  })
}
