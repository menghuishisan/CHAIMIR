/**
 * IconButton:纯图标按钮。
 * 无可见文字,类型上强制要求 aria-label(无障碍铁律);
 * 变体 ghost / outline / on-dark,尺寸 sm/md,按压反馈走 pressable。
 */
import { forwardRef } from "react";
import type { ButtonHTMLAttributes } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import type { LucideIcon } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon, type IconSize } from "../../lib/icon";

/** 变体与尺寸样式(方形等宽高,图标居中;hit-target 把命中区扩到 44×44 达标) */
const iconButtonVariants = cva(
  "hit-target pressable inline-flex shrink-0 select-none items-center justify-center rounded-md focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        ghost: "text-ink-sub hover:bg-surface-hover hover:text-ink",
        outline: "border border-line-strong bg-transparent text-ink hover:bg-surface-hover",
        "on-dark": "border border-dark-line text-on-dark-sub hover:bg-dark-elevated hover:text-on-dark",
      },
      size: {
        sm: "h-8 w-8",
        md: "h-9 w-9",
      },
    },
    defaultVariants: { variant: "ghost", size: "md" },
  },
);

/** 按钮尺寸 → 图标尺寸阶映射 */
const ICON_SIZE_BY_BUTTON: Record<"sm" | "md", IconSize> = { sm: "sm", md: "md" };

export interface IconButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof iconButtonVariants> {
  /** 无障碍名称:纯图标按钮必填,读屏用户依赖它理解用途 */
  "aria-label": string;
  /** 图标(Lucide 组件) */
  icon: LucideIcon;
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  { className, variant, size, icon, type = "button", ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      className={cn(iconButtonVariants({ variant, size }), className)}
      {...rest}
    >
      <Icon icon={icon} size={ICON_SIZE_BY_BUTTON[size ?? "md"]} />
    </button>
  );
});
