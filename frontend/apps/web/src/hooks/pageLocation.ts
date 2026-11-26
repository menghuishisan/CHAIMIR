import { createContext } from 'react'

/** 顶栏里的一层位置;label 为 undefined 表示对象名还在读取,顶栏显示占位 */
export interface PageLocationItem {
  label: string | undefined
  /** 上级的站内路径;当前层级不传 */
  to?: string
}

/** 日常页外框提供的写入口;不在日常页外框里(认证页、工作台)时为 null */
export const PageLocationContext = createContext<((items: PageLocationItem[] | null) => void) | null>(null)

interface LabelSource<T> {
  status: 'loading' | 'success' | 'empty' | 'error'
  data: T | null
}

/**
 * locationLabel 读取中返回占位，成功后返回对象名。
 * 失败、对象不存在或空名称使用通用名称，避免沿用上一对象或一直占位。
 */
export function locationLabel<T>(resource: LabelSource<T>, pick: (data: T) => string, fallback: string): string | undefined {
  if (resource.status === 'loading') return undefined
  if (resource.status !== 'success' || resource.data === null) return fallback
  const name = pick(resource.data).trim()
  return name === '' ? fallback : name
}
