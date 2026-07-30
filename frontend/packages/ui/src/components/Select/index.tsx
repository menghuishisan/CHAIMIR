/**
 * Select:单选下拉框(基于 Radix Select 封装)。
 * Trigger 样式与盒式 Input 一致;Content 走 Portal + z-dropdown,
 * 进出场 pop-in / pop-out 且 transform-origin 对齐触发源(§4.3 浮层从触发源缩放淡入);
 * 选项带 Check 指示,键盘导航与 typeahead 由 Radix 保证。
 */
import * as SelectPrimitive from "@radix-ui/react-select";
import { Check, ChevronDown, ChevronUp } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps {
  value?: string;
  onValueChange?: (value: string) => void;
  /** 未选择时的占位文案 */
  placeholder?: string;
  options: SelectOption[];
  disabled?: boolean;
  /** 校验错误态:置 aria-invalid 并转危险色边框 */
  invalid?: boolean;
  size?: "sm" | "md";
  /** 无 label 场景下的无障碍名称 */
  "aria-label"?: string;
  /** 供 FormField 关联 label 的控件 id */
  id?: string;
  className?: string;
}

export function Select({
  value,
  onValueChange,
  placeholder,
  options,
  disabled,
  invalid = false,
  size = "md",
  "aria-label": ariaLabel,
  id,
  className,
}: SelectProps) {
  const isSm = size === "sm";
  return (
    <SelectPrimitive.Root value={value} onValueChange={onValueChange} disabled={disabled}>
      {/* 触发器:与盒式 Input 同视觉,占位态用弱化文字色 */}
      <SelectPrimitive.Trigger
        id={id}
        aria-label={ariaLabel}
        aria-invalid={invalid || undefined}
        className={cn(
          "inline-flex w-full items-center justify-between gap-2 rounded-md border border-line-strong bg-surface px-3 text-ink transition-colors duration-fast data-[placeholder]:text-ink-faint hover:border-ink-faint focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-50",
          isSm ? "h-8 text-sm" : "h-9 text-base",
          // 错误态:hover 同样保持危险色(显式覆盖基类 hover),避免误导为已修复
          invalid && "border-danger hover:border-danger",
          className,
        )}
      >
        <SelectPrimitive.Value placeholder={placeholder} />
        <SelectPrimitive.Icon className="text-ink-sub">
          <Icon icon={ChevronDown} size="sm" />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content
          position="popper"
          sideOffset={4}
          className="z-dropdown max-h-72 overflow-hidden rounded-lg border border-line bg-surface shadow-md origin-(--radix-select-content-transform-origin) data-[state=open]:animate-pop-in data-[state=closed]:animate-pop-out"
        >
          {/* 超长列表滚动指示(仅溢出时由 Radix 渲染) */}
          <SelectPrimitive.ScrollUpButton className="flex items-center justify-center py-1 text-ink-sub">
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
                  "relative flex cursor-default select-none items-center rounded-sm py-1.5 pl-2 pr-8 text-ink outline-hidden transition-colors duration-fast data-[highlighted]:bg-surface-hover data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
                  isSm ? "text-sm" : "text-base",
                )}
              >
                <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
                {/* 选中指示:右侧玉色对勾 */}
                <SelectPrimitive.ItemIndicator className="absolute right-2 flex items-center text-primary">
                  <Icon icon={Check} size="sm" />
                </SelectPrimitive.ItemIndicator>
              </SelectPrimitive.Item>
            ))}
          </SelectPrimitive.Viewport>
          <SelectPrimitive.ScrollDownButton className="flex items-center justify-center py-1 text-ink-sub">
            <Icon icon={ChevronDown} size="sm" />
          </SelectPrimitive.ScrollDownButton>
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  );
}
