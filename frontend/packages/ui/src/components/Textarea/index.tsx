/**
 * Textarea:多行文本输入框。
 * 盒式风格与 Input 的 boxed 变体一致;rows 默认 4;
 * 支持 invalid 错误态(aria-invalid + 危险色边框)。
 */
import { forwardRef } from "react";
import type { TextareaHTMLAttributes } from "react";
import { cn } from "../../lib/cn";

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  /** 校验错误态:置 aria-invalid 并转危险色边框 */
  invalid?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className, invalid = false, rows = 4, ...rest },
  ref,
) {
  return (
    <textarea
      ref={ref}
      rows={rows}
      aria-invalid={invalid || undefined}
      className={cn(
        // outline-hidden(非 outline-none):保留强制对比色模式下的系统焦点可见性
        "w-full rounded-md border border-line-strong bg-surface px-3 py-2 text-base text-ink transition-colors duration-fast placeholder:text-ink-faint read-only:bg-surface-sunken hover:border-ink-faint focus:border-primary focus:outline-hidden focus:ring-2 focus:ring-primary-soft disabled:pointer-events-none disabled:opacity-50",
        // 错误态:hover/focus 均保持危险色(显式覆盖基类 hover),提示用户问题尚未解决
        invalid && "border-danger hover:border-danger focus:border-danger focus:ring-danger-bg",
        className,
      )}
      {...rest}
    />
  );
});
