/**
 * FormField:表单字段包装器。
 * 渲染显式 label[for] + 控件 + helper/error 文案(error 优先且 role="alert" 就近提示);
 * required 时加朱砂星号并配 sr-only「必填」;
 * 未传 htmlFor 时用 useId 生成 id 并注入单个子元素,保证 label 与控件始终关联;
 * 同时向子控件注入 aria-describedby(error 优先,其次 helper)与 aria-invalid,
 * 让读屏在聚焦控件时就能听到错误/说明。Fragment 子元素无法承载 id/aria,不注入。
 */
import { cloneElement, Fragment, isValidElement, useId } from "react";
import type { AriaAttributes, ReactElement, ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface FormFieldProps {
  /** 字段名称(显式 label 文字) */
  label: string;
  /** 控件 id;不传则自动生成并注入单个子元素 */
  htmlFor?: string;
  /** 必填标记:冷红星号 + sr-only「必填」 */
  required?: boolean;
  /** 辅助说明(与 error 互斥,error 优先) */
  helper?: string;
  /**
   * 错误文案:role="alert" 就近播报。
   * 允许 null —— 校验器以 null 表达「已通过」,调用方不必在每个字段处转成 undefined。
   */
  error?: string | null;
  children: ReactNode;
  className?: string;
}

export function FormField({
  label,
  htmlFor,
  required = false,
  helper,
  error,
  children,
  className,
}: FormFieldProps) {
  const generatedId = useId();
  const errorId = `${generatedId}-error`;
  const helperId = `${generatedId}-helper`;
  // id 归属优先级:显式 htmlFor > 子元素自带 id > 自动生成
  const childId = isValidElement(children) ? (children.props as { id?: string }).id : undefined;
  const fieldId = htmlFor ?? childId ?? generatedId;
  // 描述文案的关联目标:error 优先(与展示逻辑一致),其次 helper
  const messageId = error ? errorId : helper ? helperId : undefined;

  // 单个元素子节点时注入 id(仅需自动生成时)与 aria 关联;
  // Fragment 无法承载 id/aria 属性,注入无效,保持原样渲染
  let content = children;
  if (isValidElement(children) && children.type !== Fragment) {
    const childProps = children.props as { id?: string } & AriaAttributes;
    // 子元素已有 aria-describedby 时拼接,不覆盖调用方已有关联
    const describedBy =
      [childProps["aria-describedby"], messageId].filter(Boolean).join(" ") || undefined;
    content = cloneElement(children as ReactElement<{ id?: string } & AriaAttributes>, {
      // 仅在需要自动生成时注入 id,避免覆盖显式 htmlFor/子元素自带 id 的关联
      id: !htmlFor && !childId ? fieldId : childProps.id,
      "aria-describedby": describedBy,
      "aria-invalid": error ? true : childProps["aria-invalid"],
    });
  }

  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <label htmlFor={fieldId} className="text-sm font-medium text-ink">
        {label}
        {required && (
          <>
            {/* 必填星号用**冷红**不用朱砂:朱砂只给品牌章与落印动作(§2.1),
                而必填标记出现在全平台每一个必填字段上,用朱砂会把 9:1 的克制比例直接反过来 */}
            <span aria-hidden="true" className="text-danger"> *</span>
            <span className="sr-only">必填</span>
          </>
        )}
      </label>
      {content}
      {/* error 与 helper 互斥:出错时只展示错误,避免信息冲突;id 供控件 aria-describedby 引用 */}
      {error ? (
        <p id={errorId} role="alert" className="text-xs text-danger">
          {error}
        </p>
      ) : (
        helper && (
          <p id={helperId} className="text-xs text-ink-sub">
            {helper}
          </p>
        )
      )}
    </div>
  );
}
