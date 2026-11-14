/**
 * Input:单行文本输入框。
 * 变体 boxed(默认,纸面语境)/ dark(认证页与工作台的深色变体,有底色,比只有一条底线更好辨认);
 * 两种变体的轮廓都用 ≥3:1 的控件边线令牌(WCAG 1.4.11);窄屏字号 16 防 iOS 自动放大;
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

/** 输入框本体样式:boxed 白底盒式,dark 墨底盒式 */
const inputVariants = cva(
  "h-9 w-full text-md md:text-base transition-colors duration-fast disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        // outline-hidden(非 outline-none):保留强制对比色模式下的系统焦点可见性
        boxed:
          "rounded-md border border-line-control bg-surface px-3 text-ink placeholder:text-ink-sub read-only:bg-surface-sunken hover:border-ink-sub focus:border-primary focus:outline-hidden focus:ring-2 focus:ring-primary-soft",
        dark:
          "rounded-md border border-dark-control bg-dark-elevated px-3 text-on-dark placeholder:text-on-dark-sub hover:border-on-dark-sub focus:border-accent focus:outline-hidden focus:ring-2 focus:ring-on-dark-accent-soft",
      },
      invalid: {
        true: "",
        false: "",
      },
    },
    compoundVariants: [
      // 错误态:边框转危险色;hover/focus 均保持危险色(显式覆盖基类 hover),避免误导为已修复
      { variant: "boxed", invalid: true, class: "border-danger hover:border-danger focus:border-danger focus:ring-danger-bg" },
      { variant: "dark", invalid: true, class: "border-on-dark-danger hover:border-on-dark-danger focus:border-on-dark-danger" },
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
  const onDark = variant === "dark";

  return (
    <div className="relative w-full">
      {leftIcon && (
        <span
          className={cn(
            "pointer-events-none absolute left-3 top-1/2 -translate-y-1/2",
            onDark ? "text-on-dark-faint" : "text-ink-faint",
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
          leftIcon && "pl-9",
          isPassword && "pr-9",
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
            "right-2",
            onDark ? "text-on-dark-sub hover:text-on-dark" : "text-ink-sub hover:text-ink",
          )}
        >
          <Icon icon={passwordVisible ? EyeOff : Eye} size="sm" />
        </button>
      )}
    </div>
  );
});
