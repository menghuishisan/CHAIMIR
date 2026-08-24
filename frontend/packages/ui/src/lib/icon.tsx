/**
 * Icon:Lucide 图标统一封装(FE-3)。
 * 尺寸走令牌阶(xs12/sm16/md18/lg20),描边统一 1.8;
 * 默认装饰性(aria-hidden),传 label 时转为语义图标(role=img)。
 */
import type { LucideIcon } from "lucide-react";
import { cn } from "./cn";

const ICON_SIZE = { xs: 12, sm: 16, md: 18, lg: 20 } as const;

export type IconSize = keyof typeof ICON_SIZE;

export interface IconProps {
  /** Lucide 图标组件,如 icon={BookOpen} */
  icon: LucideIcon;
  /** 尺寸阶:xs 12 / sm 16 / md 18 / lg 20,默认 md */
  size?: IconSize;
  /** 读屏标签;不传则视为纯装饰(aria-hidden) */
  label?: string;
  className?: string;
}

export function Icon({ icon: LucideCmp, size = "md", label, className }: IconProps) {
  return (
    <LucideCmp
      size={ICON_SIZE[size]}
      strokeWidth={1.8}
      aria-hidden={label ? undefined : true}
      role={label ? "img" : undefined}
      aria-label={label}
      className={cn("shrink-0", className)}
    />
  );
}
