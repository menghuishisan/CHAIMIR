// useUrlState 把列表页的筛选、搜索、标签页与地址参数双向同步(规范 §6.2,全站只用这一种写法)。
// 刷新、返回、分享链接都保留当前视图;写入用替换历史记录,不给后退键增加一串筛选步骤。
// 筛选变化时同时清掉页码(筛选后停在第 5 页往往越界);页码本身由 usePagedResource 的 urlPageKey 读写。

import { useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router'
import { readQuery, writeQuery, type QuerySchema, type QueryValues } from '../utils/urlQuery'

export interface UrlStateOptions {
  /** 筛选变化时要一并清掉的页码参数名,默认 page;同页有两个分页列表时各自指定 */
  pageKey?: string
}

/**
 * useUrlState 返回当前地址里的参数值(不认识的值已回到默认值)和一个更新函数。
 * schema 必须是模块级常量(不要在组件里临时创建),否则每次渲染都会得到新的结果对象。
 */
export function useUrlState<S extends QuerySchema>(
  schema: S,
  options: UrlStateOptions = {},
): readonly [QueryValues<S>, (patch: Partial<QueryValues<S>>) => void] {
  const [searchParams, setSearchParams] = useSearchParams()
  const pageKey = options.pageKey ?? 'page'
  const search = searchParams.toString()

  // 按查询串内容记忆:地址没变时返回同一个对象,页面拿它做读取依赖不会重复请求
  const values = useMemo(() => readQuery(new URLSearchParams(search), schema), [search, schema])

  const update = useCallback(
    (patch: Partial<QueryValues<S>>) => {
      setSearchParams((previous) => writeQuery(previous, schema, patch, [pageKey]), { replace: true })
    },
    [pageKey, schema, setSearchParams],
  )

  return [values, update] as const
}
