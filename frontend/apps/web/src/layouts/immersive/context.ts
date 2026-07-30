// immersive/context 向沉浸态工作台下传壳层能力(标题与退出),
// 使各工作台不必各自推断退出目标 —— 退出目标是壳层职责(见 immersiveRoutes)。

import { createContext, useContext } from 'react'

export interface ImmersiveValue {
  /** 当前工作台标题(壳层统一提供) */
  title: string
  /** 退出沉浸态,回到登记的日常页 */
  exit: () => void
}

export const ImmersiveContext = createContext<ImmersiveValue | null>(null)

/** useImmersive 读取沉浸壳能力;在沉浸路由之外调用属装配错误,直接暴露而非静默降级。 */
export function useImmersive(): ImmersiveValue {
  const value = useContext(ImmersiveContext)
  if (!value) {
    throw new Error('useImmersive 只能在沉浸态路由内使用')
  }
  return value
}
