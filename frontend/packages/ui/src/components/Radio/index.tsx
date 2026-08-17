/**
 * Radio:单选组(基于 Radix RadioGroup 封装)。
 * 导出 RadioGroup(容器,默认纵向排列)与 RadioItem(选项,带 label 整行可点);
 * 选中态玉色圆点;方向键切换由 Radix 保证。
 */
import { forwardRef } from 'react'
import type { ComponentPropsWithoutRef, ElementRef } from 'react'
import * as RadioGroupPrimitive from '@radix-ui/react-radio-group'
import { cn } from '../../lib/cn'

export type RadioGroupProps = ComponentPropsWithoutRef<typeof RadioGroupPrimitive.Root>

/** 单选组容器:默认纵向 gap-2,可经 className 改横向 */
export const RadioGroup = forwardRef<ElementRef<typeof RadioGroupPrimitive.Root>, RadioGroupProps>(
  function RadioGroup({ className, ...rest }, ref) {
    return (
      <RadioGroupPrimitive.Root
        ref={ref}
        className={cn('flex flex-col gap-2', className)}
        {...rest}
      />
    )
  }
)

export interface RadioItemProps extends ComponentPropsWithoutRef<typeof RadioGroupPrimitive.Item> {
  /** 选项文字:传入后整行可点 */
  label?: string
}

/** 单选项:圆形指示器,选中时中心出现玉色圆点 */
export const RadioItem = forwardRef<ElementRef<typeof RadioGroupPrimitive.Item>, RadioItemProps>(
  function RadioItem({ className, label, disabled, ...rest }, ref) {
    const circle = (
      <RadioGroupPrimitive.Item
        ref={ref}
        disabled={disabled}
        className={cn(
          // hover 边框转主色作可点暗示;选中态边框已是主色,叠加无视觉冲突
          'hit-target relative flex h-4 w-4 shrink-0 items-center justify-center rounded-full border border-line-strong bg-surface transition-colors duration-fast hover:border-primary data-[state=checked]:border-primary focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none',
          // 无 label 时禁用态在圆圈上弱化;有 label 时由外层 label 统一弱化
          !label && 'disabled:opacity-50',
          !label && className
        )}
        {...rest}
      >
        <RadioGroupPrimitive.Indicator className="flex items-center justify-center">
          <span className="h-2 w-2 rounded-full bg-primary" />
        </RadioGroupPrimitive.Indicator>
      </RadioGroupPrimitive.Item>
    )

    if (!label) return circle

    return (
      <label
        className={cn(
          'inline-flex items-center gap-2',
          disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer',
          className
        )}
      >
        {circle}
        <span className="select-none text-base text-ink">{label}</span>
      </label>
    )
  }
)
