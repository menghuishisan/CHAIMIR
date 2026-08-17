/**
 * Popover:轻量浮层面板(@radix-ui/react-popover 封装)。
 * 承载筛选器、快捷设置等小块交互内容;必读信息与复杂流程用 Modal/Drawer。
 * 组合导出:Popover(Root)/ PopoverTrigger / PopoverContent / PopoverClose。
 * 动效:pop-in / pop-out,transform-origin 对齐触发点(Radix 变量)。
 * transform 所有权(§4.3):定位 transform 由 Radix popper wrapper 独占,
 * 动画 transform 只写在 Content 本体。
 */
import * as PopoverPrimitive from '@radix-ui/react-popover'
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import { cn } from '../../lib/cn'

export const Popover = PopoverPrimitive.Root
export const PopoverTrigger = PopoverPrimitive.Trigger
export const PopoverClose = PopoverPrimitive.Close

export type PopoverContentProps = ComponentPropsWithoutRef<typeof PopoverPrimitive.Content>

export const PopoverContent = forwardRef<
  ElementRef<typeof PopoverPrimitive.Content>,
  PopoverContentProps
>(({ className, style, align = 'center', sideOffset = 6, ...props }, ref) => (
  <PopoverPrimitive.Portal>
    <PopoverPrimitive.Content
      ref={ref}
      align={align}
      sideOffset={sideOffset}
      className={cn(
        'z-dropdown rounded-lg border border-line bg-surface p-4 text-ink shadow-md',
        'animate-pop-in data-[state=closed]:animate-pop-out',
        className
      )}
      /* 动态定位值:缩放原点跟随触发源(Radix 计算),属定位计算允许内联 */
      style={{ transformOrigin: 'var(--radix-popover-content-transform-origin)', ...style }}
      {...props}
    />
  </PopoverPrimitive.Portal>
))
PopoverContent.displayName = 'PopoverContent'
