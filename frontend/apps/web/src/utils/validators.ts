// validators 收拢跨页面复用的输入格式判断。
// 这些判断只在前端先挡明显非法值,给出就近提示;是否真的可用由后端最终判定
// (例如地址能否访问、对象是否存在),前端不做可达性探测。

/** parseHttpUrl 返回可直接序列化提交的HTTP(S)地址，凭据由独立配置承载。 */
export function parseHttpUrl(value: string): URL | undefined {
  try {
    const url = new URL(value.trim())
    if ((url.protocol !== 'http:' && url.protocol !== 'https:') || url.username || url.password) {
      return undefined
    }
    return url
  } catch {
    return undefined
  }
}
