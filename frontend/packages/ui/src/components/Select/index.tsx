import * as SelectPrimitive from "@radix-ui/react-select";
import type { AriaAttributes, FocusEventHandler } from "react";
import { cva } from "class-variance-authority";
import { Check, ChevronDown, ChevronUp } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps extends Pick<AriaAttributes, "aria-describedby" | "aria-invalid"> {
  value?: string;
  onValueChange?: (value: string) => void;
  /** 实际触发器失焦时执行字段校验。 */
  onBlur?: FocusEventHandler<HTMLButtonElement>;
  /** 未选择时的占位文案 */
  placeholder?: string;
  options: SelectOption[];
  disabled?: boolean;
  /** 校验错误态:置 aria-invalid 并转危险色边框 */
  invalid?: boolean;
  size?: "sm" | "md";
  /** 深色面板语境(沉浸式工作台):改用墨底语义令牌 */
  onDark?: boolean;
  /** 无 label 场景下的无障碍名称 */
  "aria-label"?: string;
  /** 供 FormField 关联 label 的控件 id */
  id?: string;
  className?: string;
}

/** 可搜索选择器与单选下拉框复用同一触发器外观。 */
export const selectTriggerVariants = cva(
  "inline-flex pointer-coarse:min-h-11 w-full items-center justify-between gap-2 rounded-md border px-3 transition-colors duration-fast focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      tone: {
        light: "border-line-control bg-surface text-ink data-[placeholder]:text-ink-sub hover:border-ink-sub",
        dark: "border-dark-control bg-dark-elevated text-on-dark data-[placeholder]:text-on-dark-sub hover:border-on-dark-sub",
      },
      size: { sm: "h-8 text-sm", md: "h-9 text-base" },
      // 错误态:hover 同样保持危险色(显式覆盖基类 hover),避免误导为已修复
      invalid: { true: "border-danger hover:border-danger", false: "" },
    },
    defaultVariants: { tone: "light", size: "md", invalid: false },
  },
);

/** Select 由 Radix 管理键盘与浮层；表单语义传到实际触发器。 */
export function Select({
  value,
  onValueChange,
  onBlur,
  placeholder,
  options,
  disabled,
  invalid = false,
  size = "md",
  onDark = false,
  "aria-label": ariaLabel,
  "aria-describedby": ariaDescribedBy,
  "aria-invalid": ariaInvalid,
  id,
  className,
}: SelectProps) {
  const hasError = invalid || (ariaInvalid !== undefined && ariaInvalid !== false && ariaInvalid !== "false");
  return (
    <SelectPrimitive.Root value={value} onValueChange={onValueChange} disabled={disabled}>
      {/* 触发器:与盒式 Input 同视觉,占位态用弱化文字色 */}
      <SelectPrimitive.Trigger
        id={id}
        onBlur={onBlur}
        aria-label={ariaLabel}
        aria-describedby={ariaDescribedBy}
        aria-invalid={invalid || ariaInvalid || undefined}
        className={cn(selectTriggerVariants({ tone: onDark ? "dark" : "light", size, invalid: hasError }), className)}
      >
        <SelectPrimitive.Value placeholder={placeholder} />
        <SelectPrimitive.Icon className={onDark ? "text-on-dark-sub" : "text-ink-sub"}>
          <Icon icon={ChevronDown} size="sm" />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content
          position="popper"
          sideOffset={4}
          className={cn(
            "z-modal max-h-72 overflow-hidden rounded-lg border shadow-md origin-radix-select data-[state=open]:animate-pop-in data-[state=closed]:animate-pop-out",
            onDark ? "border-dark-line bg-dark-elevated" : "border-line bg-surface",
          )}
        >
          {/* 超长列表滚动指示(仅溢出时由 Radix 渲染) */}
          <SelectPrimitive.ScrollUpButton
            className={cn("flex items-center justify-center py-1", onDark ? "text-on-dark-sub" : "text-ink-sub")}
          >
            <Icon icon={ChevronUp} size="sm" />
          </SelectPrimitive.ScrollUpButton>
          <SelectPrimitive.Viewport className="p-1">
            {options.map((option) => (
              <SelectPrimitive.Item
                key={option.value}
                value={option.value}
                disabled={option.disabled}
                className={cn(
                  // outline-hidden:高亮底色承担焦点反馈,强制对比色模式下仍保留系统焦点
                  "relative flex cursor-default select-none items-center rounded-sm py-1.5 pl-2 pr-8 outline-hidden transition-colors duration-fast data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
                  onDark
                    ? "text-on-dark data-[highlighted]:bg-dark-surface"
                    : "text-ink data-[highlighted]:bg-surface-hover",
                  size === "sm" ? "text-sm" : "text-base",
                )}
              >
                <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
                {/* 选中指示:右侧玉色对勾 */}
                <SelectPrimitive.ItemIndicator
                  className={cn("absolute right-2 flex items-center", onDark ? "text-accent" : "text-primary")}
                >
                  <Icon icon={Check} size="sm" />
                </SelectPrimitive.ItemIndicator>
              </SelectPrimitive.Item>
            ))}
          </SelectPrimitive.Viewport>
          <SelectPrimitive.ScrollDownButton
            className={cn("flex items-center justify-center py-1", onDark ? "text-on-dark-sub" : "text-ink-sub")}
          >
            <Icon icon={ChevronDown} size="sm" />
          </SelectPrimitive.ScrollDownButton>
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  );
}
