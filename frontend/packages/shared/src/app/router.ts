// 哈希路由：四端共用的轻量路由解析，支持深链和沉浸式工作台参数。

export interface ParsedRoute {
  path: string
  params: URLSearchParams
}

/**
 * parseHashRoute 将 location.hash 解析为路径和查询参数。
 */
export function parseHashRoute(hash: string): ParsedRoute {
  const raw = hash.replace(/^#\/?/, '')
  const [pathPart, queryPart] = raw.split('?')
  return {
    path: pathPart || '',
    params: new URLSearchParams(queryPart ?? ''),
  }
}

/**
 * routeHref 生成四端内部哈希链接。
 */
export function routeHref(path: string, params?: Record<string, string | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params ?? {})) {
    if (value) {
      search.set(key, value)
    }
  }
  const suffix = search.toString()
  return suffix ? `#/${path}?${suffix}` : `#/${path}`
}
