// validators 收拢跨页面复用的输入格式判断。
// 这些判断只在前端先挡明显非法值,给出就近提示;是否真的可用由后端最终判定
// (例如地址能否访问、对象是否存在),前端不做可达性探测。

/**
 * isHttpUrl 判断是否是带协议头的绝对地址。
 * 只看协议头而不用 URL 构造器:构造器会把 "http://" 这类残缺值也判为合法,
 * 反而放过用户明显没填完的情况;而这里的目的是提示用户补全,不是安全校验。
 */
export function isHttpUrl(value: string): boolean {
  const trimmed = value.trim()
  return trimmed.startsWith('http://') || trimmed.startsWith('https://')
}
