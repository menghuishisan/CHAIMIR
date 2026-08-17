/**
 * TenantCrest:租户徽记 A5。有校徽显示校徽,没有就显示学校名首字 —— 不做默认徽记图形。
 * 理由:图形版所有学校长得一样,徽标位退化成纯装饰;首字块每个租户都不同且都正确,
 * 而且不存在图片加载失败,不需要第二层兜底。
 * 学校名文本由调用方渲染在旁边,身份不依赖本组件传达(规范 §3 颜色/图形不作唯一信息载体)。
 */
import { useState } from 'react'
import { cn } from '../lib/cn'
import { BrandMark } from './BrandMark'

/** 两档容器尺寸:顶栏/侧栏租户区与租户设置页 */
const CREST_SIZE = {
  sm: { box: 'size-8 rounded-md', text: 'text-md' },
  lg: { box: 'size-24 rounded-pane', text: 'text-4xl' },
} as const

export type TenantCrestSize = keyof typeof CREST_SIZE

/**
 * initialOf 取显示名的首个字符。
 * 用 Array.from 而不是 name[0]:后者会把 4 字节字符(部分生僻字、emoji)切成半个码元。
 */
function initialOf(name: string): string | undefined {
  const trimmed = name.trim()
  if (trimmed === '') return undefined
  return Array.from(trimmed)[0]
}

export interface TenantCrestProps {
  /** 租户显示名:传 display_name || name,用于取首字与读屏 */
  name: string
  /**
   * 已解析好的校徽地址:租户配置与学校品牌接口返回的 logo_image(data URI)。
   * 本组件不发请求 —— UI 包不认识后端接口,解析交给页面层,组件只负责显示与回落。
   */
  logoSrc?: string
  /** 尺寸档:sm 顶栏/侧栏 40 槽位内的 32 块 / lg 租户设置页 96 */
  size?: TenantCrestSize
  /** 深底语境(侧栏、顶栏、认证页);默认浅底 */
  onDark?: boolean
  className?: string
}

export function TenantCrest({
  name,
  logoSrc,
  size = 'sm',
  onDark = false,
  className,
}: TenantCrestProps) {
  const spec = CREST_SIZE[size]
  // 只记录失败的具体地址;换了校徽后新地址自然会重新尝试加载。
  const [failedLogoSrc, setFailedLogoSrc] = useState<string>()

  const shell = cn('flex shrink-0 items-center justify-center overflow-hidden', spec.box, className)

  // 真实校徽:contain 保证不裁切校徽内容,透明底不加白框(规范:不替学校造背景)
  if (logoSrc && logoSrc.trim() !== '' && failedLogoSrc !== logoSrc) {
    return (
      <img
        src={logoSrc}
        alt={`${name}校徽`}
        loading="lazy"
        decoding="async"
        onError={() => setFailedLogoSrc(logoSrc)}
        className={cn(shell, 'object-contain')}
      />
    )
  }

  const initial = initialOf(name)

  return (
    <span
      aria-hidden="true"
      className={cn(
        shell,
        'font-display leading-none',
        onDark
          ? 'border border-on-dark-accent-line bg-on-dark-accent-soft text-accent'
          : 'border border-primary-soft bg-primary-soft text-primary',
        spec.text
      )}
    >
      {/* 显示名为空(理论上不该出现,但契约里 display_name 可选)时退到主标志,不显示空框 */}
      {initial ?? <BrandMark size={size === 'lg' ? 'xl' : 'sm'} />}
    </span>
  )
}
