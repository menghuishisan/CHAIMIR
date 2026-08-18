// 学校品牌读取(登录页用)。
//
// 只有学校私有部署才有学校品牌:平台托管的登录页面对的学校还没确定,显示任何一所学校的
// 校徽都是错的,而带学校标识的公开接口又会变成廉价的学校名单探测通道。
// 所以这个读取口不接受任何参数,并且只在部署形态是学校私有部署时才调用。

import { useEffect, useState } from 'react'
import type { TenantBrand } from '@chaimir/api-client'
import { api } from '../../app/api'
import { appConfig } from '../../app/config'
import { errorDiagnostics } from '../../utils/userFacingError'

/**
 * useTenantBrand 读取当前部署的学校品牌;平台托管或读取失败时返回 undefined。
 * 返回 undefined 表示「不显示学校身份」,登录页照常只显示平台锁定组合。
 */
export function useTenantBrand(): TenantBrand | undefined {
  const [brand, setBrand] = useState<TenantBrand>()

  useEffect(() => {
    if (appConfig.deploymentMode !== 'school') return
    let active = true
    void (async () => {
      try {
        const result = await api.identity.getTenantBrand()
        if (active && result.display_name.trim() !== '') setBrand(result)
      } catch (error) {
        // 学校品牌只是登录页的身份标注,取不到不该挡住登录:原因只进控制台供排障。
        console.error('学校品牌读取失败', errorDiagnostics(error))
      }
    })()
    return () => {
      active = false
    }
  }, [])

  return brand
}
