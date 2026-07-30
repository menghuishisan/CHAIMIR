/**
 * SegmentedControl:分段切换器(视图/模式切换,基于 Radix RadioGroup 横向封装)。
 * 容器下陷底 + 激活项浮起(bg-surface + shadow-xs);
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
  /** 分组的无障碍名称(说明在切换什么) */
  "aria-label"?: string;
  className?: string;
}

/** 分段项样式:激活项浮起为白面,非激活弱化文字 */
const segmentedItemVariants = cva(
  "inline-flex select-none items-center gap-1.5 rounded-md font-medium text-ink-sub transition-colors duration-fast hover:text-ink data-[state=checked]:bg-surface data-[state=checked]:text-ink data-[state=checked]:shadow-xs focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      size: {
        sm: "h-6 px-2 text-xs",
        md: "h-7 px-3 text-sm",
      },
    },
    defaultVariants: { size: "md" },
  },
);

export function SegmentedControl({
  options,
  value,
  onValueChange,
  size = "md",
  disabled,
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
      className={cn("inline-flex items-center gap-1 rounded-lg bg-surface-sunken p-1", className)}
    >
      {options.map((option) => (
        <RadioGroupPrimitive.Item
          key={option.value}
          value={option.value}
          className={segmentedItemVariants({ size })}
        >
          {option.icon && <Icon icon={option.icon} size="sm" />}
          {option.label}
        </RadioGroupPrimitive.Item>
      ))}
    </RadioGroupPrimitive.Root>
  );
}
