// Tokens 令牌系统导出
// 导入全局样式以便应用加载
import './global.css'

// 导出 TypeScript 常量供 JS 使用
export const colors = {
  // 主色
  primary: 'var(--color-primary)',
  primaryHover: 'var(--color-primary-hover)',
  primaryFg: 'var(--color-primary-fg)',
  primarySoft: 'var(--color-primary-soft)',
  primaryText: 'var(--color-primary-text)',
  accent: 'var(--color-accent)',

  // 副色
  secondary: 'var(--color-secondary)',
  secondaryHover: 'var(--color-secondary-hover)',
  secondaryFg: 'var(--color-secondary-fg)',
  secondarySoft: 'var(--color-secondary-soft)',
  secondaryText: 'var(--color-secondary-text)',

  // 文字
  text: 'var(--color-text)',
  textStrong: 'var(--color-text-strong)',
  textSub: 'var(--color-text-sub)',
  textFaint: 'var(--color-text-faint)',

  // 背景
  bg: 'var(--color-bg)',
  surface: 'var(--color-surface)',
  surfaceSunken: 'var(--color-surface-sunken)',
  surfaceHover: 'var(--color-surface-hover)',

  // 边框
  border: 'var(--color-border)',
  borderStrong: 'var(--color-border-strong)',
  borderSubtle: 'var(--color-border-subtle)',

  // 语义色
  success: 'var(--color-success)',
  successBg: 'var(--color-success-bg)',
  warning: 'var(--color-warning)',
  warningBg: 'var(--color-warning-bg)',
  danger: 'var(--color-danger)',
  dangerBg: 'var(--color-danger-bg)',
  info: 'var(--color-info)',
  infoBg: 'var(--color-info-bg)',

  // 深色面板
  darkBg: 'var(--color-dark-bg)',
  darkSurface: 'var(--color-dark-surface)',
  darkText: 'var(--color-dark-text)',
  darkTextSub: 'var(--color-dark-text-sub)',
  onDarkAccent: 'var(--color-on-dark-accent)',
  onDarkAccentHover: 'var(--color-on-dark-accent-hover)',
  onDarkAccentFg: 'var(--color-on-dark-accent-fg)',
  mutedBg: 'var(--color-muted-bg)',
  mutedText: 'var(--color-muted-text)',
  mutedAccent: 'var(--color-muted-accent)',
} as const

export const spacing = {
  0: 'var(--space-0)',
  1: 'var(--space-1)',
  2: 'var(--space-2)',
  3: 'var(--space-3)',
  4: 'var(--space-4)',
  5: 'var(--space-5)',
  6: 'var(--space-6)',
  8: 'var(--space-8)',
  10: 'var(--space-10)',
  12: 'var(--space-12)',
  16: 'var(--space-16)',
} as const

export const radius = {
  xs: 'var(--radius-xs)',
  sm: 'var(--radius-sm)',
  base: 'var(--radius)',
  lg: 'var(--radius-lg)',
  xl: 'var(--radius-xl)',
  full: 'var(--radius-full)',
} as const

export const shadow = {
  xs: 'var(--shadow-xs)',
  base: 'var(--shadow)',
  md: 'var(--shadow-md)',
  lg: 'var(--shadow-lg)',
} as const

export const breakpoints = {
  sm: 640,
  md: 768,
  lg: 1024,
  xl: 1280,
  '2xl': 1536,
} as const

export const zIndex = {
  below: -1,
  ground: 0,
  base: 1,
  sticky: 50,
  dropdown: 60,
  drawer: 80,
  immersive: 100,
  modal: 120,
  toast: 200,
} as const

export const layout = {
  topnavHeight: 'var(--topnav-h)',
  sidebarWidth: 'var(--sidebar-w)',
  sidebarWidthCollapsed: 'var(--sidebar-w-collapsed)',
  contentMax: 'var(--content-max)',
} as const

export const transition = {
  fast: 'var(--t-fast)',
  base: 'var(--t)',
  slow: 'var(--t-slow)',
} as const

export const easing = {
  base: 'var(--ease)',
  out: 'var(--ease-out)',
  in: 'var(--ease-in)',
} as const
