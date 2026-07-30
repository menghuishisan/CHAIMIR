/**
 * Input:单行文本输入框。
 * 变体 boxed(默认盒式,光面语境)/ underline(认证页无盒变体,深色语境仅下边线);
 * 支持左图标、invalid 错误态(aria-invalid + 危险色边框);
 * type="password" 时内置显隐切换按钮(Eye/EyeOff,带中文 aria-label)。
 */
import { forwardRef, useState } from "react";
import type { InputHTMLAttributes } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { Eye, EyeOff } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

/** 输入框本体样式:boxed 走宣纸盒式,underline 走深底下边线 */
const inputVariants = cva(
  "h-9 w-full text-base transition-colors duration-fast disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        // outline-hidden(非 outline-none):保留强制对比色模式下的系统焦点可见性
        boxed:
          "rounded-md border border-line-strong bg-surface px-3 text-ink placeholder:text-ink-faint read-only:bg-surface-sunken hover:border-ink-faint focus:border-primary focus:outline-hidden focus:ring-2 focus:ring-primary-soft",
        underline:
          "rounded-none border-b border-dark-line bg-transparent px-0 text-on-dark placeholder:text-on-dark-faint hover:border-on-dark-faint focus:border-accent focus:outline-hidden",
      },
      invalid: {
        true: "",
        false: "",
      },
    },
    compoundVariants: [
      // 错误态:边框转危险色;hover/focus 均保持危险色(显式覆盖基类 hover),避免误导为已修复
      { variant: "boxed", invalid: true, class: "border-danger hover:border-danger focus:border-danger focus:ring-danger-bg" },
      { variant: "underline", invalid: true, class: "border-danger hover:border-danger focus:border-danger" },
    ],
    defaultVariants: { variant: "boxed", invalid: false },
  },
);

export interface InputProps
  extends InputHTMLAttributes<HTMLInputElement>,
    Omit<VariantProps<typeof inputVariants>, "invalid"> {
  /** 校验错误态:置 aria-invalid 并转危险色边框 */
  invalid?: boolean;
  /** 左侧装饰图标(Lucide 组件) */
  leftIcon?: LucideIcon;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, variant, invalid = false, leftIcon, type, disabled, ...rest },
  ref,
) {
  // 密码显隐:仅 type="password" 时启用切换按钮
  const isPassword = type === "password";
  const [passwordVisible, setPasswordVisible] = useState(false);
  const effectiveType = isPassword && passwordVisible ? "text" : type;
  const onDark = variant === "underline";

  return (
    <div className="relative w-full">
      {leftIcon && (
        <span
          className={cn(
            "pointer-events-none absolute top-1/2 -translate-y-1/2",
            onDark ? "left-0 text-on-dark-faint" : "left-3 text-ink-faint",
          )}
        >
          <Icon icon={leftIcon} size="sm" />
        </span>
      )}
      <input
        ref={ref}
        type={effectiveType}
        disabled={disabled}
        aria-invalid={invalid || undefined}
        className={cn(
          inputVariants({ variant, invalid }),
          // 为左图标与密码切换按钮让出内边距
          leftIcon && (onDark ? "pl-6" : "pl-9"),
          isPassword && (onDark ? "pr-7" : "pr-9"),
          className,
        )}
        {...rest}
      />
      {isPassword && (
        <button
          type="button"
          disabled={disabled}
          aria-label={passwordVisible ? "隐藏密码" : "显示密码"}
          onClick={() => setPasswordVisible((v) => !v)}
          className={cn(
            // hit-target:视觉尺寸不变,透明命中区扩到 44×44(§3.2)
            "hit-target absolute top-1/2 -translate-y-1/2 rounded-sm p-1 transition-colors duration-fast focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-50",
            onDark ? "right-0 text-on-dark-sub hover:text-on-dark" : "right-2 text-ink-sub hover:text-ink",
          )}
        >
          <Icon icon={passwordVisible ? EyeOff : Eye} size="sm" />
        </button>
      )}
    </div>
  );
});
