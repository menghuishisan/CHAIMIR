// 统一字段标签、帮助和错误，并关联单个控件的 ID 与 ARIA 属性。
import { cloneElement, Fragment, isValidElement, useId } from "react";
import type { AriaAttributes, ReactElement, ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface FormFieldProps {
  label: string;
  /** 控件 id;不传则自动生成并注入单个子元素 */
  htmlFor?: string;
  required?: boolean;
  helper?: string;
  /** 有错误时替换帮助文案；null 表示校验通过。 */
  error?: string | null;
  children: ReactNode;
  className?: string;
  /** 深色面板中的标签、说明和错误使用对应语义令牌。 */
  onDark?: boolean;
}

/** FormField 自动关联单个控件；显式 htmlFor 时由调用方提供匹配的控件 id。 */
export function FormField({
  label,
  htmlFor,
  required = false,
  helper,
  error,
  children,
  className,
  onDark = false,
}: FormFieldProps) {
  const generatedId = useId();
  const errorId = `${generatedId}-error`;
  const helperId = `${generatedId}-helper`;
  const childId = isValidElement(children) ? (children.props as { id?: string }).id : undefined;
  const fieldId = htmlFor ?? childId ?? generatedId;
  const messageId = error ? errorId : helper ? helperId : undefined;

  // Fragment 不能承载控件属性。
  let content = children;
  if (isValidElement(children) && children.type !== Fragment) {
    const childProps = children.props as { id?: string } & AriaAttributes;
    const describedBy =
      [childProps["aria-describedby"], messageId].filter(Boolean).join(" ") || undefined;
    content = cloneElement(children as ReactElement<{ id?: string } & AriaAttributes>, {
      id: !htmlFor && !childId ? fieldId : childProps.id,
      "aria-describedby": describedBy,
      "aria-invalid": error ? true : childProps["aria-invalid"],
    });
  }

  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <label htmlFor={fieldId} className={cn("text-sm font-medium", onDark ? "text-on-dark" : "text-ink")}>
        {label}
        {required && (
          <>
            <span aria-hidden="true" className={onDark ? "text-on-dark-danger" : "text-danger"}> *</span>
            <span className="sr-only">必填</span>
          </>
        )}
      </label>
      {content}
      {error ? (
        <p id={errorId} role="alert" className={cn("text-xs", onDark ? "text-on-dark-danger" : "text-danger")}>
          {error}
        </p>
      ) : (
        helper && (
          <p id={helperId} className={cn("text-xs", onDark ? "text-on-dark-sub" : "text-ink-sub")}>
            {helper}
          </p>
        )
      )}
    </div>
  );
}
