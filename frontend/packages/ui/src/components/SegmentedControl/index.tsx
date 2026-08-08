/**
 * SegmentedControl:分段切换器(视图/模式切换,基于 Radix RadioGroup 横向封装)。
 * 容器下陷底 + 激活项浮起(bg-surface + shadow-xs);
 * 深色面板(沉浸式工作台,§7.1)传 onDark,改用墨底语义令牌,不在页面里另拼一套配色。
 * 单选语义与方向键切换由 Radix 保证;过渡 duration-fast。
 */
import * as RadioGroupPrimitive from "@radix-ui/react-radio-group";
import { cva } from "class-variance-authority";
import type { LucideIcon } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface SegmentedOption {
  value: string;
  label: string;
  /** 可选前置图标(Lucide 组件) */
  icon?: LucideIcon;
}

export interface SegmentedControlProps {
  options: SegmentedOption[];
  value: string;
  onValueChange: (value: string) => void;
  size?: "sm" | "md";
  disabled?: boolean;
  /** 深色面板语境(沉浸式工作台) */
  onDark?: boolean;
  /** 分组的无障碍名称(说明在切换什么) */
  "aria-label"?: string;
  className?: string;
}

/** 分段项样式:光面语境激活项浮起为白面,墨底语境激活项浮起为深色面板 + 玉色文字 */
const segmentedItemVariants = cva(
  "hit-target relative inline-flex min-h-11 select-none items-center gap-1.5 rounded-md font-medium transition-colors duration-fast focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      size: {
        sm: "h-6 px-2 text-xs",
        md: "h-7 px-3 text-sm",
      },
      onDark: {
        false:
          "text-ink-sub hover:text-ink data-[state=checked]:bg-surface data-[state=checked]:text-ink data-[state=checked]:shadow-xs",
        true:
          "text-on-dark-sub hover:text-on-dark data-[state=checked]:bg-dark-elevated data-[state=checked]:text-accent",
      },
    },
    defaultVariants: { size: "md", onDark: false },
  },
);

export function SegmentedControl({
  options,
  value,
  onValueChange,
  size = "md",
  disabled,
  onDark = false,
  "aria-label": ariaLabel,
  className,
}: SegmentedControlProps) {
  return (
    <RadioGroupPrimitive.Root
      orientation="horizontal"
      value={value}
      onValueChange={onValueChange}
      disabled={disabled}
      aria-label={ariaLabel}
      className={cn(
        "inline-flex items-center gap-1 rounded-lg p-1",
        onDark ? "border border-dark-line bg-dark-surface" : "bg-surface-sunken",
        className,
      )}
    >
      {options.map((option) => (
        <RadioGroupPrimitive.Item
          key={option.value}
          value={option.value}
          className={segmentedItemVariants({ size, onDark })}
        >
          {option.icon && <Icon icon={option.icon} size="sm" />}
          {option.label}
        </RadioGroupPrimitive.Item>
      ))}
    </RadioGroupPrimitive.Root>
  );
}
