/**
 * Tabs:选项卡(§5.1,Radix Tabs 底座)。
 * TabsList 下边线上有一条滑动指示条:测量激活 Trigger 的 offsetLeft/offsetWidth,
 * 以 transform(translateX + scaleX,1px 基准宽放大)平滑滑动 —— 只动 transform,
 * 符合 §4「只动 transform/opacity」(内联 style 仅承载动态定位计算,属规范允许的例外)。
 * 测量在激活值变化、列表尺寸变化(ResizeObserver)与 webfont 加载完成后均会补跑。
 * 键盘左右切换与焦点管理由 Radix 保证。
 */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ComponentPropsWithoutRef,
} from 'react'
import * as TabsPrimitive from '@radix-ui/react-tabs'
import type { LucideIcon } from 'lucide-react'
import { cn } from '../../lib/cn'
import { Icon } from '../../lib/icon'

/** 当前激活值经 Context 下发给 TabsList,供指示条在受控/非受控两种用法下都能重新测量 */
const TabsValueContext = createContext<string | undefined>(undefined)

export type TabsProps = ComponentPropsWithoutRef<typeof TabsPrimitive.Root>

export function Tabs({ value, defaultValue, onValueChange, children, ...rest }: TabsProps) {
  // 非受控时在本层镜像一份当前值;受控时以外部 value 为准
  const [innerValue, setInnerValue] = useState(defaultValue)
  const currentValue = value !== undefined ? value : innerValue

  const handleValueChange = (next: string) => {
    if (value === undefined) setInnerValue(next)
    onValueChange?.(next)
  }

  return (
    <TabsPrimitive.Root
      value={value}
      defaultValue={defaultValue}
      onValueChange={handleValueChange}
      {...rest}
    >
      <TabsValueContext.Provider value={currentValue}>{children}</TabsValueContext.Provider>
    </TabsPrimitive.Root>
  )
}

export type TabsListProps = ComponentPropsWithoutRef<typeof TabsPrimitive.List>

export function TabsList({ className, children, ...rest }: TabsListProps) {
  const activeValue = useContext(TabsValueContext)
  const listRef = useRef<HTMLDivElement>(null)
  const [indicator, setIndicator] = useState<{ left: number; width: number } | null>(null)

  // 测量激活 Trigger 相对列表的位置,驱动指示条滑动
  const measure = useCallback(() => {
    const list = listRef.current
    if (!list) return
    const active = list.querySelector<HTMLElement>('[role="tab"][data-state="active"]')
    setIndicator(active ? { left: active.offsetLeft, width: active.offsetWidth } : null)
  }, [])

  // 激活值变化后同步测量(useLayoutEffect 避免指示条闪跳)
  useLayoutEffect(() => {
    measure()
  }, [measure, activeValue])

  // 尺寸与字体导致的排布变化都要重新测量:
  // 1) 视口 resize;2) 列表容器尺寸变化(子项增删/换行会引发 list 重排,observe list 即可);
  // 3) webfont 加载完成后字宽变化(document.fonts 在旧环境可能不存在,故可选链)
  useEffect(() => {
    window.addEventListener('resize', measure)
    const observer = new ResizeObserver(measure)
    if (listRef.current) observer.observe(listRef.current)
    document.fonts?.ready.then(measure)
    return () => {
      window.removeEventListener('resize', measure)
      observer.disconnect()
    }
  }, [measure])

  return (
    <TabsPrimitive.List
      ref={listRef}
      className={cn('relative flex items-center gap-1 border-b border-line', className)}
      {...rest}
    >
      {children}
      {indicator && (
        <span
          aria-hidden="true"
          // 1px 基准宽(w-px)经 scaleX 放大到实际宽,位移走 translateX:只动 transform
          className="absolute bottom-0 left-0 h-0.5 w-px origin-left bg-primary transition-transform duration-base ease-in-out"
          // 动态定位:transform 数值由测量结果计算(样式铁律的定位计算例外)
          style={{ transform: `translateX(${indicator.left}px) scaleX(${indicator.width})` }}
        />
      )}
    </TabsPrimitive.List>
  )
}

export interface TabsTriggerProps extends ComponentPropsWithoutRef<typeof TabsPrimitive.Trigger> {
  /** 可选前置图标(Lucide) */
  icon?: LucideIcon
}

export function TabsTrigger({ icon, className, children, ...rest }: TabsTriggerProps) {
  return (
    <TabsPrimitive.Trigger
      className={cn(
        'inline-flex min-h-11 items-center gap-1.5 rounded-sm px-3 py-2 text-sm text-ink-sub',
        'transition-colors duration-fast hover:text-ink',
        'data-[state=active]:font-medium data-[state=active]:text-primary',
        'focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2',
        'disabled:pointer-events-none disabled:opacity-50',
        className
      )}
      {...rest}
    >
      {icon && <Icon icon={icon} size="sm" />}
      {children}
    </TabsPrimitive.Trigger>
  )
}

export type TabsContentProps = ComponentPropsWithoutRef<typeof TabsPrimitive.Content>

export function TabsContent({ className, ...rest }: TabsContentProps) {
  return (
    <TabsPrimitive.Content
      className={cn(
        'pt-4 focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2',
        className
      )}
      {...rest}
    />
  )
}
