/**
 * 触感反馈工具：安全封装 navigator.vibrate，作为渐进增强能力使用。
 */

export function triggerHaptic(pattern: number | number[] = 10): void {
  if (
    typeof window !== 'undefined'
    && 'vibrate' in navigator
    && !window.matchMedia('(prefers-reduced-motion: reduce)').matches
  ) {
    navigator.vibrate(pattern)
  }
}
