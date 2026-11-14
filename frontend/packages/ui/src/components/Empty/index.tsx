/**
 * Empty:空状态(规范 §5.1 / §6.5.3 三态)。
 * 标题说清这里本来放什么,说明说清为什么是空的,有下一步时放按钮;
 * 分「整页」(page:整页只有这一块)与「卡内」(section:卡片或列表区里,默认)两档尺寸。
 */
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface EmptyProps {
  /** 场景图标(Lucide) */
  icon?: LucideIcon;
  /** 空态标题:说明「这里本来放什么」 */
  title: string;
  /** 辅助说明:为什么是空的 / 可以做什么 */
  description?: string;
  /** 行动按钮插槽(由调用方放入 Button 等) */
  action?: ReactNode;
  /** 尺寸:page 整页空状态(上下留白更大)/ section 卡片或列表区内(默认) */
  size?: "page" | "section";
  className?: string;
}

/** Empty 居中排列图标、标题、说明与行动按钮;role=status 让读屏在结果变空时播报。 */
export function Empty({ icon, title, description, action, size = "section", className }: EmptyProps) {
  return (
    <div
      role="status"
      className={cn(
        "flex flex-col items-center justify-center px-6 text-center",
        size === "page" ? "py-16" : "py-10",
        className,
      )}
    >
      {icon && (
        <div className="mb-4 flex size-12 items-center justify-center rounded-full bg-surface-hover text-ink-sub">
          <Icon icon={icon} size="lg" />
        </div>
      )}
      <div className="text-md font-semibold text-ink">{title}</div>
      {description && <p className="mt-1.5 max-w-md text-sm text-ink-sub">{description}</p>}
      {action !== undefined && action !== null && <div className="mt-5">{action}</div>}
    </div>
  );
}
