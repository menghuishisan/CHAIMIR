/**
 * Empty:空状态引导(§5.1)。
 * 图标 + 标题 + 说明 + 行动按钮插槽,居中排版;
 * 用于列表/查询无数据时给出下一步指引,而不是留白。
 */
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface EmptyProps {
  /** 场景图标(Lucide) */
  icon?: LucideIcon;
  /** 空态标题:说明「这里是什么」 */
  title: string;
  /** 辅助说明:为什么是空的 / 可以做什么 */
  description?: string;
  /** 行动按钮插槽(由调用方放入 Button 等) */
  action?: ReactNode;
  className?: string;
}

export function Empty({ icon, title, description, action, className }: EmptyProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center px-6 py-12 text-center",
        className,
      )}
    >
      {icon && (
        <div className="mb-4 flex size-12 items-center justify-center rounded-lg bg-surface-sunken text-ink-sub">
          <Icon icon={icon} size="lg" />
        </div>
      )}
      <div className="text-md font-medium text-ink">{title}</div>
      {description && <p className="mt-1 max-w-sm text-sm text-ink-sub">{description}</p>}
      {action !== undefined && action !== null && <div className="mt-4">{action}</div>}
    </div>
  );
}
