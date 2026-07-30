/**
 * Switch:开关(基于 Radix Switch 封装)。
 * 开=玉色实底、关=浅灰轨道;白色滑块 transform 平滑滑动(duration-fast);
 * 传 label 时整行可点;aria-checked 语义由 Radix 保证。
 */
import { forwardRef } from "react";
import type { ComponentPropsWithoutRef, ElementRef } from "react";
import * as SwitchPrimitive from "@radix-ui/react-switch";
import { cn } from "../../lib/cn";

export interface SwitchProps extends ComponentPropsWithoutRef<typeof SwitchPrimitive.Root> {
  /** 开关文字:传入后整行可点 */
  label?: string;
}

export const Switch = forwardRef<ElementRef<typeof SwitchPrimitive.Root>, SwitchProps>(
  function Switch({ className, label, disabled, ...rest }, ref) {
    const track = (
      <SwitchPrimitive.Root
        ref={ref}
        disabled={disabled}
        className={cn(
          // 未选中 hover 轨道加深作可点暗示;限定 data-[state=unchecked] 保证不与 checked 的主色底争覆盖
          "inline-flex h-5 w-9 shrink-0 items-center rounded-full bg-line-strong p-0.5 transition-colors duration-fast hover:data-[state=unchecked]:bg-ink-faint data-[state=checked]:bg-primary focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none",
          // 无 label 时禁用态在轨道上弱化;有 label 时由外层 label 统一弱化
          !label && "disabled:opacity-50",
          !label && className,
        )}
        {...rest}
      >
        {/* 滑块:轨道宽 36 - 内边距 4 - 滑块 16 = 行程 16px */}
        <SwitchPrimitive.Thumb className="block h-4 w-4 translate-x-0 rounded-full bg-on-solid shadow-xs transition-transform duration-fast data-[state=checked]:translate-x-4" />
      </SwitchPrimitive.Root>
    );

    if (!label) return track;

    return (
      <label
        className={cn(
          "inline-flex items-center gap-2",
          disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer",
          className,
        )}
      >
        {track}
        <span className="select-none text-base text-ink">{label}</span>
      </label>
    );
  },
);
