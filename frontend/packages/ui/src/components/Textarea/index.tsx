/**
 * Textarea:多行文本输入框。
 * 盒式风格与 Input 的 boxed 变体一致;rows 默认 4;
 * 支持 invalid 错误态(aria-invalid + 危险色边框)。
 * 深色面板(沉浸式工作台,§7.1)传 onDark,改用墨底语义令牌 —— 页面不在深色语境里另拼一套配色。
 */
import { forwardRef } from "react";
import type { TextareaHTMLAttributes } from "react";
import { cn } from "../../lib/cn";

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  /** 校验错误态:置 aria-invalid 并转危险色边框 */
  invalid?: boolean;
  /** 深色面板语境(沉浸式工作台):改用墨底语义令牌 */
  onDark?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className, invalid = false, onDark = false, rows = 4, ...rest },
  ref,
) {
  return (
    <textarea
      ref={ref}
      rows={rows}
      aria-invalid={invalid || undefined}
      className={cn(
        // outline-hidden(非 outline-none):保留强制对比色模式下的系统焦点可见性
        "w-full rounded-md border px-3 py-2 text-base transition-colors duration-fast focus:outline-hidden disabled:pointer-events-none disabled:opacity-50",
        onDark
          ? "border-dark-line bg-dark-surface text-on-dark placeholder:text-on-dark-faint hover:border-on-dark-faint focus:border-accent"
          : "border-line-strong bg-surface text-ink placeholder:text-ink-faint read-only:bg-surface-sunken hover:border-ink-faint focus:border-primary focus:ring-2 focus:ring-primary-soft",
        // 错误态:hover/focus 均保持危险色(显式覆盖基类 hover),提示用户问题尚未解决
        invalid && "border-danger hover:border-danger focus:border-danger",
        !onDark && invalid && "focus:ring-danger-bg",
        className,
      )}
      {...rest}
    />
  );
});
