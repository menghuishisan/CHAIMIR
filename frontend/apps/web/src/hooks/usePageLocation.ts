// usePageLocation 让页面声明它在顶栏里的位置(规范 §6.2):只声明侧栏入口之后的层级。
// 详情页在对象读到之前传 label: undefined,顶栏显示占位;读到后传对象名。
// 卸载时清空声明,切到另一个对象时不会残留上一个对象的名字。

import { useContext, useLayoutEffect } from 'react'
import { PageLocationContext, type PageLocationItem } from './pageLocation'

/**
 * usePageLocation 把页面的位置层级写进顶栏。
 * 依赖按内容比较(序列化),页面每次渲染传新数组不会触发重复写入;
 * 用 layout effect 写入,顶栏与页面同一帧更新,不会先闪一下旧位置。
 * 不在日常页外框里(上下文为空)时什么都不做。
 */
export function usePageLocation(items: PageLocationItem[]): void {
  const setLocation = useContext(PageLocationContext)
  const key = JSON.stringify(items)
  useLayoutEffect(() => {
    if (!setLocation) return
    setLocation(JSON.parse(key) as PageLocationItem[])
    return () => setLocation(null)
  }, [key, setLocation])
}
