// pageLocation.ts 定义顶栏「当前位置」的层级类型与上下文(规范 §6.2)。
// 类型单独成文件:layouts/main/pageTrail.ts 的纯函数要在 node:test 里运行,只能 import type 到这里。

import { createContext } from 'react'

/** 顶栏里的一层位置;label 为 undefined 表示对象名还在读取,顶栏显示占位 */
export interface PageLocationItem {
  label: string | undefined
  /** 上级的站内路径;当前层级不传 */
  to?: string
}

/** 日常页外框提供的写入口;不在日常页外框里(认证页、工作台)时为 null */
export const PageLocationContext = createContext<((items: PageLocationItem[] | null) => void) | null>(null)

/** 详情页读取状态里与顶栏名称有关的字段(AsyncResourceState 的子集) */
interface LabelSource<T> {
  status: 'loading' | 'success' | 'empty' | 'error'
  data: T | null
}

/**
 * locationLabel 给出详情页在顶栏里的对象名:
 * 读取中返回 undefined(顶栏显示占位,不沿用上一个对象的名字);读到后是对象名;
 * 读取失败、对象不存在或名字为空时用通用名称(如「课程详情」),顶栏不会一直停在占位。
 */
export function locationLabel<T>(resource: LabelSource<T>, pick: (data: T) => string, fallback: string): string | undefined {
  if (resource.status === 'loading') return undefined
  if (resource.status !== 'success' || resource.data === null) return fallback
  const name = pick(resource.data).trim()
  return name === '' ? fallback : name
}
