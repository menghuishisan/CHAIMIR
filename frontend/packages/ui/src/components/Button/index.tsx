/**
 * Button:平台统一按钮组件。
 * CVA 变体:primary(玉实底)/ seal(朱砂落印)/ outline / ghost / danger / on-dark(深色语境);
 * 尺寸 sm/md/lg;按压反馈走 pressable 工具类。
 * loading 时以 LoaderCircle 旋转图标替换左图标,但不设原生 disabled:
 * 原生 disabled 会让浏览器把焦点甩回 body,读屏用户丢失上下文;
 * 改用 aria-disabled + aria-busy + onClick 守卫拦截,元素保持可聚焦。
 */
import { forwardRef } from "react";
import type { ButtonHTMLAttributes, MouseEvent } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { LoaderCircle } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon, type IconSize } from "../../lib/icon";

/** 变体与尺寸样式(仅主题工具类,零任意值;四态齐全)。
 * 对外导出:少数必须是链接的按钮(如认证页成功态的「前往登录」)需要同一套视觉,
 * 由调用方把它套到 <Link> 上,避免把类名再抄一遍写出第二套落印按钮。 */
export const buttonVariants = cva(
  "pressable inline-flex select-none items-center justify-center gap-2 whitespace-nowrap rounded-md font-medium focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        primary: "bg-primary text-on-solid hover:bg-primary-hover",
        seal: "bg-seal-action text-on-solid hover:bg-seal-action-hover",
        outline: "border border-line-strong bg-transparent text-ink hover:bg-surface-hover",
        ghost: "text-ink-sub hover:bg-surface-hover hover:text-ink",
        danger: "bg-danger text-on-solid hover:bg-danger-hover",
        "on-dark": "border border-dark-line text-on-dark-sub hover:bg-dark-elevated hover:text-on-dark",
      },
      size: {
        sm: "h-8 px-3 text-sm",
        md: "h-9 px-4 text-base",
        lg: "h-11 px-5 text-md",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  },
);

/** 按钮尺寸 → 图标尺寸阶映射(与字号节奏匹配) */
const ICON_SIZE_BY_BUTTON: Record<"sm" | "md" | "lg", IconSize> = {
  sm: "sm",
  md: "sm",
  lg: "md",
};

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  /** 异步进行中:旋转图标替换左图标,点击被守卫拦截(aria-disabled/aria-busy,不设原生 disabled 以保持可聚焦) */
  loading?: boolean;
  /** 左侧图标(Lucide 组件) */
  leftIcon?: LucideIcon;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    className,
    variant,
    size,
    loading = false,
    leftIcon,
    disabled,
    type = "button",
    onClick,
    children,
    ...rest
  },
  ref,
) {
  const iconSize = ICON_SIZE_BY_BUTTON[size ?? "md"];
  const blocked = disabled || loading;
  // 点击守卫:loading/disabled 时拦截并不调用用户 onClick。
  // loading 不设原生 disabled(保持可聚焦,焦点不被浏览器甩回 body),故必须在这里拦截;
  // 键盘 Enter/Space 触发的也是 click 事件,同样被拦截
  const handleClick = (event: MouseEvent<HTMLButtonElement>) => {
    if (blocked) {
      event.preventDefault();
      return;
    }
    onClick?.(event);
  };
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled}
      aria-disabled={blocked || undefined}
      aria-busy={loading || undefined}
      onClick={handleClick}
      className={cn(
        buttonVariants({ variant, size }),
        // loading 复用禁用视觉(opacity-50)但不加 pointer-events-none:
        // 保持元素可聚焦可命中,拦截交由 handleClick;
        // pressable 的按压缩放由 :active:not(:disabled) 控制,loading 非原生禁用仍会缩,
        // 用 active:transform-none! 抑制(pressable 的 :active 规则特异性更高,需 important 压过)
        loading && "opacity-50 active:transform-none!",
        className,
      )}
      {...rest}
    >
      {/* loading 时旋转图标顶替左图标位,保持内容节奏不跳动 */}
      {loading ? (
        <Icon icon={LoaderCircle} size={iconSize} className="animate-spin" />
      ) : (
        leftIcon && <Icon icon={leftIcon} size={iconSize} />
      )}
      {children}
    </button>
  );
});
