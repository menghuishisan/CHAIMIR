// AuthBrand 负责认证壳与认证页面共用的平台品牌和学校部署身份展示。
// 这是应用级跨层拼装:只组合设计系统品牌原语,不承载 identity 流程或数据获取。

import { BrandLockup, TenantCrest } from '@chaimir/ui'

interface AuthBrandMarkProps {
  /** large 桌面品牌区的放大字号;窄屏与特权通道用常规字号 */
  large?: boolean
  /** subtitle 锁定组合下的副标题:默认平台全称,特权通道在此标明这是平台管理通道 */
  subtitle?: string
}

/**
 * AuthBrandMark 渲染认证区的平台锁定组合与副标题。
 * 品牌章留给成功回执,避免入口同时展示同源的两种品牌形态。
 */
export function AuthBrandMark({ large, subtitle = '区块链教学实验竞赛平台' }: AuthBrandMarkProps) {
  return (
    <div className="animate-develop">
      <BrandLockup
        variant={large ? 'wide' : 'narrow'}
        markClassName="text-accent"
        className="text-on-dark"
      />
      <span className="mt-1.5 block font-mono text-xs text-on-dark-sub">{subtitle}</span>
    </div>
  )
}

interface SchoolBrandLineProps {
  /** 学校显示名 */
  name: string
  /** 校徽的内联地址;为空时徽记位回落学校名首字 */
  logoSrc: string
}

/**
 * SchoolBrandLine 渲染当前私有部署的学校徽记与校名。
 * 学校身份使用较小徽记,保持平台锁定组合的主视觉层级。
 */
export function SchoolBrandLine({ name, logoSrc }: SchoolBrandLineProps) {
  return (
    <div className="flex items-center gap-2.5">
      <TenantCrest name={name} logoSrc={logoSrc} size="sm" onDark />
      <span className="truncate text-sm text-on-dark">{name}</span>
    </div>
  )
}
