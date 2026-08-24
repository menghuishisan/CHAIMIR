/**
 * Callout:段落级提示块(§5.1)。
 * 左侧 3px 语义色边 + 语义浅底 + 固定图标(色与图标双通道,色非唯一信息载体)。
 * 用于页面/表单内的就地说明与警示;danger 用独立冷红(朱砂不用于错误)。
 *
 * **反馈类 tone 是 live region。** §6.7 C 指定 Callout(冷红)承担「表单/动作提交失败」的就近内联展示,
 * 而动作结果必须被读屏播报 —— 否则盲用户点了提交、错误只在视觉上出现,他什么都听不到。
 * 故 danger 取 `role="alert"`(assertive:阻断当前动作,要立刻打断),
 * warning/success 取 `role="status"`(polite:等读屏念完当前内容再补报);
 * info 是**说明类**语气,常静态排在版面里,不做 live region,否则每次进页面都会被念一遍。
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

/** 反馈类 tone 的读屏角色;info 为说明类,不做 live region(见文件头) */
const TONE_ROLE: Record<CalloutTone, "alert" | "status" | undefined> = {
  info: undefined,
  warning: "status",
  danger: "alert",
  success: "status",
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
    <div role={TONE_ROLE[tone]} className={cn(calloutVariants({ tone }), className)}>
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
