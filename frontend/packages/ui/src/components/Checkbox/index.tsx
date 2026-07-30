/**
 * Checkbox:复选框(基于 Radix Checkbox 封装)。
 * 选中态玉色实底 + 白色对勾;传 label 时整行(框 + 文字)可点;
 * 键盘与 aria-checked 语义由 Radix 保证。
 */
import { forwardRef } from "react";
import type { ComponentPropsWithoutRef, ElementRef } from "react";
import * as CheckboxPrimitive from "@radix-ui/react-checkbox";
import { Check } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface CheckboxProps extends ComponentPropsWithoutRef<typeof CheckboxPrimitive.Root> {
  /** 复选项文字:传入后整行可点 */
  label?: string;
}

export const Checkbox = forwardRef<ElementRef<typeof CheckboxPrimitive.Root>, CheckboxProps>(
  function Checkbox({ className, label, disabled, ...rest }, ref) {
    const box = (
      <CheckboxPrimitive.Root
        ref={ref}
        disabled={disabled}
        className={cn(
          // hover 边框转主色作可点暗示;选中态边框已是主色,叠加无视觉冲突
          "flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border border-line-strong bg-surface transition-colors duration-fast hover:border-primary data-[state=checked]:border-primary data-[state=checked]:bg-primary focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none",
          // 无 label 时禁用态在框上弱化;有 label 时由外层 label 统一弱化,避免二次叠加
          !label && "disabled:opacity-50",
          !label && className,
        )}
        {...rest}
      >
        <CheckboxPrimitive.Indicator className="flex items-center justify-center text-on-solid">
          <Icon icon={Check} size="xs" />
        </CheckboxPrimitive.Indicator>
      </CheckboxPrimitive.Root>
    );

    if (!label) return box;

    // label 包裹使文字区域同样可点;禁用时整行弱化并阻断点击手势。
    // 文字色定义在外层(内层继承),深色语境可经 className 覆写(如 text-on-dark-sub)。
    return (
      <label
        className={cn(
          "inline-flex items-center gap-2 text-ink",
          disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer",
          className,
        )}
      >
        {box}
        <span className="select-none text-base">{label}</span>
      </label>
    );
  },
);
