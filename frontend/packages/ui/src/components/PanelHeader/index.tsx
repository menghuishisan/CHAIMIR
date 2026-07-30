/**
 * PanelHeader:面板标题层级(§5.1)。
 * 统一「eyebrow 小引题 / 标题 / 描述 / meta 辅助信息 / actions 操作区」的面板头结构;
 * 窄屏时操作区自动换行,不挤压标题。
 */
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface PanelHeaderProps {
  /** 小引题(Data 层等宽小字),标示所属域/分组 */
  eyebrow?: string;
  title: string;
  description?: string;
  icon?: LucideIcon;
  /** 辅助信息插槽(记录数/更新时间等),渲染在描述之后 */
  meta?: ReactNode;
  /** 操作区插槽(按钮/筛选等),窄屏自动换行到下一行 */
  actions?: ReactNode;
  className?: string;
}

export function PanelHeader({ eyebrow, title, description, icon, meta, actions, className }: PanelHeaderProps) {
  return (
    // flex-wrap:容器过窄时 actions 整体换行,标题区优先保持完整
    <div className={cn("flex flex-wrap items-start justify-between gap-x-4 gap-y-3", className)}>
      <div className="min-w-0">
        {eyebrow && (
          <p className="font-mono text-xs uppercase tracking-widest text-ink-sub">{eyebrow}</p>
        )}
        <div className={cn("flex items-center gap-2", eyebrow && "mt-1")}>
          {icon && <Icon icon={icon} size="lg" className="text-ink-sub" />}
          <h2 className="text-lg font-semibold text-ink">{title}</h2>
        </div>
        {description && <p className="mt-1 text-sm text-ink-sub">{description}</p>}
        {meta && <div className="mt-1 text-sm text-ink-sub">{meta}</div>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}
