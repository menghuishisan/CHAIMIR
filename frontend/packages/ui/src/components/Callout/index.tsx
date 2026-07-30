/**
 * Callout:段落级提示块(§5.1)。
 * 左侧 3px 语义色边 + 语义浅底 + 固定图标(色与图标双通道,色非唯一信息载体)。
 * 用于页面/表单内的就地说明与警示;danger 用独立冷红(朱砂不用于错误)。
 */
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { CircleAlert, CircleCheck, Info, TriangleAlert } from "lucide-react";
import { cva } from "class-variance-authority";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export type CalloutTone = "info" | "warning" | "danger" | "success";

const calloutVariants = cva("flex gap-3 rounded-md border-l-3 p-4", {
  variants: {
    tone: {
      info: "border-l-info bg-info-bg",
      warning: "border-l-warning bg-warning-bg",
      danger: "border-l-danger bg-danger-bg",
      success: "border-l-success bg-success-bg",
    },
  },
});

/** 每个语义色对应固定图标与图标色,不可由调用方替换,保证全站一致 */
const TONE_ICON: Record<CalloutTone, { icon: LucideIcon; className: string }> = {
  info: { icon: Info, className: "text-info" },
  warning: { icon: TriangleAlert, className: "text-warning" },
  danger: { icon: CircleAlert, className: "text-danger" },
  success: { icon: CircleCheck, className: "text-success" },
};

export interface CalloutProps {
  /** 语义色族 */
  tone: CalloutTone;
  /** 可选标题;正文放 children */
  title?: ReactNode;
  children: ReactNode;
  className?: string;
}

export function Callout({ tone, title, children, className }: CalloutProps) {
  const { icon, className: iconClassName } = TONE_ICON[tone];
  return (
    <div className={cn(calloutVariants({ tone }), className)}>
      <Icon icon={icon} size="md" className={cn("mt-0.5", iconClassName)} />
      <div className="min-w-0 text-sm">
        {title !== undefined && title !== null && (
          <div className="mb-1 font-medium text-ink">{title}</div>
        )}
        <div className="text-ink-sub">{children}</div>
      </div>
    </div>
  );
}
