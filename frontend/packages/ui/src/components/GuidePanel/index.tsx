/**
 * GuidePanel:引导页骨架(规范 §6.5.3 第 ⑧ 族「引导」)。
 *
 * 有些页面本身没有主体数据,它的全部职责是告诉用户「这件事在别处办」——
 * 平台端监控面板(真实面板由运维部署在外部系统)、教师端某些跳转入口。
 * 这类页面此前按资源列表族排出骨架,再拿空态块和 info Callout 把版面填满,
 * 结果是整页没有一句有效信息却占了一屏(§6.5.0 通则 3 禁止这种填充)。
 *
 * 骨架就三段:这里能做什么、**为什么不在这里做**、以及明确的出口。
 * 出口必须是按钮组而不是正文里的链接 —— 页面的唯一目的就是让人离开去办事,
 * 把动作藏在句子里等于把主操作降级成注脚。
 */
import type { LucideIcon } from "lucide-react";
import { useId, type ReactNode } from "react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface GuidePanelProps {
  /** 页面图标(Lucide);这一族的图标承载「这是什么事」,所以保留 */
  icon?: LucideIcon;
  /** 一句话说明这里能做什么(用户向) */
  title: string;
  /**
   * 为什么不在这里做。这一段是本族的核心 ——
   * 少了它,页面就退化成一个没解释的死胡同,用户会反复回来确认自己是不是操作错了。
   */
  reason: ReactNode;
  /** 出口按钮组:传 Button;至少一个 */
  actions: ReactNode;
  /** 补充说明(可选),如权限前提、需要单独登录等 */
  hint?: ReactNode;
  className?: string;
}

export function GuidePanel({ icon, title, reason, actions, hint, className }: GuidePanelProps) {
  // 区域名取自标题:无名 section 不会作为地标暴露给读屏,跳转时找不到这块
  const titleId = useId();
  return (
    // 抬起片(§6.5.1 第 1 级)。不铺满纸高、不加装饰块:内容短就短,纸的下边缘在视口外(§1.2)
    <section
      aria-labelledby={titleId}
      className={cn(
        "flex min-w-0 max-w-2xl flex-col items-start gap-4 rounded-lg bg-surface p-6 shadow-xs",
        className,
      )}
    >
      {icon && (
        <span className="inline-flex rounded-lg bg-primary-soft p-2.5 text-primary">
          <Icon icon={icon} size="lg" />
        </span>
      )}
      <div className="min-w-0">
        <h2 id={titleId} className="text-lg font-semibold text-ink">
          {title}
        </h2>
        <p className="mt-1.5 text-sm text-ink-sub">{reason}</p>
      </div>
      <div className="flex flex-wrap items-center gap-2">{actions}</div>
      {hint && <p className="text-xs text-ink-sub">{hint}</p>}
    </section>
  );
}
